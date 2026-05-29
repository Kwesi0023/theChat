package websocket

import (
	"encoding/json"
	"fmt"
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
	Send       chan interface{}
}

// NewClient creates a new client instance with room status
func NewClient(conn *websocket.Conn, roomHub *RoomHub, user *models.User, roomStatus string) *Client {
	return &Client{
		conn:       conn,
		roomHub:    roomHub,
		User:       user,
		roomStatus: roomStatus,
		Send:       make(chan interface{}, 256),
	}
}

// Start initiates the client's read and write goroutines
func (c *Client) Start() {
	go c.ReadPump()
	go c.WritePump()
}

// readPump reads messages from the WebSocket connection and broadcasts them to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.roomHub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		var wsMsg models.WebSocketMessage
		if err := json.Unmarshal(messageBytes, &wsMsg); err != nil {
			log.Printf("error unmarshaling message: %v", err)
			continue
		}

		// Look for your type switch block inside the loop
		switch wsMsg.Type {
		case "message":
			wsMsg.Username = c.User.Username
			wsMsg.UserID = c.User.ID
			wsMsg.RoomID = c.roomHub.roomID
			wsMsg.Timestamp = time.Now()

			// 1. Generate the fresh numeric ID string
			msgIDStr := fmt.Sprintf("%d", time.Now().UnixMilli())
			wsMsg.MessageID = msgIDStr

			// 2. Build your clean database transaction model
			dbMessage := &models.Message{
				ID:        msgIDStr, // Passes your clean pure numeric timestamp string
				RoomID:    wsMsg.RoomID,
				UserID:    wsMsg.UserID,
				Username:  wsMsg.Username,
				Content:   wsMsg.Content,
				MsgType:   "message",
				Timestamp: wsMsg.Timestamp,
				CreatedAt: wsMsg.Timestamp,
			}

			// Save the formatted record directly to your database table
			if err := database.SaveMessage(dbMessage); err != nil {
				log.Printf("Failed to save message row to MySQL: %v", err)
			} else {
				log.Printf("Success! Message from '%s' saved safely into MySQL table.", wsMsg.Username)
			}

			c.roomHub.broadcast <- wsMsg

		case "reaction":
			c.handleReaction(wsMsg)
		}
	}

}

// writePump writes messages from the hub's broadcast channel to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
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
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				data, err := json.Marshal(<-c.Send)
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
		c.Send <- errorMsg
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

func (c *Client) handleHistory(wsMsg models.WebSocketMessage) {
	if wsMsg.RoomID == "" {
		log.Println("History request failed: RoomID is empty")
		return
	}

	// 1. Fetch history from the DB.. 50 for now
	history, err := database.GetChatHistory(wsMsg.RoomID, 50)
	if err != nil {
		log.Printf("Could not retrieve history for room %s: %v", wsMsg.RoomID, err)
		return
	}

	// the wsmsg struct
	response := models.WebSocketMessage{
		Type:     "history",
		RoomID:   wsMsg.RoomID,
		Messages: history,
	}

	// 3. Send ONLY to the client who requested it
	c.Send <- response
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
