package server

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Cod-e-Codes/marchat/shared"
	_ "modernc.org/sqlite"
)

// SQLiteDB implements the Database interface for SQLite
type SQLiteDB struct {
	db *sql.DB
}

// NewSQLiteDB creates a new SQLite database instance
func NewSQLiteDB() *SQLiteDB {
	return &SQLiteDB{}
}

// Open establishes a connection to the SQLite database
func (s *SQLiteDB) Open(config DatabaseConfig) error {
	db, err := sql.Open("sqlite", config.FilePath)
	if err != nil {
		return err
	}

	// Enable WAL mode for better concurrency and performance
	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		return fmt.Errorf("failed to enable WAL: %w", err)
	}

	// Performance optimizations
	_, _ = db.Exec("PRAGMA synchronous=NORMAL;")
	_, _ = db.Exec("PRAGMA cache_size=10000;")
	_, _ = db.Exec("PRAGMA temp_store=MEMORY;")

	s.db = db
	return nil
}

// Close closes the database connection
func (s *SQLiteDB) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Ping tests the database connection
func (s *SQLiteDB) Ping() error {
	return s.db.Ping()
}

// CreateSchema creates the database schema
func (s *SQLiteDB) CreateSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id INTEGER DEFAULT 0,
		sender TEXT,
		content TEXT,
		created_at DATETIME,
		is_encrypted BOOLEAN DEFAULT 0,
		encrypted_data BLOB,
		nonce BLOB,
		recipient TEXT
	);
	
	CREATE TABLE IF NOT EXISTS user_message_state (
		username TEXT PRIMARY KEY,
		last_message_id INTEGER NOT NULL DEFAULT 0,
		last_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE TABLE IF NOT EXISTS ban_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		banned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		unbanned_at DATETIME,
		banned_by TEXT NOT NULL
	);
	
	CREATE INDEX IF NOT EXISTS idx_messages_message_id ON messages(message_id);
	CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
	CREATE INDEX IF NOT EXISTS idx_user_message_state_username ON user_message_state(username);
	CREATE INDEX IF NOT EXISTS idx_ban_history_username ON ban_history(username);
	CREATE INDEX IF NOT EXISTS idx_ban_history_banned_at ON ban_history(banned_at);
	CREATE INDEX IF NOT EXISTS idx_ban_history_unbanned_at ON ban_history(unbanned_at);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	// Check if message_id column exists, if not add it
	var columnExists int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name='message_id'`).Scan(&columnExists)
	if err != nil {
		log.Printf("Warning: failed to check for message_id column: %v", err)
	}

	if columnExists == 0 {
		// Add message_id column to existing table
		_, err = s.db.Exec(`ALTER TABLE messages ADD COLUMN message_id INTEGER DEFAULT 0`)
		if err != nil {
			log.Printf("Warning: failed to add message_id column: %v", err)
		} else {
			log.Printf("Added message_id column to messages table")
		}
	}

	// Migration: Update existing messages to have message_id = id
	_, err = s.db.Exec(`UPDATE messages SET message_id = id WHERE message_id = 0 OR message_id IS NULL`)
	if err != nil {
		log.Printf("Warning: failed to migrate existing messages: %v", err)
	} else {
		log.Printf("Successfully migrated existing messages")
	}

	return nil
}

// Migrate performs database migrations
func (s *SQLiteDB) Migrate() error {
	// For now, migrations are handled in CreateSchema
	// This can be expanded to handle versioned migrations
	return nil
}

// InsertMessage inserts a new message into the database
func (s *SQLiteDB) InsertMessage(msg shared.Message) error {
	result, err := s.db.Exec(`INSERT INTO messages (sender, content, created_at, is_encrypted) VALUES (?, ?, ?, ?)`,
		msg.Sender, msg.Content, msg.CreatedAt, msg.Encrypted)
	if err != nil {
		return err
	}

	// Get the inserted ID and update message_id
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE messages SET message_id = ? WHERE id = ?`, id, id)
	if err != nil {
		return err
	}

	// Enforce message cap: keep only the most recent 1000 messages
	_, err = s.db.Exec(`DELETE FROM messages WHERE id NOT IN (SELECT id FROM messages ORDER BY id DESC LIMIT 1000)`)
	if err != nil {
		log.Printf("Error enforcing message cap: %v", err)
	}

	return nil
}

// InsertEncryptedMessage stores an encrypted message in the database
func (s *SQLiteDB) InsertEncryptedMessage(msg *shared.EncryptedMessage) error {
	result, err := s.db.Exec(`INSERT INTO messages (sender, content, created_at, is_encrypted, encrypted_data, nonce, recipient) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.Sender, msg.Content, msg.CreatedAt, true, msg.Encrypted, msg.Nonce, msg.Recipient)
	if err != nil {
		return err
	}

	// Get the inserted ID and update message_id
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE messages SET message_id = ? WHERE id = ?`, id, id)
	if err != nil {
		return err
	}

	// Enforce message cap: keep only the most recent 1000 messages
	_, err = s.db.Exec(`DELETE FROM messages WHERE id NOT IN (SELECT id FROM messages ORDER BY id DESC LIMIT 1000)`)
	if err != nil {
		log.Printf("Error enforcing message cap: %v", err)
	}

	return nil
}

