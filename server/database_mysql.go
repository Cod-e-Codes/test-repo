package server

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Cod-e-Codes/marchat/shared"
	_ "github.com/go-sql-driver/mysql"
)

// MySQLDB implements the Database interface for MySQL
type MySQLDB struct {
	db *sql.DB
}

// NewMySQLDB creates a new MySQL database instance
func NewMySQLDB() *MySQLDB {
	return &MySQLDB{}
}

// Open establishes a connection to the MySQL database
func (m *MySQLDB) Open(config DatabaseConfig) error {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true",
		config.Username, config.Password,
		config.Host, config.Port, config.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}

	m.db = db
	return nil
}

// Close closes the database connection
func (m *MySQLDB) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// Ping tests the database connection
func (m *MySQLDB) Ping() error {
	return m.db.Ping()
}

// CreateSchema creates the database schema
func (m *MySQLDB) CreateSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id INT AUTO_INCREMENT PRIMARY KEY,
		message_id INT DEFAULT 0,
		sender TEXT,
		content TEXT,
		created_at DATETIME,
		is_encrypted BOOLEAN DEFAULT false,
		encrypted_data BLOB,
		nonce BLOB,
		recipient TEXT
	);
	
	CREATE TABLE IF NOT EXISTS user_message_state (
		username VARCHAR(255) PRIMARY KEY,
		last_message_id INT NOT NULL DEFAULT 0,
		last_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE TABLE IF NOT EXISTS ban_history (
		id INT AUTO_INCREMENT PRIMARY KEY,
		username VARCHAR(255) NOT NULL,
		banned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		unbanned_at DATETIME,
		banned_by VARCHAR(255) NOT NULL,
		INDEX(username, banned_at)
	);
	
	CREATE INDEX idx_messages_message_id ON messages(message_id);
	CREATE INDEX idx_messages_created_at ON messages(created_at);
	CREATE INDEX idx_user_message_state_username ON user_message_state(username);
	CREATE INDEX idx_ban_history_username ON ban_history(username);
	CREATE INDEX idx_ban_history_banned_at ON ban_history(banned_at);
	CREATE INDEX idx_ban_history_unbanned_at ON ban_history(unbanned_at);
	`

	_, err := m.db.Exec(schema)
	if err != nil {
		return err
	}

	// Check if message_id column exists, if not add it
	var columnExists int
	err = m.db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name='messages' AND column_name='message_id' AND table_schema=DATABASE()`).Scan(&columnExists)
	if err != nil {
		log.Printf("Warning: failed to check for message_id column: %v", err)
	}

	if columnExists == 0 {
		// Add message_id column to existing table
		_, err = m.db.Exec(`ALTER TABLE messages ADD COLUMN message_id INT DEFAULT 0`)
		if err != nil {
			log.Printf("Warning: failed to add message_id column: %v", err)
		} else {
			log.Printf("Added message_id column to messages table")
		}
	}

	// Migration: Update existing messages to have message_id = id
	_, err = m.db.Exec(`UPDATE messages SET message_id = id WHERE message_id = 0 OR message_id IS NULL`)
	if err != nil {
		log.Printf("Warning: failed to migrate existing messages: %v", err)
	} else {
		log.Printf("Successfully migrated existing messages")
	}

	return nil
}

// Migrate performs database migrations
func (m *MySQLDB) Migrate() error {
	// For now, migrations are handled in CreateSchema
	// This can be expanded to handle versioned migrations
	return nil
}

// InsertMessage inserts a new message into the database
func (m *MySQLDB) InsertMessage(msg shared.Message) error {
	result, err := m.db.Exec(`INSERT INTO messages (sender, content, created_at, is_encrypted) VALUES (?, ?, ?, ?)`,
		msg.Sender, msg.Content, msg.CreatedAt, msg.Encrypted)
	if err != nil {
		return err
	}

	// Get the inserted ID and update message_id
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	_, err = m.db.Exec(`UPDATE messages SET message_id = ? WHERE id = ?`, id, id)
	if err != nil {
		return err
	}

	// Enforce message cap: keep only the most recent 1000 messages
	_, err = m.db.Exec(`DELETE FROM messages WHERE id NOT IN (SELECT id FROM messages ORDER BY id DESC LIMIT 1000)`)
	if err != nil {
		log.Printf("Error enforcing message cap: %v", err)
	}

	return nil
}

