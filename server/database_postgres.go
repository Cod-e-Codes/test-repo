package server

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Cod-e-Codes/marchat/shared"
	_ "github.com/lib/pq"
)

// PostgresDB implements the Database interface for PostgreSQL
type PostgresDB struct {
	db *sql.DB
}

// NewPostgresDB creates a new PostgreSQL database instance
func NewPostgresDB() *PostgresDB {
	return &PostgresDB{}
}

// Open establishes a connection to the PostgreSQL database
func (p *PostgresDB) Open(config DatabaseConfig) error {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.Username,
		config.Password, config.Database, config.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}

	p.db = db
	return nil
}

// Close closes the database connection
func (p *PostgresDB) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// Ping tests the database connection
func (p *PostgresDB) Ping() error {
	return p.db.Ping()
}

// CreateSchema creates the database schema
func (p *PostgresDB) CreateSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id SERIAL PRIMARY KEY,
		message_id INTEGER DEFAULT 0,
		sender TEXT,
		content TEXT,
		created_at TIMESTAMP,
		is_encrypted BOOLEAN DEFAULT false,
		encrypted_data BYTEA,
		nonce BYTEA,
		recipient TEXT
	);
	
	CREATE TABLE IF NOT EXISTS user_message_state (
		username TEXT PRIMARY KEY,
		last_message_id INTEGER NOT NULL DEFAULT 0,
		last_seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE TABLE IF NOT EXISTS ban_history (
		id SERIAL PRIMARY KEY,
		username TEXT NOT NULL,
		banned_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		unbanned_at TIMESTAMP,
		banned_by TEXT NOT NULL
	);
	
	CREATE INDEX IF NOT EXISTS idx_messages_message_id ON messages(message_id);
	CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
	CREATE INDEX IF NOT EXISTS idx_user_message_state_username ON user_message_state(username);
	CREATE INDEX IF NOT EXISTS idx_ban_history_username ON ban_history(username);
	CREATE INDEX IF NOT EXISTS idx_ban_history_banned_at ON ban_history(banned_at);
	CREATE INDEX IF NOT EXISTS idx_ban_history_unbanned_at ON ban_history(unbanned_at);
	`

	_, err := p.db.Exec(schema)
	if err != nil {
		return err
	}

	// Check if message_id column exists, if not add it
	var columnExists int
	err = p.db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name='messages' AND column_name='message_id'`).Scan(&columnExists)
	if err != nil {
		log.Printf("Warning: failed to check for message_id column: %v", err)
	}

	if columnExists == 0 {
		// Add message_id column to existing table
		_, err = p.db.Exec(`ALTER TABLE messages ADD COLUMN message_id INTEGER DEFAULT 0`)
		if err != nil {
			log.Printf("Warning: failed to add message_id column: %v", err)
		} else {
			log.Printf("Added message_id column to messages table")
		}
	}

	// Migration: Update existing messages to have message_id = id
	_, err = p.db.Exec(`UPDATE messages SET message_id = id WHERE message_id = 0 OR message_id IS NULL`)
	if err != nil {
		log.Printf("Warning: failed to migrate existing messages: %v", err)
	} else {
		log.Printf("Successfully migrated existing messages")
	}

	return nil
}

// Migrate performs database migrations
func (p *PostgresDB) Migrate() error {
	// For now, migrations are handled in CreateSchema
	// This can be expanded to handle versioned migrations
	return nil
}

// InsertMessage inserts a new message into the database
func (p *PostgresDB) InsertMessage(msg shared.Message) error {
	var id int64
	err := p.db.QueryRow(`INSERT INTO messages (sender, content, created_at, is_encrypted) VALUES ($1, $2, $3, $4) RETURNING id`,
		msg.Sender, msg.Content, msg.CreatedAt, msg.Encrypted).Scan(&id)
	if err != nil {
		return err
	}

	// Update message_id to match id
	_, err = p.db.Exec(`UPDATE messages SET message_id = $1 WHERE id = $1`, id)
	if err != nil {
		return err
	}

	// Enforce message cap: keep only the most recent 1000 messages
	_, err = p.db.Exec(`DELETE FROM messages WHERE id NOT IN (SELECT id FROM messages ORDER BY id DESC LIMIT 1000)`)
	if err != nil {
		log.Printf("Error enforcing message cap: %v", err)
	}

	return nil
}

