package server

import (
	"database/sql"

	"github.com/Cod-e-Codes/marchat/shared"
)

// DatabaseWrapper provides backward compatibility for existing code
// that expects direct database function calls
type DatabaseWrapper struct {
	db Database
}

// NewDatabaseWrapper creates a new database wrapper
func NewDatabaseWrapper(db Database) *DatabaseWrapper {
	return &DatabaseWrapper{db: db}
}

// Close closes the database connection
func (w *DatabaseWrapper) Close() error {
	return w.db.Close()
}

// Open opens the database connection
func (w *DatabaseWrapper) Open(config DatabaseConfig) error {
	return w.db.Open(config)
}

// Ping checks the database connection
func (w *DatabaseWrapper) Ping() error {
	return w.db.Ping()
}

// CreateSchema creates the database schema
func (w *DatabaseWrapper) CreateSchema() error {
	return w.db.CreateSchema()
}

// Migrate runs database migrations
func (w *DatabaseWrapper) Migrate() error {
	return w.db.Migrate()
}

// GetDB returns the raw database connection for compatibility
func (w *DatabaseWrapper) GetDB() *sql.DB {
	return w.db.GetDB()
}

// InsertMessage provides backward compatibility for InsertMessage function
func (w *DatabaseWrapper) InsertMessage(msg shared.Message) error {
	return w.db.InsertMessage(msg)
}

// InsertEncryptedMessage provides backward compatibility for InsertEncryptedMessage function
func (w *DatabaseWrapper) InsertEncryptedMessage(msg *shared.EncryptedMessage) error {
	return w.db.InsertEncryptedMessage(msg)
}

// GetRecentMessages provides backward compatibility for GetRecentMessages function
func (w *DatabaseWrapper) GetRecentMessages() []shared.Message {
	return w.db.GetRecentMessages()
}

// GetMessagesAfter provides backward compatibility for GetMessagesAfter function
func (w *DatabaseWrapper) GetMessagesAfter(lastMessageID int64, limit int) []shared.Message {
	return w.db.GetMessagesAfter(lastMessageID, limit)
}

// GetRecentMessagesForUser provides backward compatibility for GetRecentMessagesForUser function
func (w *DatabaseWrapper) GetRecentMessagesForUser(username string, defaultLimit int, banGapsHistory bool) ([]shared.Message, int64) {
	return w.db.GetRecentMessagesForUser(username, defaultLimit, banGapsHistory)
}

// ClearMessages provides backward compatibility for ClearMessages function
func (w *DatabaseWrapper) ClearMessages() error {
	return w.db.ClearMessages()
}

// GetDatabaseStats provides backward compatibility for GetDatabaseStats function
func (w *DatabaseWrapper) GetDatabaseStats() (string, error) {
	return w.db.GetDatabaseStats()
}

// BackupDatabase provides backward compatibility for BackupDatabase function
func (w *DatabaseWrapper) BackupDatabase(dbPath string) (string, error) {
	return w.db.BackupDatabase(dbPath)
}

// RecordBanEvent provides backward compatibility for recordBanEvent function
func (w *DatabaseWrapper) RecordBanEvent(username, bannedBy string) error {
	return w.db.RecordBanEvent(username, bannedBy)
}

// RecordUnbanEvent provides backward compatibility for recordUnbanEvent function
func (w *DatabaseWrapper) RecordUnbanEvent(username string) error {
	return w.db.RecordUnbanEvent(username)
}

// GetUserBanPeriods provides backward compatibility for getUserBanPeriods function
func (w *DatabaseWrapper) GetUserBanPeriods(username string) ([]BanPeriod, error) {
	return w.db.GetUserBanPeriods(username)
}

// GetUserLastMessageID provides backward compatibility for getUserLastMessageID function
func (w *DatabaseWrapper) GetUserLastMessageID(username string) (int64, error) {
	return w.db.GetUserLastMessageID(username)
}

// SetUserLastMessageID provides backward compatibility for setUserLastMessageID function
func (w *DatabaseWrapper) SetUserLastMessageID(username string, messageID int64) error {
	return w.db.SetUserLastMessageID(username, messageID)
}

// ClearUserMessageState provides backward compatibility for clearUserMessageState function
func (w *DatabaseWrapper) ClearUserMessageState(username string) error {
	return w.db.ClearUserMessageState(username)
}

// GetLatestMessageID provides backward compatibility for getLatestMessageID function
func (w *DatabaseWrapper) GetLatestMessageID() int64 {
	return w.db.GetLatestMessageID()
}

// Query provides direct access to the underlying database Query method
func (w *DatabaseWrapper) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return w.db.GetDB().Query(query, args...)
}

// QueryRow provides direct access to the underlying database QueryRow method
func (w *DatabaseWrapper) QueryRow(query string, args ...interface{}) *sql.Row {
	return w.db.GetDB().QueryRow(query, args...)
}