// InsertEncryptedMessage stores an encrypted message in the database
func (m *MySQLDB) InsertEncryptedMessage(msg *shared.EncryptedMessage) error {
	result, err := m.db.Exec(`INSERT INTO messages (sender, content, created_at, is_encrypted, encrypted_data, nonce, recipient) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.Sender, msg.Content, msg.CreatedAt, true, msg.Encrypted, msg.Nonce, msg.Recipient)
	if err != nil {
		return err
	}

	// Get the inserted ID and update message_id
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	_, err = m.db.Exec(`UPDATE messages SET message_id = ? WHERE id = ?`, id, id)
	if err != nil {
		return err
	}

	// Enforce message cap: keep only the most recent 1000 messages
	_, err = m.db.Exec(`DELETE FROM messages WHERE id NOT IN (SELECT id FROM messages ORDER BY id DESC LIMIT 1000)`)
	if err != nil {
		log.Printf("Error enforcing message cap: %v", err)
	}

	return nil
}

// GetRecentMessages retrieves the most recent messages
func (m *MySQLDB) GetRecentMessages() []shared.Message {
	rows, err := m.db.Query(`SELECT sender, content, created_at, is_encrypted FROM messages ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		log.Println("Query error:", err)
		return nil
	}
	defer rows.Close()

	var messages []shared.Message
	for rows.Next() {
		var msg shared.Message
		var isEncrypted bool
		err := rows.Scan(&msg.Sender, &msg.Content, &msg.CreatedAt, &isEncrypted)
		if err == nil {
			msg.Encrypted = isEncrypted
			messages = append(messages, msg)
		}
	}

	// Sort messages by timestamp for consistent chronological display
	sortMessagesByTimestamp(messages)
	return messages
}

// GetMessagesAfter retrieves messages with ID > lastMessageID
func (m *MySQLDB) GetMessagesAfter(lastMessageID int64, limit int) []shared.Message {
	rows, err := m.db.Query(`SELECT sender, content, created_at, is_encrypted FROM messages WHERE message_id > ? ORDER BY created_at DESC LIMIT ?`, lastMessageID, limit)
	if err != nil {
		log.Println("Query error in GetMessagesAfter:", err)
		return nil
	}
	defer rows.Close()

	var messages []shared.Message
	for rows.Next() {
		var msg shared.Message
		var isEncrypted bool
		err := rows.Scan(&msg.Sender, &msg.Content, &msg.CreatedAt, &isEncrypted)
		if err == nil {
			msg.Encrypted = isEncrypted
			messages = append(messages, msg)
		}
	}

	// Sort messages by timestamp for consistent chronological display
	sortMessagesByTimestamp(messages)
	return messages
}

// GetRecentMessagesForUser returns personalized message history for a specific user
func (m *MySQLDB) GetRecentMessagesForUser(username string, defaultLimit int, banGapsHistory bool) ([]shared.Message, int64) {
	lowerUsername := strings.ToLower(username)

	// Get user's last seen message ID
	lastMessageID, err := m.GetUserLastMessageID(lowerUsername)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error getting last message ID for user %s: %v", username, err)
		// Fall back to recent messages for new users or on error
		messages := m.GetRecentMessages()
		sortMessagesByTimestamp(messages) // Ensure consistent ordering
		return messages, 0
	}

	var messages []shared.Message

	if lastMessageID == 0 {
		// New user or no history - get recent messages
		messages = m.GetRecentMessages()
	} else {
		// Returning user - get messages after their last seen ID
		messages = m.GetMessagesAfter(lastMessageID, defaultLimit)

		// If they have few new messages, combine with recent history
		if len(messages) < defaultLimit/2 {
			recentMessages := m.GetRecentMessages()
			// Combine recent messages with new messages, avoiding duplicates
			existingIDs := make(map[string]bool)
			for _, msg := range messages {
				key := msg.Sender + ":" + msg.Content + ":" + msg.CreatedAt.Format("2006-01-02 15:04:05")
				existingIDs[key] = true
			}

			for _, msg := range recentMessages {
				key := msg.Sender + ":" + msg.Content + ":" + msg.CreatedAt.Format("2006-01-02 15:04:05")
				if !existingIDs[key] && len(messages) < defaultLimit {
					messages = append(messages, msg)
				}
			}
		}
	}

	// Sort messages by timestamp for consistent chronological display
	sortMessagesByTimestamp(messages)

	// Filter messages during ban periods if feature is enabled
	if banGapsHistory {
		banPeriods, err := m.GetUserBanPeriods(lowerUsername)
		if err != nil {
			log.Printf("Warning: failed to get ban periods for user %s: %v", username, err)
		} else if len(banPeriods) > 0 {
			// Filter out messages sent during ban periods
			filteredMessages := make([]shared.Message, 0, len(messages))
			for _, msg := range messages {
				if !isMessageInBanPeriod(msg.CreatedAt, banPeriods) {
					filteredMessages = append(filteredMessages, msg)
				}
			}
			messages = filteredMessages
			log.Printf("Filtered %d messages for user %s due to ban history gaps", len(messages), username)
		}
	}

	// Update user's last seen message ID
	if len(messages) > 0 {
		latestID := m.GetLatestMessageID()
		if latestID > 0 {
			err = m.SetUserLastMessageID(lowerUsername, latestID)
			if err != nil {
				log.Printf("Warning: failed to update last message ID for user %s: %v", username, err)
			}
		}
	}

	return messages, lastMessageID
}

