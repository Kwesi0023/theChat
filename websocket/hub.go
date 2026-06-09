package websocket

import (
	"log"
	"sync"

	"github.com/Kwesi0023/theChat/database"
	"github.com/Kwesi0023/theChat/models"
)

// Hub maintains active client connections per room and handles message broadcasting
type Hub struct {
	// Rooms maps room IDs to their respective room hubs
	rooms map[string]*RoomHub
	mu    sync.RWMutex
}

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

// JoinRoom sends a client connection into the unexported registration channel safely across packages
func (rh *RoomHub) JoinRoom(client *Client) {
	rh.register <- client
}

func (h *Hub) GetOrCreateRoomHub(roomID string) *RoomHub {
	h.mu.Lock()
	defer h.mu.Unlock()

	// If the room hub already exists, just return it cleanly
	if room, exists := h.rooms[roomID]; exists {
		return room
	}

	//RoomHub configuration instance
	roomHub := &RoomHub{
		roomID:     roomID,
		roomStatus: "active",
		clients:    make(map[*Client]bool),
		users:      make(map[string]*models.User),
		broadcast:  make(chan interface{}, 32),
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
	}

	h.rooms[roomID] = roomHub

	//Spin up the room's background event channel thread!
	go roomHub.Run()

	log.Printf("A new live background channel for room: %s", roomID)
	return roomHub
}

func (h *Hub) CloseRoomHub(roomID string) {
	h.mu.Lock()
	roomHub, exists := h.rooms[roomID]
	if !exists {
		h.mu.Unlock()
		return
	}
	// 1. Wipe the room instance completely from the central application memory map
	delete(h.rooms, roomID)
	h.mu.Unlock()

	// 2. Safely lock down this specific room's client map
	roomHub.mu.Lock()
	defer roomHub.mu.Unlock()

	log.Printf("Shutting down room hub %s. Evicting %d live clients...", roomID, len(roomHub.clients))

	// 3. Loop through every active connection, notify them, and close their egress mailboxes
	for client := range roomHub.clients {
		// Send a final system alert warning to the client browser console before disconnect
		systemNotice := models.WebSocketMessage{
			Type:    "system",
			Content: "This chat room has been deleted by the admin.",
		}

		select {
		case client.Send <- systemNotice:
		default:
			// If channel is blocked, ignore and continue to teardown
		}

		close(client.Send)

		client.conn.Close()

		// Remove them from maps
		delete(roomHub.clients, client)
		delete(roomHub.users, client.User.Username)
	}
}

// run manages client registrations, unregistrations, and message broadcasting for a room
func (rh *RoomHub) Run() {
	for {
		select {
		case client := <-rh.register:
			rh.mu.Lock()
			rh.clients[client] = true
			rh.users[client.User.Username] = client.User
			rh.mu.Unlock()
			log.Printf("%s joined room: %s", client.User.Username, rh.roomID)
			rh.broadcastUserList()

		case client := <-rh.unregister:
			rh.mu.Lock()
			if _, ok := rh.clients[client]; ok {
				delete(rh.clients, client)
				delete(rh.users, client.User.Username)
				close(client.Send)
			}
			rh.mu.Unlock()
			log.Printf("%s left room: %s", client.User.Username, rh.roomID)
			rh.broadcastUserList()

		case message := <-rh.broadcast:
			rh.mu.RLock()
			// This loop clones and delivers the message to EVERY active browser connected!
			for client := range rh.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(rh.clients, client)
					delete(rh.users, client.User.Username)
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
		UserID:    msg.SenderID,
		RoomID:    string(msg.RoomID),
		MessageID: msg.ID,
		Timestamp: msg.Timestamp,
	}
	rh.broadcast <- wsMsg
}

// BroadcastSystemMessage sends a system notification to all clients in the room
func (rh *RoomHub) BroadcastSystemMessage(wsMsg models.WebSocketMessage) {
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

// RegisterWithStatus registers a client with the room hub and sets the room status
func (rh *RoomHub) RegisterWithStatus(client *Client, status string) {
	rh.mu.Lock()
	rh.roomStatus = status
	rh.mu.Unlock()

	// Send client to register channel
	rh.register <- client
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

func (rh *RoomHub) SaveMessage(msg *models.Message) error {
	return database.SaveMessage(msg)
}

// BroadcastReaction broadcasts a reaction to all clients in the room
func (rh *RoomHub) BroadcastReaction(reaction *models.Reaction) {
	wsMsg := models.WebSocketMessage{
		Type:      "reaction",
		MessageID: reaction.MessageID,
		Emoji:     reaction.Emoji,
		Username:  reaction.Username,
		Timestamp: reaction.CreatedAt,
	}
	rh.broadcast <- wsMsg
}
