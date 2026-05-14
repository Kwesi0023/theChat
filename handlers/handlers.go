package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Kwesi0023/theChat/database"
	"github.com/Kwesi0023/theChat/models"
	ws "github.com/Kwesi0023/theChat/websocket"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// Global WebSocket hub
var Hub *ws.Hub

// Initialize sets up the handlers hub
func Initialize() {
	Hub = ws.NewHub()
}

// CreateRoomRequest represents the request body for creating a room
type CreateRoomRequest struct {
	Name string `json:"name"`
}

// WebSocketUpgrader is configured to allow local testing
var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from localhost for testing
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		// In production, validate the origin against your allowed hosts
		// we might just use one of this.. usually the localhost
		return strings.HasPrefix(origin, "http://localhost") ||
			strings.HasPrefix(origin, "http://127.0.0.1") ||
			strings.HasPrefix(origin, "ws://localhost") ||
			strings.HasPrefix(origin, "ws://127.0.0.1")
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// CreateRoom handles POST /api/rooms
func CreateRoom(w http.ResponseWriter, r *http.Request) {
	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "Room name cannot be empty", http.StatusBadRequest)
		return
	}

	// Generate room ID
	roomID := generateRoomID()

	room := &models.Room{
		ID:        roomID,
		Name:      req.Name,
		CreatedAt: time.Now(),
	}

	if err := database.CreateRoom(room); err != nil {
		log.Printf("Failed to create room: %v", err)
		http.Error(w, "Failed to create room", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(room)
}

// GetRoomMessages handles GET /api/rooms/{id}/messages
func GetRoomMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID := vars["id"]

	// Verify room exists
	room, err := database.GetRoom(roomID)
	if err != nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	// Fetch message history (last 100 messages)
	messages, err := database.GetMessagesByRoom(roomID, 100)
	if err != nil {
		log.Printf("Failed to fetch messages: %v", err)
		http.Error(w, "Failed to fetch messages", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"room":     room,
		"messages": messages,
	}
	json.NewEncoder(w).Encode(response)
}

// GetAllRooms handles GET /api/rooms
func GetAllRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := database.GetAllRooms()
	if err != nil {
		log.Printf("Failed to fetch rooms: %v", err)
		http.Error(w, "Failed to fetch rooms", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"rooms": rooms,
	}
	json.NewEncoder(w).Encode(response)
}

// ServeWs handles the WebSocket upgrade and connection lifecycle
func ServeWs(w http.ResponseWriter, r *http.Request) {
	// Extract username and room_id from query parameters (validated by middleware)
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	roomID := strings.TrimSpace(r.URL.Query().Get("room_id"))

	// Check room status using raw SQL (before upgrade)
	status, err := database.GetRoomStatus(roomID)
	if err != nil {
		log.Printf("Room not found: %s", roomID)
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	// Handle room status logic
	if status == "hidden" {
		log.Printf("Attempted access to hidden room: %s", roomID)
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	// The Handshake: Upgrade HTTP connection to WebSocket
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		http.Error(w, "Failed to upgrade connection", http.StatusBadRequest)
		return
	}

	log.Printf("WebSocket connection established for user %s in room %s (status: %s)", username, roomID, status)

	// Create user model
	user := &models.User{
		ID:       generateUserID(),
		Username: username,
		RoomID:   roomID,
		JoinedAt: time.Now(),
	}

	// The Hand-Off: Get or create room hub and create client
	roomHub := Hub.GetOrCreateRoomHub(roomID)
	client := ws.NewClient(conn, roomHub, user, status)

	// Fetch message history(descending order)
	messages, err := database.GetLastMessages(roomID, 50)
	if err != nil {
		log.Printf("Failed to fetch message history: %v", err)
	} else if messages != nil && len(messages) > 0 {
		// Fetch reactions for each message and attach them
		for i := len(messages) - 1; i >= 0; i-- {
			reactions, _ := database.GetReactionsByMessageID(messages[i].ID)
			messages[i].Reactions = reactions
		}
		// Reverse the slice to chronological order (oldest first)
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}

		// Send history to client
		historyMsg := models.WebSocketMessage{
			Type:     "history",
			Messages: messages,
			RoomID:   roomID,
		}
		client.send <- historyMsg
		log.Printf("Sent %d message history items to user %s", len(messages), username)
	}

	// If room is archived, send read-only notification
	if status == "archived" {
		log.Printf("User %s connected to archived room %s (read-only)", username, roomID)
		readOnlyMsg := models.WebSocketMessage{
			Type:    "system",
			Content: "This room is read-only. You can view messages but cannot send new ones.",
		}
		client.send <- readOnlyMsg
	}

	// Save join message to database with msg_type = 'join'
	joinMsg := &models.Message{
		ID:        generateMessageID(),
		RoomID:    roomID,
		UserID:    user.ID,
		Username:  username,
		Content:   "",
		MsgType:   "join",
		Timestamp: time.Now(),
	}
	if err := database.SaveMessageWithType(joinMsg); err != nil {
		log.Printf("Failed to save join message: %v", err)
	}

	// Register client with the room hub, passing the room status
	roomHub.registerWithStatus(client, status)

	// Concurrency: Start two separate goroutines
	client.Start()

	log.Printf("Client goroutines started for user %s", username)
}

// HealthCheck handles GET /health
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// Helper functions

func generateRoomID() string {
	return time.Now().Format("20060102150405") + "-room-" + randomString(8)
}

func generateUserID() string {
	return time.Now().Format("20060102150405") + "-user-" + randomString(8)
}

func generateMessageID() string {
	return time.Now().Format("20060102150405") + "-msg-" + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