// ClearMessages removes all messages from the database
func (m *MySQLDB) ClearMessages() error {
	_, err := m.db.Exec(`DELETE FROM messages`)
	return err
}

// GetUserLastMessageID queries user_message_state table
func (m *MySQLDB) GetUserLastMessageID(username string) (int64, error) {
	var lastMessageID int64
	err := m.db.QueryRow(`SELECT last_message_id FROM user_message_state WHERE username = ?`, username).Scan(&lastMessageID)
	return lastMessageID, err
}

// SetUserLastMessageID INSERT OR REPLACE into user_message_state
func (m *MySQLDB) SetUserLastMessageID(username string, messageID int64) error {
	_, err := m.db.Exec(`INSERT INTO user_message_state (username, last_message_id, last_seen) VALUES (?, ?, NOW()) ON DUPLICATE KEY UPDATE last_message_id = VALUES(last_message_id), last_seen = NOW()`, username, messageID)
	return err
}

// ClearUserMessageState deletes user's record from user_message_state
func (m *MySQLDB) ClearUserMessageState(username string) error {
	_, err := m.db.Exec(`DELETE FROM user_message_state WHERE username = ?`, username)
	return err
}

// GetLatestMessageID returns MAX(id) from messages table
func (m *MySQLDB) GetLatestMessageID() int64 {
	var latestID int64
	err := m.db.QueryRow(`SELECT MAX(id) FROM messages`).Scan(&latestID)
	if err != nil {
		// Handle empty table case
		return 0
	}
	return latestID
}

// RecordBanEvent records a ban event in the ban_history table
func (m *MySQLDB) RecordBanEvent(username, bannedBy string) error {
	_, err := m.db.Exec(`INSERT INTO ban_history (username, banned_by) VALUES (?, ?)`, username, bannedBy)
	if err != nil {
		log.Printf("Warning: failed to record ban event for user %s: %v", username, err)
	}
	return err
}

// RecordUnbanEvent records an unban event in the ban_history table
func (m *MySQLDB) RecordUnbanEvent(username string) error {
	_, err := m.db.Exec(`UPDATE ban_history SET unbanned_at = NOW() WHERE username = ? AND unbanned_at IS NULL`, username)
	if err != nil {
		log.Printf("Warning: failed to record unban event for user %s: %v", username, err)
	}
	return err
}

// GetUserBanPeriods retrieves all ban periods for a user
func (m *MySQLDB) GetUserBanPeriods(username string) ([]BanPeriod, error) {
	rows, err := m.db.Query(`SELECT banned_at, unbanned_at FROM ban_history WHERE username = ? ORDER BY banned_at ASC`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var periods []BanPeriod

	for rows.Next() {
		var bannedAt time.Time
		var unbannedAt *time.Time
		err := rows.Scan(&bannedAt, &unbannedAt)
		if err != nil {
			log.Printf("Warning: failed to scan ban period for user %s: %v", username, err)
			continue
		}
		periods = append(periods, BanPeriod{
			BannedAt:   bannedAt,
			UnbannedAt: unbannedAt,
		})
	}

	return periods, nil
}

// GetDatabaseStats returns database statistics
func (m *MySQLDB) GetDatabaseStats() (string, error) {
	var messageCount, userCount, banCount int

	err := m.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&messageCount)
	if err != nil {
		return "", err
	}

	err = m.db.QueryRow(`SELECT COUNT(*) FROM user_message_state`).Scan(&userCount)
	if err != nil {
		return "", err
	}

	err = m.db.QueryRow(`SELECT COUNT(*) FROM ban_history`).Scan(&banCount)
	if err != nil {
		return "", err
	}

	stats := fmt.Sprintf("Messages: %d, Users: %d, Ban Events: %d", messageCount, userCount, banCount)
	return stats, nil
}

// BackupDatabase creates a backup of the current database
func (m *MySQLDB) BackupDatabase(dbPath string) (string, error) {
	// MySQL backup would typically use mysqldump
	// For now, return a placeholder message
	backupFilename := fmt.Sprintf("mysql_backup_%s.sql", time.Now().Format("2006-01-02_15-04-05"))
	return backupFilename, fmt.Errorf("MySQL backup not implemented - use mysqldump manually")
}

// GetDB returns the raw database connection for compatibility
func (m *MySQLDB) GetDB() *sql.DB {
	return m.db
}
