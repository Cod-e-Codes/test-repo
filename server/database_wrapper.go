package server

import (
	"database/sql"
	"log"

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

// GetDB returns the raw database connection for compatibility
func (w *DatabaseWrapper) GetDB() *sql.DB {
	return w.db.GetDB()
}

// InsertMessage provides backward compatibility for InsertMessage function
func (w *DatabaseWrapper) InsertMessage(msg shared.Message) {
	if err := w.db.InsertMessage(msg); err != nil {
		log.Println("Insert error:", err)
	}
}

// InsertEncryptedMessage provides backward compatibility for InsertEncryptedMessage function
func (w *DatabaseWrapper) InsertEncryptedMessage(msg *shared.EncryptedMessage) {
	if err := w.db.InsertEncryptedMessage(msg); err != nil {
		log.Println("Insert encrypted message error:", err)
	}
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
