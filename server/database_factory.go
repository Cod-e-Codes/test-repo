package server

import (
	"fmt"
)

// NewDatabase creates a new database instance based on the configuration
func NewDatabase(config DatabaseConfig) (Database, error) {
	var db Database

	switch config.Type {
	case "sqlite":
		db = NewSQLiteDB()
	case "postgres", "postgresql":
		db = NewPostgresDB()
	case "mysql":
		db = NewMySQLDB()
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}

	if err := db.Open(config); err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.CreateSchema(); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return db, nil
}
