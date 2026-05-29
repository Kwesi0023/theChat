package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/Kwesi0023/theChat/database"
	"github.com/Kwesi0023/theChat/models"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

type Client struct {
	conn       *websocket.Conn
	roomHub    *RoomHub
	User       *models.User
	roomStatus string
	Send       chan interface{}
}

func NewClient(conn *websocket.Conn, roomHub *RoomHub, user *models.User, roomStatus string) *Client {
	return &Client{
		conn:       conn,
		roomHub:    roomHub,
		User:       user,
		roomStatus: roomStatus,
		Send:       make(chan interface{}, 256),
	}
}

// (Server memory -> Browser stream)
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		//Listen to your private outbound mailbox channel
		case message, ok := <-c.Send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The Hub closed the channel because this client left
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.conn.WriteJSON(message)
			if err != nil {
				log.Printf("Write Error for user %s: %v", c.User.Username, err)
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

// (Browser stream -> Server memory)
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
			log.Printf("error unmarshaling payload: %v", err)
			continue
		}

		switch wsMsg.Type {
		case "message":
			wsMsg.Username = c.User.Username
			wsMsg.UserID = c.User.ID
			wsMsg.RoomID = c.roomHub.roomID
			wsMsg.Timestamp = time.Now()

			// Route it into the central room broadcast channel thread
			c.roomHub.broadcast <- wsMsg

		case "reaction":
			c.handleReaction(wsMsg)
		}
	}
}

func (c *Client) handleReaction(msg models.WebSocketMessage) {
	// Fallback placeholder logic for text emojis
	log.Printf("Reaction received from %s", c.User.Username)
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

/*
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
*/

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
