package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/Kwesi0023/theChat/models"
	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

// InitDB initializes the database connection
func InitDB(dsn string) error {
	var err error
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	err = DB.Ping()
	if err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * time.Minute)

	log.Println("Database connection established")
	return nil
}

// CreateTables verifies tables exist (assumes user created them manually)
func CreateTables() error {
	log.Println("Database tables verified (assumed to exist)")
	return nil
}

// CreateRoom inserts a new room into the database
func CreateRoom(room *models.Room) error {
	query := "INSERT INTO rooms (id, name, description, creator_id, status, type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)"
	// Default to 'active' status and 'public' type if not specified
	status := room.Status
	if status == "" {
		status = "active"
	}
	roomType := room.Type
	if roomType == "" {
		roomType = "public"
	}
	_, err := DB.Exec(query, room.ID, room.Name, room.Description, room.CreatorID, status, roomType, room.CreatedAt)
	return err
}

// GetRoom retrieves a room by ID with all fields
func GetRoom(roomID string) (*models.Room, error) {
	query := "SELECT id, name, description, creator_id, status, type, created_at FROM rooms WHERE id = ?"
	row := DB.QueryRow(query, roomID)

	room := &models.Room{}
	err := row.Scan(&room.ID, &room.Name, &room.Description, &room.CreatorID, &room.Status, &room.Type, &room.CreatedAt)
	if err != nil {
		return nil, err
	}

	return room, nil
}

// GetRoomStatus retrieves only the status of a room (raw SQL)
func GetRoomStatus(roomID string) (string, error) {
	query := "SELECT status FROM rooms WHERE id = ?"
	row := DB.QueryRow(query, roomID)

	var status string
	err := row.Scan(&status)
	if err != nil {
		return "", err
	}

	return status, nil
}

// GetAllRooms retrieves all rooms excluding 'hidden' status (shows 'active' and 'archived')
func GetAllRooms() ([]*models.Room, error) {
	query := "SELECT id, name, description, creator_id, status, type, created_at FROM rooms WHERE status IN ('active', 'archived') ORDER BY created_at DESC"
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*models.Room
	for rows.Next() {
		room := &models.Room{}
		err := rows.Scan(&room.ID, &room.Name, &room.Description, &room.CreatorID, &room.Status, &room.Type, &room.CreatedAt)
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}

	return rooms, rows.Err()
}

// SaveMessage saves a message to the database
func SaveMessage(msg *models.Message) error {
	query := "INSERT INTO messages (id, room_id, user_id, username, content, timestamp) VALUES (?, ?, ?, ?, ?, ?)"
	_, err := DB.Exec(query, msg.ID, msg.RoomID, msg.UserID, msg.Username, msg.Content, msg.Timestamp)
	return err
}

// SaveMessageWithType saves a message with msg_type to the database (raw SQL)
func SaveMessageWithType(msg *models.Message) error {
	query := "INSERT INTO messages (id, room_id, user_id, username, content, msg_type, created_at, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	createdAt := time.Now()
	_, err := DB.Exec(query, msg.ID, msg.RoomID, msg.UserID, msg.Username, msg.Content, msg.MsgType, createdAt, msg.Timestamp)
	return err
}

// SaveSilentJoinMessage silently logs a join event without broadcasting
func SaveSilentJoinMessage(roomID string, userID uint, username string) error {
	query := "INSERT INTO messages (id, room_id, user_id, username, content, msg_type, created_at, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	msgID := generateSystemMessageID()
	content := fmt.Sprintf("[%s] connected", username)
	now := time.Now()
	_, err := DB.Exec(query, msgID, roomID, userID, username, content, "join", now, now)
	return err
}

// SaveSilentLeaveMessage silently logs a leave event without broadcasting
func SaveSilentLeaveMessage(roomID string, userID uint, username string) error {
	query := "INSERT INTO messages (id, room_id, user_id, username, content, msg_type, created_at, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	msgID := generateSystemMessageID()
	content := fmt.Sprintf("[%s] disconnected", username)
	now := time.Now()
	_, err := DB.Exec(query, msgID, roomID, userID, username, content, "leave", now, now)
	return err
}

// Helper function to generate system message IDs
func generateSystemMessageID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return time.Now().Format("20060102150405") + "-sys-" + string(b)
}

// GetLastMessages retrieves the last N messages for a room in DESC order (newest first)
func GetLastMessages(roomID string, limit int) ([]*models.Message, error) {
	query := "SELECT id, room_id, user_id, username, content, msg_type, created_at, timestamp FROM messages WHERE room_id = ? ORDER BY created_at DESC LIMIT ?"
	rows, err := DB.Query(query, roomID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		err := rows.Scan(&msg.ID, &msg.RoomID, &msg.UserID, &msg.Username, &msg.Content, &msg.MsgType, &msg.CreatedAt, &msg.Timestamp)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// GetMessagesByRoom retrieves all messages for a room
func GetMessagesByRoom(roomID string, limit int) ([]*models.Message, error) {
	query := "SELECT id, room_id, user_id, username, content, timestamp FROM messages WHERE room_id = ? ORDER BY timestamp ASC LIMIT ?"
	rows, err := DB.Query(query, roomID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		err := rows.Scan(&msg.ID, &msg.RoomID, &msg.UserID, &msg.Username, &msg.Content, &msg.Timestamp)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// SaveReaction saves a reaction to a message (raw SQL)
func SaveReaction(reactions *models.Reaction) error {
	query := "INSERT INTO reactions (id, message_id, user_id, username, emoji, created_at) VALUES (?, ?, ?, ?, ?, ?)"
	createdAt := time.Now()
	_, err := DB.Exec(query, reactions.ID, reactions.MessageID, reactions.UserID, reactions.Username, reactions.Emoji, createdAt)
	return err
}

// GetReactionsByMessageID retrieves all reactions for a message (raw SQL)
func GetReactionsByMessageID(messageID string) ([]*models.Reaction, error) {
	query := "SELECT id, message_id, user_id, username, emoji, created_at FROM reactions WHERE message_id = ? ORDER BY created_at ASC"
	rows, err := DB.Query(query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []*models.Reaction
	for rows.Next() {
		reaction := &models.Reaction{}
		err := rows.Scan(&reaction.ID, &reaction.MessageID, &reaction.UserID, &reaction.Username, &reaction.Emoji, &reaction.CreatedAt)
		if err != nil {
			return nil, err
		}
		reactions = append(reactions, reaction)
	}

	return reactions, rows.Err()
}

// UpdateRoomStatus updates a room's status
func UpdateRoomStatus(roomID string, newStatus string) error {
	query := "UPDATE rooms SET status = ? WHERE id = ?"
	_, err := DB.Exec(query, newStatus, roomID)
	return err
}

// DeleteRoom removes a room from the database
func DeleteRoom(roomID string) error {
	query := "DELETE FROM rooms WHERE id = ?"
	_, err := DB.Exec(query, roomID)
	return err
}

// CloseDB closes the database connection
func CloseDB() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
