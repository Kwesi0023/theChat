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
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// JWT Configuration
// JWTSecret is used for signing and validating JWT tokens.
// WARNING: In production, load this from environment variables.
const JWTSecret = "your-jwt-secret" // TODO: Use environment variable in production

// Claims represents the JWT claims structure with verified user identity.
// The ID field is the auto-incremented primary key from the users database table,
// ensuring the authenticated user's true database identity is embedded in every token.
type Claims struct {
	ID       uint   `json:"id"` // Verified database user_id (primary key from users table)
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// LoginRequest represents the request body for login
type LoginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents the response body for successful login
type LoginResponse struct {
	Token   string `json:"token"`
	Message string `json:"message"`
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

// CreateRoom handles POST /api/rooms - requires JWT authentication
// SECURITY: creator_id is automatically extracted from the authenticated JWT token.
// The frontend CANNOT pass or override a creator_id in the request body.
// This ensures room ownership is tied to the verified database user_id from the JWT claims.
func CreateRoom(w http.ResponseWriter, r *http.Request) {
	// Extract JWT token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}

	// Extract token from "Bearer <token>" format
	var tokenStr string
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenStr = authHeader[7:]
	} else {
		http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
		return
	}

	// Parse and validate JWT using the secret key
	// This ensures only tokens signed with our secret can be accepted
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(JWTSecret), nil
	})

	if err != nil || !token.Valid {
		log.Printf("CreateRoom - Invalid JWT token - Error: %v | Token Valid: %v | Claims ID: %v", err, token.Valid, claims.ID)
		http.Error(w, fmt.Sprintf("Unauthorized: %v", err), http.StatusUnauthorized)
		return
	}

	// Extract room name from request
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

	// SECURITY: Use verified user_id from JWT claims as room creator
	// claims.ID is the auto-incremented user PK from the database, not user-supplied
	// This guarantees the room creator is the authenticated user - no spoofing possible
	room := &models.Room{
		ID:        roomID,
		Name:      req.Name,
		CreatorID: claims.ID, // Automatically set from verified JWT claims
		Status:    "active",  // default status
		Type:      "public",  // default type
		CreatedAt: time.Now(),
	}

	if err := database.CreateRoom(room); err != nil {
		log.Printf("CreateRoom - Database error: %v | Room ID: %s | Creator ID: %d | Room Name: %s", err, room.ID, room.CreatorID, room.Name)
		http.Error(w, fmt.Sprintf("Failed to create room: %v", err), http.StatusInternalServerError)
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

// ServeWs handles the WebSocket upgrade and connection lifecycle with JWT token validation
func ServeWs(w http.ResponseWriter, r *http.Request) {
	// Extract JWT token from query parameter
	tokenStr := strings.TrimSpace(r.URL.Query().Get("token"))
	if tokenStr == "" {
		log.Println("WebSocket connection attempt: missing token")
		http.Error(w, "Unauthorized: token is required", http.StatusUnauthorized)
		return
	}

	// Parse and validate the JWT token
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(JWTSecret), nil
	})

	if err != nil || !token.Valid {
		log.Printf("Invalid JWT token: %v", err)
		http.Error(w, "Unauthorized: invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Extract room_name from query parameter
	roomName := strings.TrimSpace(r.URL.Query().Get("room_name"))
	if roomName == "" {
		log.Println("WebSocket connection attempt: missing room_name")
		http.Error(w, "Bad request: room_name is required", http.StatusBadRequest)
		return
	}

	log.Printf("WebSocket connection attempt: user_id=%d, username=%s, room_name=%s", claims.ID, claims.Username, roomName)

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

	log.Printf("WebSocket connection established for user %s (ID:%d) in room %s (status: %s)", claims.Username, claims.ID, roomName, status)

	// Create user model from verified JWT claims
	user := &models.User{
		ID:       claims.ID,
		Username: claims.Username,
		RoomID:   roomName,
		JoinedAt: time.Now(),
	}

	// The Hand-Off: Get or create room hub and create client
	roomHub := Hub.GetOrCreateRoomHub(roomName)
	client := ws.NewClient(conn, roomHub, user, status)

	// Fetch message history(descending order)
	messages, err := database.GetLastMessages(roomName, 50)
	if err != nil {

		log.Printf("Failed to fetch message history: %v", err)
	} else if len(messages) > 0 {
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
		client.Send <- historyMsg
		log.Printf("Sent %d message history items to user %s", len(messages), user.Username)
	}

	// If room is archived, send read-only notification
	if status == "archived" {
		log.Printf("%s connected to archived room %s (read-only)", user.Username, roomName)
		readOnlyMsg := models.WebSocketMessage{
			Type:    "system",
			Content: "This room is read-only. You can view messages but cannot send new ones.",
		}
		client.Send <- readOnlyMsg
	}

	// Save join message silently (no broadcast)
	if err := database.SaveSilentJoinMessage(roomName, user.ID, user.Username); err != nil {
		log.Printf("Failed to save silent join message: %v", err)
	}

	// Register client with the room hub, passing the room status
	roomHub.RegisterWithStatus(client, status)

	// Concurrency: Start two separate goroutines
	client.Start()

	log.Printf("Client goroutines started for user %s", user.Username)
}

// HealthCheck handles GET /health
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// Login handles POST /api/auth/login
func Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.Username) == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Password) == "" {
		http.Error(w, "password is required", http.StatusBadRequest)
		return
	}

	// Authenticate user
	user, err := database.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		log.Printf("Authentication failed for user %s: %v", req.Username, err)
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// Create JWT claims
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		ID:       user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// Create and sign the JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(JWTSecret))
	if err != nil {
		log.Printf("Failed to sign token: %v", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Return token in response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(LoginResponse{
		Token:   tokenString,
		Message: "Login successful",
	})
	log.Printf("User %s logged in successfully", user.Username)
}

// Register handles POST /api/auth/register
func Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.Username) == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Email) == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Password) == "" {
		http.Error(w, "password is required", http.StatusBadRequest)
		return
	}

	// Register the user
	if err := database.RegisterUser(req.Username, req.Email, req.Password); err != nil {
		log.Printf("Registration failed for user %s: %v", req.Username, err)
		http.Error(w, "Registration failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "User registered successfully",
		"username": req.Username,
	})
	log.Printf("User %s registered successfully", req.Username)
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
