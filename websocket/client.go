package websocket

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Kwesi0023/theChat/database"
	"github.com/Kwesi0023/theChat/models"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period
	pingInterval = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

// Client represents a connected WebSocket client
type Client struct {
	conn       *websocket.Conn
	roomHub    *RoomHub
	User       *models.User
	roomStatus string // 'active', 'archived', or 'hidden'
	send       chan interface{}
}

// NewClient creates a new client instance with room status
func NewClient(conn *websocket.Conn, roomHub *RoomHub, user *models.User, roomStatus string) *Client {
	return &Client{
		conn:       conn,
		roomHub:    roomHub,
		User:       user,
		roomStatus: roomStatus,
		send:       make(chan interface{}, 256),
	}
}

// Start initiates the client's read and write goroutines
func (c *Client) Start() {
	go c.readPump()
	go c.writePump()
}

// readPump reads messages from the WebSocket connection and broadcasts them to the hub
func (c *Client) readPump() {
	defer func() {
		c.roomHub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	c.conn.SetReadLimit(maxMessageSize)

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var wsMsg models.WebSocketMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			continue
		}

		// Process different message types
		switch wsMsg.Type {
		case "message":
			c.handleMessage(wsMsg)
		case "reaction":
			c.handleReaction(wsMsg)
		default:
			log.Printf("Unknown message type: %s", wsMsg.Type)
		}
	}
}

// writePump writes messages from the hub's broadcast channel to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))

			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			data, err := json.Marshal(message)
			if err != nil {
				w.Close()
				return
			}

			w.Write(data)

			// Add queued messages to the current WebSocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				data, err := json.Marshal(<-c.send)
				if err != nil {
					w.Close()
					return
				}
				w.Write(data)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming chat messages
func (c *Client) handleMessage(wsMsg models.WebSocketMessage) {
	if wsMsg.Content == "" {
		return
	}

	// Check if room is archived - prevent saves and broadcasts
	if c.roomStatus == "archived" {
		log.Printf("%s attempted to send message in archived room %s", c.User.Username, c.roomHub.roomID)
		errorMsg := models.WebSocketMessage{
			Type:    "error",
			Content: "This room has been archived. You cannot send messages.",
		}
		c.send <- errorMsg
		return
	}

	msg := &models.Message{
		ID:        generateID(),
		RoomID:    c.roomHub.roomID,
		UserID:    c.User.ID,
		Username:  c.User.Username,
		Content:   wsMsg.Content,
		MsgType:   "message",
		Timestamp: time.Now(),
	}

	// Save to database with msg_type
	if err := database.SaveMessageWithType(msg); err != nil {
		log.Printf("Failed to save message: %v", err)
		return
	}

	// Broadcast to all clients in the room
	c.roomHub.BroadcastMessage(msg)
}

// handleReaction processes incoming reaction payloads
func (c *Client) handleReaction(wsMsg models.WebSocketMessage) {
	if strings.TrimSpace(wsMsg.MessageID) == "" || wsMsg.Emoji == "" {
		log.Printf("Invalid reaction: missing message_id or emoji")
		return
	}

	// Convert string message_id to uint
	messageID, err := strconv.ParseUint(wsMsg.MessageID, 10, 32)
	if err != nil {
		log.Printf("Invalid message_id format: %s", wsMsg.MessageID)
		return
	}

	// Block reactions in archived rooms
	if c.roomStatus == "archived" {
		log.Printf("%s attempted to react in archived room %s", c.User.Username, c.roomHub.roomID)
		return
	}

	reaction := &models.Reaction{
		ID:        generateID(),
		MessageID: strconv.FormatUint(messageID, 10),
		UserID:    c.User.ID,
		Username:  c.User.Username,
		Emoji:     wsMsg.Emoji,
	}

	// Save reaction to database (raw SQL)
	if err := database.SaveReaction(reaction); err != nil {
		log.Printf("Failed to save reaction: %v", err)
		return
	}

	// Broadcast reaction to all clients in the room
	c.roomHub.BroadcastReaction(reaction)
	log.Printf("%s reacted %s to message", c.User.Username, wsMsg.Emoji)
}

// Helper function to generate a unique ID (simplified UUID v4)
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// Helper function to generate random string
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
