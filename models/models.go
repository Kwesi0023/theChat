package models

import "time"

// Room represents a chat room
type Room struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"` // Username of room creator	Status      string    `json:"status"` // 'active', 'archived', 'hidden'
	Type        string    `json:"type"`       // 'public', 'private'

}

// Message represents a chat message
type Message struct {
	ID        string     `json:"id"`
	RoomID    string     `json:"room_id"`
	UserID    uint       `json:"user_id"`
	Username  string     `json:"username"`
	Content   string     `json:"content"`
	MsgType   string     `json:"msg_type"` // 'message', 'join', 'leave', 'offline'
	Timestamp time.Time  `json:"timestamp"`
	CreatedAt time.Time  `json:"created_at"`
	Reactions []Reaction `json:"reactions"` // Nested slice for the history fetch
}

// User represents an active user in a room
type User struct {
	ID       uint      `json:"id"`
	Username string    `json:"username"`
	RoomID   uint      `json:"room_id"`
	JoinedAt time.Time `json:"joined_at"`
	Is_Admin bool      `json:"is_admin"`
}

// AuthRequest represents the authentication payload for WebSocket
type AuthRequest struct {
	Username string `json:"username"`
	RoomID   string `json:"room_id"`
}

// WebSocketMessage represents a message sent through WebSocket
type WebSocketMessage struct {
	Type      string     `json:"type"` // "message", "join", "leave", "user_list", "reaction", "history", "system", "offline"
	Content   string     `json:"content,omitempty"`
	Username  string     `json:"username,omitempty"`
	UserID    uint       `json:"user_id,omitempty"`
	RoomID    string     `json:"room_id,omitempty"`
	MsgType   string     `json:"msg_type,omitempty"` // for system messages
	Users     []User     `json:"users,omitempty"`
	Messages  []Message  `json:"messages,omitempty"` // for history
	Reactions []Reaction `json:"reactions,omitempty"`
	MessageID uint       `json:"message_id,omitempty"` // for reactions
	Emoji     string     `json:"emoji,omitempty"`      // for reactions "fire", "heart"
	Timestamp time.Time  `json:"timestamp,omitempty"`
}
