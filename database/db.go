package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/Kwesi0023/theChat/models"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

var DB *sql.DB

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

// i did not use gorm.. created entire database myself
func CreateTables() error {
	log.Println("Tables created successfully")
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

// GetRoom retrieves a room by ID with all fields -- --GetRoomByID would be equal to GetRooms
func GetRoomByID(roomID string) (*models.Room, error) {
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

// RoomNameExists checks if a room name is already present in the database
func RoomNameExists(name string) (bool, error) {
	query := "SELECT COUNT(*) FROM rooms WHERE name = ?"
	var count int
	err := DB.QueryRow(query, name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check room name uniqueness: %w", err)
	}
	return count > 0, nil
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
		err := rows.Scan(
			&room.ID, &room.Name, &room.Description, &room.CreatorID, &room.Status, &room.Type, &room.CreatedAt)
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}

	return rooms, rows.Err()
}

func SaveMessage(msg *models.Message) error {
	query := "INSERT INTO messages (id, content, sender_id, room_id, created_at, msg_type) VALUES (?, ?, ?, ?, ?, ?)"
	_, err := DB.Exec(query, msg.ID, msg.Content, msg.SenderID, msg.RoomID, msg.Timestamp, msg.MsgType)
	return err
}

// GetLastMessages retrieves the last N messages for a room in DESC order (newest first)
func GetLastMessages(roomID string, limit int) ([]*models.Message, error) {
	query := "SELECT id, room_id, sender_id, content, msg_type, created_at, timestamp FROM messages WHERE room_id = ? ORDER BY created_at DESC LIMIT ?"
	rows, err := DB.Query(query, roomID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		err := rows.Scan(&msg.ID, &msg.RoomID, &msg.SenderID, &msg.Content, &msg.MsgType, &msg.CreatedAt, &msg.Timestamp)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// GetChatHistory fetches the last 50 messages for a specific room
func GetChatHistory(roomID string, limit int) ([]*models.Message, error) {
	// Query: Sort by oldest first so the chat flows naturally
	query := `SELECT id, content, sender_id, room_id, timestamp FROM messages WHERE room_id = ? ORDER BY timestamp ASC LIMIT ?`

	rows, err := DB.Query(query, roomID, limit) // Assuming 'db' is your *sql.DB connection
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []*models.Message

	for rows.Next() {
		msg := &models.Message{}
		// Scan into the pointer. Ensure your models.Message fields match these!
		err := rows.Scan(&msg.ID, &msg.Content, &msg.SenderID, &msg.RoomID, &msg.Timestamp)
		if err != nil {
			log.Printf("Error scanning message history: %v", err)
			continue
		}
		history = append(history, msg)
	}

	return history, nil
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
		err := rows.Scan(&msg.ID, &msg.RoomID, &msg.SenderID, &msg.Content, &msg.Timestamp)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// SaveReaction saves a reaction to a message (raw SQL)
func SaveReaction(reactions *models.Reaction) error {
	query := "INSERT INTO reactions (id, message_id, user_id, username, emoji, content, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)"
	createdAt := time.Now()
	_, err := DB.Exec(query, reactions.ID, reactions.MessageID, reactions.UserID, reactions.Username, reactions.Emoji, reactions.Content, createdAt)
	return err
}

// GetReactionsByMessageID retrieves all reactions for a message (raw SQL)
func GetReactionsByMessageID(messageID string) ([]*models.Reaction, error) {
	query := "SELECT id, message_id, user_id, username, emoji, content, created_at FROM reactions WHERE message_id = ? ORDER BY created_at ASC"
	rows, err := DB.Query(query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []*models.Reaction
	for rows.Next() {
		reaction := &models.Reaction{}
		err := rows.Scan(&reaction.ID, &reaction.MessageID, &reaction.UserID, &reaction.Username, &reaction.Emoji, &reaction.Content, &reaction.CreatedAt)
		if err != nil {
			return nil, err
		}
		reactions = append(reactions, reaction)
	}

	return reactions, rows.Err()
}

// GetMessageByID retrieves a single message row from MySQL to inspect its text content
func GetMessageByID(messageID string) (*models.Message, error) {
	var msg models.Message
	query := "SELECT id, room_id, sender_id, content, created_at FROM messages WHERE id = ?"

	err := DB.QueryRow(query, messageID).Scan(&msg.ID, &msg.RoomID, &msg.SenderID, &msg.Content, &msg.Timestamp)
	if err != nil {
		return nil, err
	}
	return &msg, nil
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
	result, err := DB.Exec(query, roomID)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("room not found")
	}
	return nil
}

// CloseDB closes the database connection
func CloseDB() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

// RegisterUser hashes a password and inserts a new user record into the database
func RegisterUser(username, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	query := "INSERT INTO users (username, password_hash) VALUES (?, ?)"
	_, err = DB.Exec(query, username, string(hashedPassword))
	if err != nil {
		return fmt.Errorf("failed to insert user into database: %w", err)
	}

	return nil
}

// AuthenticateUser retrieves a user by username and verifies the password hash
func AuthenticateUser(username, password string) (*models.User, error) {
	query := "SELECT id, username, password_hash FROM users WHERE username = ?"
	row := DB.QueryRow(query, username)

	var user models.User
	var passwordHash string

	err := row.Scan(&user.ID, &user.Username, &passwordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found") // Caught by our handler
		}
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	return &user, nil
}

//'5228
