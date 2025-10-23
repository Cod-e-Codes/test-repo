package server

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// CreateTestDatabase creates a test database and returns a DatabaseWrapper
func CreateTestDatabase(t *testing.T) Database {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create schema
	CreateSchema(db)

	// Create SQLiteDB instance and set the db
	sqliteDB := NewSQLiteDB()
	sqliteDB.db = db

	// Wrap in DatabaseWrapper
	return NewDatabaseWrapper(sqliteDB)
}

// CreateTestHub creates a test hub with a test database
func CreateTestHub(t *testing.T) (*Hub, Database) {
	db := CreateTestDatabase(t)
	hub := NewHub("./plugins", "./data", "http://registry.example.com", db)
	return hub, db
}
