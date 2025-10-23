package server

import (
	"database/sql"
	"time"

	"github.com/Cod-e-Codes/marchat/shared"
)

// Database interface defines the contract for database operations
type Database interface {
	// Connection management
	Open(config DatabaseConfig) error
	Close() error
	Ping() error

	// Schema management
	CreateSchema() error
	Migrate() error

	// Message operations
	InsertMessage(msg shared.Message) error
	InsertEncryptedMessage(msg *shared.EncryptedMessage) error
	GetRecentMessages() []shared.Message
	GetMessagesAfter(lastMessageID int64, limit int) []shared.Message
	GetRecentMessagesForUser(username string, defaultLimit int, banGapsHistory bool) ([]shared.Message, int64)
	ClearMessages() error

	// User state management
	GetUserLastMessageID(username string) (int64, error)
	SetUserLastMessageID(username string, messageID int64) error
	ClearUserMessageState(username string) error
	GetLatestMessageID() int64

	// Ban history
	RecordBanEvent(username, bannedBy string) error
	RecordUnbanEvent(username string) error
	GetUserBanPeriods(username string) ([]BanPeriod, error)

	// Statistics
	GetDatabaseStats() (string, error)
	BackupDatabase(dbPath string) (string, error)

	// Raw DB access for compatibility
	GetDB() *sql.DB
}

// DatabaseConfig holds configuration for database connections
type DatabaseConfig struct {
	Type     string // "sqlite", "postgres", "mysql"
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string

	// SQLite-specific
	FilePath string
}

// BanPeriod represents a period when a user was banned
type BanPeriod struct {
	BannedAt   time.Time
	UnbannedAt *time.Time
}
