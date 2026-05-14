package websocket

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Kwesi0023/theChat/database"
	"github.com/Kwesi0023/theChat/models"
)

// Hub maintains active client connections per room and handles message broadcasting
type Hub struct {
	// Rooms maps room IDs to their respective room hubs
	rooms map[string]*RoomHub
	mu    sync.RWMutex
}

// RoomHub manages clients and users in a specific room
type RoomHub struct {
	roomID     string
	roomStatus string // 'active', 'archived', or 'hidden'
	clients    map[*Client]bool
	users      map[string]*models.User // username -> User
	broadcast  chan interface{}
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]*RoomHub),
	}

}

// GetOrCreateRoomHub returns an existing room hub or creates a new one
func (h *Hub) GetOrCreateRoomHub(roomID string) *RoomHub {
	h.mu.Lock()
	defer h.mu.Unlock()

	if hub, exists := h.rooms[roomID]; exists {
		return hub
	}

	roomHub := &RoomHub{
		roomID:     roomID,
		roomStatus: "active", // default status
		clients:    make(map[*Client]bool),
		users:      make(map[string]*models.User),
		broadcast:  make(chan interface{}, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}

	h.rooms[roomID] = roomHub
	go roomHub.run()

	log.Printf("Created new room hub for room: %v", roomID)
	return roomHub
}

// run manages client registrations, unregistrations, and message broadcasting for a room
func (rh *RoomHub) run() {
	for {
		select {
		case client := <-rh.register:
			rh.mu.Lock()
			rh.clients[client] = true
			rh.users[client.User.Username] = client.User
			rh.mu.Unlock()

			log.Printf("User %s joined room %s", client.User.Username, rh.roomID)

			// Broadcast user list to all clients
			rh.broadcastUserList()

		case client := <-rh.unregister:
			rh.mu.Lock()
			if _, ok := rh.clients[client]; ok {
				delete(rh.clients, client)
				delete(rh.users, client.User.Username)
				close(client.send)
				rh.mu.Unlock()

				log.Printf("User %s left room %s", client.User.Username, rh.roomID)

				// Save leave message to database with msg_type = 'leave' (raw SQL)
				leaveMsg := &models.Message{
					ID:        generateSystemMessageID(fmt.Sprintf("%d", rh.roomID)),
					RoomID:    rh.roomID,
					UserID:    client.User.ID,
					Username:  client.User.Username,
					Content:   "",
					MsgType:   "leave",
					Timestamp: time.Now(),
				}
				rh.saveMessageWithType(leaveMsg)

				// Broadcast leave notification to remaining clients
				rh.BroadcastLeaveNotification(client.User.Username)

				// Broadcast user list to remaining clients
				rh.broadcastUserList()
			} else {
				rh.mu.Unlock()
			}

		case message := <-rh.broadcast:
			rh.mu.RLock()
			for client := range rh.clients {
				select {
				case client.send <- message:
				default:
					// Client's send channel is full, skip
					log.Printf("Client send channel full for %s", client.User.Username)
				}
			}
			rh.mu.RUnlock()
		}
	}
}

// BroadcastMessage sends a message to all clients in the room
func (rh *RoomHub) BroadcastMessage(msg *models.Message) {
	wsMsg := models.WebSocketMessage{
		Type:      "message",
		Content:   msg.Content,
		Username:  msg.Username,
		UserID:    msg.UserID,
		RoomID:    msg.RoomID,
		Timestamp: msg.Timestamp,
	}
	rh.broadcast <- wsMsg
}

// BroadcastJoinNotification notifies clients that a user joined
func (rh *RoomHub) BroadcastJoinNotification(user *models.User) {
	wsMsg := models.WebSocketMessage{
		Type:     "join",
		Username: user.Username,
		UserID:   user.ID,
		RoomID:   rh.roomID,
	}
	rh.broadcast <- wsMsg
}

// BroadcastLeaveNotification notifies clients that a user left
func (rh *RoomHub) BroadcastLeaveNotification(username string) {
	wsMsg := models.WebSocketMessage{
		Type:     "leave",
		Username: username,
		RoomID:   rh.roomID,
	}
	rh.broadcast <- wsMsg
}

// broadcastUserList sends the current list of users in the room to all clients
func (rh *RoomHub) broadcastUserList() {
	rh.mu.RLock()
	users := make([]models.User, 0, len(rh.users))
	for _, user := range rh.users {
		users = append(users, *user)
	}
	rh.mu.RUnlock()

	wsMsg := models.WebSocketMessage{
		Type:   "user_list",
		Users:  users,
		RoomID: rh.roomID,
	}
	rh.broadcast <- wsMsg
}

// registerWithStatus sets the room status and then registers the client
func (rh *RoomHub) registerWithStatus(client *Client, status string) {
	rh.mu.Lock()
	rh.roomStatus = status
	rh.mu.Unlock()

	// Send to register channel
	rh.register <- client
}

// GetUsers returns a copy of the current users in the room
func (rh *RoomHub) GetUsers() []*models.User {
	rh.mu.RLock()
	defer rh.mu.RUnlock()

	users := make([]*models.User, 0, len(rh.users))
	for _, user := range rh.users {
		users = append(users, user)
	}
	return users
}

// GetClientCount returns the number of connected clients in the room
func (rh *RoomHub) GetClientCount() int {
	rh.mu.RLock()
	defer rh.mu.RUnlock()
	return len(rh.clients)
}

// saveMessageWithType saves a message with msg_type to database (raw SQL helper)
func (rh *RoomHub) saveMessageWithType(msg *models.Message) error {
	return database.SaveMessageWithType(msg)
}

// BroadcastReaction broadcasts a reaction to all clients in the room
func (rh *RoomHub) BroadcastReaction(reaction *models.Reaction) {
	wsMsg := models.WebSocketMessage{
		Type:      "reaction",
		MessageID: reaction.MessageID,
		Emoji:     reaction.Emoji,
		Username:  reaction.Username,
		UserID:    reaction.UserID,
		Timestamp: reaction.CreatedAt,
	}
	rh.broadcast <- wsMsg
}

// Helper function to generate system message IDs
func generateSystemMessageID(roomID string) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return time.Now().Format("20060102150405") + "-sys-" + string(b[:8])
}