// InsertEncryptedMessage stores an encrypted message in the database
func (p *PostgresDB) InsertEncryptedMessage(msg *shared.EncryptedMessage) error {
	var id int64
	err := p.db.QueryRow(`INSERT INTO messages (sender, content, created_at, is_encrypted, encrypted_data, nonce, recipient) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		msg.Sender, msg.Content, msg.CreatedAt, true, msg.Encrypted, msg.Nonce, msg.Recipient).Scan(&id)
	if err != nil {
		return err
	}

	// Update message_id to match id
	_, err = p.db.Exec(`UPDATE messages SET message_id = $1 WHERE id = $1`, id)
	if err != nil {
		return err
	}

	// Enforce message cap: keep only the most recent 1000 messages
	_, err = p.db.Exec(`DELETE FROM messages WHERE id NOT IN (SELECT id FROM messages ORDER BY id DESC LIMIT 1000)`)
	if err != nil {
		log.Printf("Error enforcing message cap: %v", err)
	}

	return nil
}

// GetRecentMessages retrieves the most recent messages
func (p *PostgresDB) GetRecentMessages() []shared.Message {
	rows, err := p.db.Query(`SELECT sender, content, created_at, is_encrypted FROM messages ORDER BY created_at DESC LIMIT 50`)
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
func (p *PostgresDB) GetMessagesAfter(lastMessageID int64, limit int) []shared.Message {
	rows, err := p.db.Query(`SELECT sender, content, created_at, is_encrypted FROM messages WHERE message_id > $1 ORDER BY created_at DESC LIMIT $2`, lastMessageID, limit)
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
func (p *PostgresDB) GetRecentMessagesForUser(username string, defaultLimit int, banGapsHistory bool) ([]shared.Message, int64) {
	lowerUsername := strings.ToLower(username)

	// Get user's last seen message ID
	lastMessageID, err := p.GetUserLastMessageID(lowerUsername)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error getting last message ID for user %s: %v", username, err)
		// Fall back to recent messages for new users or on error
		messages := p.GetRecentMessages()
		sortMessagesByTimestamp(messages) // Ensure consistent ordering
		return messages, 0
	}

	var messages []shared.Message

	if lastMessageID == 0 {
		// New user or no history - get recent messages
		messages = p.GetRecentMessages()
	} else {
		// Returning user - get messages after their last seen ID
		messages = p.GetMessagesAfter(lastMessageID, defaultLimit)

		// If they have few new messages, combine with recent history
		if len(messages) < defaultLimit/2 {
			recentMessages := p.GetRecentMessages()
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
		banPeriods, err := p.GetUserBanPeriods(lowerUsername)
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
		latestID := p.GetLatestMessageID()
		if latestID > 0 {
			err = p.SetUserLastMessageID(lowerUsername, latestID)
			if err != nil {
				log.Printf("Warning: failed to update last message ID for user %s: %v", username, err)
			}
		}
	}

	return messages, lastMessageID
}

// ClearMessages removes all messages from the database
func (p *PostgresDB) ClearMessages() error {
	_, err := p.db.Exec(`DELETE FROM messages`)
	return err
}

// GetUserLastMessageID queries user_message_state table
func (p *PostgresDB) GetUserLastMessageID(username string) (int64, error) {
	var lastMessageID int64
	err := p.db.QueryRow(`SELECT last_message_id FROM user_message_state WHERE username = $1`, username).Scan(&lastMessageID)
	return lastMessageID, err
}

// SetUserLastMessageID INSERT OR REPLACE into user_message_state
func (p *PostgresDB) SetUserLastMessageID(username string, messageID int64) error {
	_, err := p.db.Exec(`INSERT INTO user_message_state (username, last_message_id, last_seen) VALUES ($1, $2, CURRENT_TIMESTAMP) ON CONFLICT (username) DO UPDATE SET last_message_id = $2, last_seen = CURRENT_TIMESTAMP`, username, messageID)
	return err
}

// ClearUserMessageState deletes user's record from user_message_state
func (p *PostgresDB) ClearUserMessageState(username string) error {
	_, err := p.db.Exec(`DELETE FROM user_message_state WHERE username = $1`, username)
	return err
}

// GetLatestMessageID returns MAX(id) from messages table
func (p *PostgresDB) GetLatestMessageID() int64 {
	var latestID int64
	err := p.db.QueryRow(`SELECT MAX(id) FROM messages`).Scan(&latestID)
	if err != nil {
		// Handle empty table case
		return 0
	}
	return latestID
}

// RecordBanEvent records a ban event in the ban_history table
func (p *PostgresDB) RecordBanEvent(username, bannedBy string) error {
	_, err := p.db.Exec(`INSERT INTO ban_history (username, banned_by) VALUES ($1, $2)`, username, bannedBy)
	if err != nil {
		log.Printf("Warning: failed to record ban event for user %s: %v", username, err)
	}
	return err
}

// RecordUnbanEvent records an unban event in the ban_history table
func (p *PostgresDB) RecordUnbanEvent(username string) error {
	_, err := p.db.Exec(`UPDATE ban_history SET unbanned_at = CURRENT_TIMESTAMP WHERE username = $1 AND unbanned_at IS NULL`, username)
	if err != nil {
		log.Printf("Warning: failed to record unban event for user %s: %v", username, err)
	}
	return err
}

// GetUserBanPeriods retrieves all ban periods for a user
func (p *PostgresDB) GetUserBanPeriods(username string) ([]BanPeriod, error) {
	rows, err := p.db.Query(`SELECT banned_at, unbanned_at FROM ban_history WHERE username = $1 ORDER BY banned_at ASC`, username)
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
func (p *PostgresDB) GetDatabaseStats() (string, error) {
	var messageCount, userCount, banCount int

	err := p.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&messageCount)
	if err != nil {
		return "", err
	}

	err = p.db.QueryRow(`SELECT COUNT(*) FROM user_message_state`).Scan(&userCount)
	if err != nil {
		return "", err
	}

	err = p.db.QueryRow(`SELECT COUNT(*) FROM ban_history`).Scan(&banCount)
	if err != nil {
		return "", err
	}

	stats := fmt.Sprintf("Messages: %d, Users: %d, Ban Events: %d", messageCount, userCount, banCount)
	return stats, nil
}

// BackupDatabase creates a backup of the current database
func (p *PostgresDB) BackupDatabase(dbPath string) (string, error) {
	// PostgreSQL backup would typically use pg_dump
	// For now, return a placeholder message
	backupFilename := fmt.Sprintf("postgres_backup_%s.sql", time.Now().Format("2006-01-02_15-04-05"))
	return backupFilename, fmt.Errorf("PostgreSQL backup not implemented - use pg_dump manually")
}

// GetDB returns the raw database connection for compatibility
func (p *PostgresDB) GetDB() *sql.DB {
	return p.db
}
