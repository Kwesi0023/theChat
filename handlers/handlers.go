package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
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
	Name      string `json:"name"`
	CreatorID string `json:"creator_id"` // String ID of the room creator
}

// WebSocketUpgrader is configured to allow local testing
var wsUpgrader = websocket.Upgrader{
	/*
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
	*/
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

	if strings.TrimSpace(req.CreatorID) == "" {
		http.Error(w, "creator_id is required", http.StatusBadRequest)
		return
	}

	// Convert string creator_id to uint
	creatorID, err := strconv.ParseUint(req.CreatorID, 10, 32)
	if err != nil {
		http.Error(w, "creator_id must be a valid number", http.StatusBadRequest)
		return
	}

	if creatorID == 0 {
		http.Error(w, "creator_id must be greater than 0", http.StatusBadRequest)
		return
	}

	// Generate room ID
	roomID := generateRoomID()

	room := &models.Room{
		ID:        roomID,
		Name:      req.Name,
		CreatorID: uint(creatorID),
		Status:    "active", // default status
		Type:      "public", // default type
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

// UpdateRoomStatusRequest represents the request body for updating room status
type UpdateRoomStatusRequest struct {
	Status  string `json:"status"`   // 'active', 'archived', 'hidden'
	UserID  string `json:"user_id"`  // String ID of user making the request
	IsAdmin bool   `json:"is_admin"` // Whether user has admin privileges
}

// UpdateRoomStatus handles PATCH /api/rooms/{id}/status - only creator or admin can change status
func UpdateRoomStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID := vars["id"]

	var req UpdateRoomStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Convert string user_id to uint
	if strings.TrimSpace(req.UserID) == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseUint(req.UserID, 10, 32)
	if err != nil {
		http.Error(w, "user_id must be a valid number", http.StatusBadRequest)
		return
	}

	// Fetch the room to check creator
	room, err := database.GetRoom(roomID)
	if err != nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	// Check if user is the creator or an admin
	if room.CreatorID != uint(userID) && !req.IsAdmin {
		http.Error(w, "Unauthorized: only the room creator or admin can change room status", http.StatusForbidden)
		return
	}

	// Validate new status
	if req.Status != "active" && req.Status != "archived" && req.Status != "hidden" {
		http.Error(w, "Invalid status. Must be 'active', 'archived', or 'hidden'", http.StatusBadRequest)
		return
	}

	// Update room status in database
	if err := database.UpdateRoomStatus(roomID, req.Status); err != nil {
		log.Printf("Failed to update room status: %v", err)
		http.Error(w, "Failed to update room status", http.StatusInternalServerError)
		return
	}

	// Return updated room
	room.Status = req.Status
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(room)
}

// DeleteRoomRequest represents the request body for deleting a room
type DeleteRoomRequest struct {
	UserID  string `json:"user_id"`  // String ID of user making the request
	IsAdmin bool   `json:"is_admin"` // Whether user has admin privileges
}

// DeleteRoom handles DELETE /api/rooms/{id} - only creator or admin can delete
func DeleteRoom(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID := vars["id"]

	var req DeleteRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Convert string user_id to uint
	if strings.TrimSpace(req.UserID) == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseUint(req.UserID, 10, 32)
	if err != nil {
		http.Error(w, "user_id must be a valid number", http.StatusBadRequest)
		return
	}

	// Fetch the room to check creator
	room, err := database.GetRoom(roomID)
	if err != nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	// Check if user is the creator or an admin
	if room.CreatorID != uint(userID) && !req.IsAdmin {
		http.Error(w, "Unauthorized: only the room creator or admin can delete this room", http.StatusForbidden)
		return
	}

	// Delete room from database
	if err := database.DeleteRoom(roomID); err != nil {
		log.Printf("Failed to delete room: %v", err)
		http.Error(w, "Failed to delete room", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Room deleted successfully"})
}

// ServeWs handles the WebSocket upgrade and connection lifecycle
func ServeWs(w http.ResponseWriter, r *http.Request) {
	// Extract username and room_id from query parameters (validated by middleware)
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	roomID := strings.TrimSpace(r.URL.Query().Get("room_id"))
	roomName := strings.TrimSpace(r.URL.Query().Get("room_name")) // Optional, for logging

	log.Printf("WebSocket connection attempt: username=%s, room_id=%s, room_name=%s", username, roomID, roomName)

	// Check room status using raw SQL (before upgrade)
	status, err := database.GetRoomStatus(roomName)
	if err != nil {
		log.Printf("Room not found: %s", roomName)
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	// Handle room status logic
	if status == "hidden" {
		log.Printf("Attempted access to hidden room: %s", roomName)
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

	// Create user model with generated user ID

	user := &models.User{
		ID:       0, // Will be assigned from userID string if needed, otherwise use as reference
		Username: username,

		JoinedAt: time.Now(),
	}

	// The Hand-Off: Get or create room hub and create client
	roomHub := Hub.GetOrCreateRoomHub(roomName)
	client := ws.NewClient(conn, roomHub, user, status)

	// Fetch message history(descending order)
	messages, err := database.GetLastMessages(roomName, 50)
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
			RoomID:   roomName,
		}
		client.send <- historyMsg
		log.Printf("Sent %d message history items to user %s", len(messages), username)
	}

	// If room is archived, send read-only notification
	if status == "archived" {
		log.Printf("%s connected to archived room %s (read-only)", username, roomName)
		readOnlyMsg := models.WebSocketMessage{
			Type:    "system",
			Content: "This room is read-only. You can view messages but cannot send new ones.",
		}
		client.send <- readOnlyMsg
	}

	// Save join message silently (no broadcast)
	if err := database.SaveSilentJoinMessage(roomID, user.ID, username); err != nil {
		log.Printf("Failed to save silent join message: %v", err)
	}

	// Register client with the room hub, passing the room status
	roomHub.RegisterWithStatus(client, status)

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
