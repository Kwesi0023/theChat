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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
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
	CreatorID string `json:"creator_id"`
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
	creatorIDStr := strings.TrimSpace(req.CreatorID)
	if req.Name == "" || creatorIDStr == "" {
		http.Error(w, "Room name or creator_id cannot be empty", http.StatusBadRequest)
		return
	}

	// Convert creator string ID to a numeric uint for our models.Room struct
	cID, err := strconv.ParseUint(creatorIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid creator_id format", http.StatusBadRequest)
		return
	}

	exists, err := database.RoomNameExists(req.Name)
	if err != nil {
		http.Error(w, "Database verification failed", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "A room with that name already exists", http.StatusConflict)
		return
	}

	// Derive unique ID slug (e.g., "The General Lounge" -> "the-general-lounge")
	roomID := strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))

	room := &models.Room{
		ID:          roomID,
		Name:        req.Name,
		Description: "Welcome to the " + req.Name + " chat room",
		Status:      "active",
		Type:        "public",
		CreatorID:   uint(cID), // ✅ Linked: user_id equals creator_id
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

	messages, err := database.GetChatHistory(roomID, 50)
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
}

// handles PATCH /api/rooms/{id}/status - admin only
func UpdateRoomStatus(hub *ws.Hub, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID := vars["id"]

	var req UpdateRoomStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	//Admin-only gatekeeper
	if !req.IsAdmin {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized. Admin privileges required."})
		return
	}

	// Fetch the room to verify it exists
	room, err := database.GetRoomByID(roomID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Room not found"})
		return
	}

	// Validate new status
	if req.Status != "active" && req.Status != "archived" && req.Status != "hidden" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid status. Must be 'active', 'archived', or 'hidden'"})
		return
	}

	if err := database.UpdateRoomStatus(roomID, req.Status); err != nil {
		log.Printf("Failed to update room status: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update room status"})
		return
	}

	//Sync the in-memory RoomHub status (CRITICAL for client enforcement)
	if err := hub.UpdateRoomStatus(roomID, req.Status); err != nil {
		log.Printf("Warning: failed to update in-memory room status: %v", err)
		// Log warning but don't fail the request - database is already updated
	}

	roomHub := hub.GetOrCreateRoomHub(roomID)
	systemMsg := models.WebSocketMessage{
		Type:      "system",
		MsgType:   "status_change",
		Content:   fmt.Sprintf("Room status changed to: %s", req.Status),
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
	RoomID  string `json:"room_id"`  // Room ID to delete
	UserID  string `json:"user_id"`  // String ID of user making the request
	IsAdmin bool   `json:"is_admin"` // Whether user has admin privileges
}

func DeleteRoomHandler(hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only allow standard HTTP DELETE actions
		if r.Method != http.MethodDelete {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
			return
		}

		// Parse request body for roomID, UserID and admin status
		var req DeleteRoomRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		roomID := strings.TrimSpace(req.RoomID)
		if roomID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Missing room_id in request body"})
			return
		}

		// CRITICAL: Admin-only gatekeeper
		if !req.IsAdmin {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized. Admin privileges required."})
			return
		}

		//Delete from MySQL database
		if err := database.DeleteRoom(roomID); err != nil {
			if err.Error() == "room not found" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "There is not such said room in the Database"})
				return
			}
			log.Printf("Database deletion error for room %s: %v", roomID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server database error"})
			return
		}

		// disrupts the websocket connection and drops the live feed
		hub.CloseRoomHub(roomID)

		log.Printf(" The Room ``%s`` has been dropped successfully.", roomID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": fmt.Sprintf("Room %s and all its chat history were permanently deleted.", roomID),
		})
	}
} //'5228

// ServeWs handles the WebSocket upgrade and connection lifecycle
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

	log.Printf("Ws connection is up for user %s inside room %s", username, roomID)

	go client.WritePump()

	client.ReadPump()
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
		http.Error(w, "Username or password cannot be empty", http.StatusBadRequest)
		return
	}
	// Attempt to authenticate the user assuming they already exist
	user, err := database.AuthenticateUser(req.Username, req.Password)
	//0423
	if err != nil {
		// If user wasn't found, seamlessly register them right now!
		if err.Error() == "user not found" {
			time.Sleep(3 * time.Second)
			log.Printf("%s not found. registering new user...", req.Username)

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
			log.Printf("%s was not aunthenicated: %v", req.Username, err)
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
	log.Printf("%s logged in successfully", user.Username)
}

// Helper functions

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
