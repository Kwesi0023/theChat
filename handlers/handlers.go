package handlers

import (
	"encoding/json"
	"fmt"
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

// Setup the Gorilla WebSocket Upgrader configuration for this file
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allows testing across environments locally
		return true
	},
}

// LoginRequest represents the request body for login
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

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

// The frontend CANNOT pass or override a creator_id in the request body.
// This ensures room ownership is tied to the verified database user_id from the JWT claims.
func CreateRoom(w http.ResponseWriter, r *http.Request) {
	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		http.Error(w, "Room name cannot be empty", http.StatusBadRequest)
		return
	}

	// 1. Check database for unique room name constraint
	exists, err := database.RoomNameExists(req.Name)
	if err != nil {
		http.Error(w, "Database verification failed", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "A room with that name already exists", http.StatusConflict)
		return
	}

	// 2. Derive unique ID slug (e.g., "The General Lounge" -> "the-general-lounge")
	roomID := strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))

	room := &models.Room{
		ID:          roomID,
		Name:        req.Name,
		Description: "Welcome to the " + req.Name + " chat room",
		CreatorID:   0 + 1,
		Status:      "active",
		Type:        "public",
		CreatedAt:   time.Now(),
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
	room, err := database.GetRoomByID(roomID)
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

func JoinRoom(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RoomID   string `json:"room_id"`
		UserID   string `json:"user_id"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.RoomID) == "" {
		http.Error(w, "room_id is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.RoomID) == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}
}

// handles GET /api/rooms
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

// handles PATCH /api/rooms/{id}/status - only creator or admin can change status
func UpdateRoomStatus(hub *ws.Hub, w http.ResponseWriter, r *http.Request) {
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
	room, err := database.GetRoomByID(roomID)
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

	// Broadcast status change to the room
	roomHub := hub.GetOrCreateRoomHub(roomID)
	systemMsg := models.WebSocketMessage{
		Type:      "system",
		MsgType:   "status_change",
		Content:   fmt.Sprintf("The room is now %s", req.Status),
		RoomID:    roomID,
		Timestamp: time.Now(),
	}
	roomHub.BroadcastSystemMessage(systemMsg)

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
func DeleteRoom(hub *ws.Hub, w http.ResponseWriter, r *http.Request) {
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

	// Fetch the room to check creator --GetRoomByID would be equal to GetRooms
	room, err := database.GetRoomByID(roomID)
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
	//'5228
}

// ServeWs handles the WebSocket upgrade and connection lifecycle with JWT token validation
func ServeWs(hub *ws.Hub, w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimSpace(r.URL.Query().Get("roomID"))
	userIDStr := strings.TrimSpace(r.URL.Query().Get("userID"))
	username := strings.TrimSpace(r.URL.Query().Get("username"))

	if roomID == "" || userIDStr == "" || username == "" {
		log.Printf("WebSocket connection attempt blocked: missing query parameters")
		http.Error(w, "Missing roomID, userID, or username", http.StatusBadRequest)
		return
	}

	userID64, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid userID format", http.StatusBadRequest)
		return
	}

	// Fetch room to confirm visibility status
	room, err := database.GetRoomByID(roomID)
	if err != nil {
		log.Printf("Room validation failed for room %s: %v", roomID, err)
		http.Error(w, "Room not found", http.StatusBadRequest)
		return
	}

	if room.Status == "hidden" || room.Status == "archived" {
		log.Printf("Access denied: room %s is %s", roomID, room.Status)
		http.Error(w, "Room is not accessible", http.StatusForbidden)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	userModel := &models.User{
		ID:       uint(userID64),
		Username: username,
		RoomID:   roomID,
	}

	roomHub := hub.GetOrCreateRoomHub(roomID)
	client := ws.NewClient(conn, roomHub, userModel, room.Status)

	roomHub.JoinRoom(client)

	log.Printf("WebSocket connection established for user %s (ID: %d) in room %s", username, userModel.ID, roomID)
}

// HealthCheck handles GET /health
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "oooohhhhhhh yyyhhhhhh",
	})
}

// Login handles POST /api/auth/login
func Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	// 2. Validate that the fields are not empty or full of white spaces
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password cannot be empty", http.StatusBadRequest)
		return
	}
	// Step 1: Attempt to authenticate the user assuming they already exist
	user, err := database.AuthenticateUser(req.Username, req.Password)
	//0423
	if err != nil {
		// Step 2: If user wasn't found, seamlessly register them right now!
		if err.Error() == "user not found" {
			time.Sleep(3 * time.Second)
			log.Printf("User %s not found. Attempting automatic registration...", req.Username)

			err = database.RegisterUser(req.Username, req.Password)
			if err != nil {
				log.Printf("Automatic registration failed for %s: %v", req.Username, err)
				http.Error(w, "Failed to create user profile", http.StatusInternalServerError)
				return
			}

			// Try authenticating one more time now that they are registered successfully
			user, err = database.AuthenticateUser(req.Username, req.Password)
			if err != nil {
				http.Error(w, "Authentication failed after registration", http.StatusInternalServerError)
				return
			}
		} else {
			// If user was found but password didn't match
			log.Printf("User %s was not aunthenicated: %v", req.Username, err)
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Congrats, Login successful",
		"user_id":  user.ID,
		"username": user.Username,
	})
	log.Printf("User %s logged in successfully", user.Username)
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
