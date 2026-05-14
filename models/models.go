package models

import "time"

// Message represents a chat message
type Message struct {
	ID        uint       `json:"id"`
	RoomID    uint       `json:"room_id"`
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
	RoomID   uint   `json:"room_id"`
}