// GetRecentMessages retrieves the most recent messages
func (s *SQLiteDB) GetRecentMessages() []shared.Message {
	rows, err := s.db.Query(`SELECT sender, content, created_at, is_encrypted FROM messages ORDER BY created_at DESC LIMIT 50`)
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
func (s *SQLiteDB) GetMessagesAfter(lastMessageID int64, limit int) []shared.Message {
	rows, err := s.db.Query(`SELECT sender, content, created_at, is_encrypted FROM messages WHERE message_id > ? ORDER BY created_at DESC LIMIT ?`, lastMessageID, limit)
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
func (s *SQLiteDB) GetRecentMessagesForUser(username string, defaultLimit int, banGapsHistory bool) ([]shared.Message, int64) {
	lowerUsername := strings.ToLower(username)

	// Get user's last seen message ID
	lastMessageID, err := s.GetUserLastMessageID(lowerUsername)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error getting last message ID for user %s: %v", username, err)
		// Fall back to recent messages for new users or on error
		messages := s.GetRecentMessages()
		sortMessagesByTimestamp(messages) // Ensure consistent ordering
		return messages, 0
	}

	var messages []shared.Message

	if lastMessageID == 0 {
		// New user or no history - get recent messages
		messages = s.GetRecentMessages()
	} else {
		// Returning user - get messages after their last seen ID
		messages = s.GetMessagesAfter(lastMessageID, defaultLimit)

		// If they have few new messages, combine with recent history
		if len(messages) < defaultLimit/2 {
			recentMessages := s.GetRecentMessages()
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
		banPeriods, err := s.GetUserBanPeriods(lowerUsername)
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
		latestID := s.GetLatestMessageID()
		if latestID > 0 {
			err = s.SetUserLastMessageID(lowerUsername, latestID)
			if err != nil {
				log.Printf("Warning: failed to update last message ID for user %s: %v", username, err)
			}
		}
	}

	return messages, lastMessageID
}

// ClearMessages removes all messages from the database
func (s *SQLiteDB) ClearMessages() error {
	_, err := s.db.Exec(`DELETE FROM messages`)
	return err
}

// GetUserLastMessageID queries user_message_state table
func (s *SQLiteDB) GetUserLastMessageID(username string) (int64, error) {
	var lastMessageID int64
	err := s.db.QueryRow(`SELECT last_message_id FROM user_message_state WHERE username = ?`, username).Scan(&lastMessageID)
	return lastMessageID, err
}

// SetUserLastMessageID INSERT OR REPLACE into user_message_state
func (s *SQLiteDB) SetUserLastMessageID(username string, messageID int64) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO user_message_state (username, last_message_id, last_seen) VALUES (?, ?, CURRENT_TIMESTAMP)`, username, messageID)
	return err
}

// ClearUserMessageState deletes user's record from user_message_state
func (s *SQLiteDB) ClearUserMessageState(username string) error {
	_, err := s.db.Exec(`DELETE FROM user_message_state WHERE username = ?`, username)
	return err
}

// GetLatestMessageID returns MAX(id) from messages table
func (s *SQLiteDB) GetLatestMessageID() int64 {
	var latestID int64
	err := s.db.QueryRow(`SELECT MAX(id) FROM messages`).Scan(&latestID)
	if err != nil {
		// Handle empty table case
		return 0
	}
	return latestID
}

// RecordBanEvent records a ban event in the ban_history table
func (s *SQLiteDB) RecordBanEvent(username, bannedBy string) error {
	_, err := s.db.Exec(`INSERT INTO ban_history (username, banned_by) VALUES (?, ?)`, username, bannedBy)
	if err != nil {
		log.Printf("Warning: failed to record ban event for user %s: %v", username, err)
	}
	return err
}

// RecordUnbanEvent records an unban event in the ban_history table
func (s *SQLiteDB) RecordUnbanEvent(username string) error {
	_, err := s.db.Exec(`UPDATE ban_history SET unbanned_at = CURRENT_TIMESTAMP WHERE username = ? AND unbanned_at IS NULL`, username)
	if err != nil {
		log.Printf("Warning: failed to record unban event for user %s: %v", username, err)
	}
	return err
}

// GetUserBanPeriods retrieves all ban periods for a user
func (s *SQLiteDB) GetUserBanPeriods(username string) ([]BanPeriod, error) {
	rows, err := s.db.Query(`SELECT banned_at, unbanned_at FROM ban_history WHERE username = ? ORDER BY banned_at ASC`, username)
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
func (s *SQLiteDB) GetDatabaseStats() (string, error) {
	var messageCount, userCount, banCount int

	err := s.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&messageCount)
	if err != nil {
		return "", err
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM user_message_state`).Scan(&userCount)
	if err != nil {
		return "", err
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM ban_history`).Scan(&banCount)
	if err != nil {
		return "", err
	}

	stats := fmt.Sprintf("Messages: %d, Users: %d, Ban Events: %d", messageCount, userCount, banCount)
	return stats, nil
}

// BackupDatabase creates a backup of the current database
func (s *SQLiteDB) BackupDatabase(dbPath string) (string, error) {
	// For WAL mode, we need to checkpoint the WAL file to ensure all data is in the main file
	_, err := s.db.Exec("PRAGMA wal_checkpoint(FULL);")
	if err != nil {
		log.Printf("Warning: WAL checkpoint failed during backup: %v", err)
		// Continue with backup even if checkpoint fails
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFilename := fmt.Sprintf("marchat_backup_%s.db", timestamp)

	// Get directory of the original database
	backupPath := dbPath + ".backup." + timestamp

	// Execute VACUUM INTO to create a clean backup
	_, err = s.db.Exec(fmt.Sprintf("VACUUM INTO '%s';", backupPath))
	if err != nil {
		return "", fmt.Errorf("failed to create database backup: %v", err)
	}

	return backupFilename, nil
}

// GetDB returns the raw database connection for compatibility
func (s *SQLiteDB) GetDB() *sql.DB {
	return s.db
}

// Helper functions are defined in handlers.go to avoid duplication
