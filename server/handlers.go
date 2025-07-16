package server

import (
	"database/sql"
	"log"

	"marchat/shared"
)

func CreateSchema(db *sql.DB) {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sender TEXT,
		content TEXT,
		created_at DATETIME
	);`
	_, err := db.Exec(schema)
	if err != nil {
		log.Fatal("failed to create schema:", err)
	}
}

func InsertMessage(db *sql.DB, msg shared.Message) {
	_, err := db.Exec(`INSERT INTO messages (sender, content, created_at) VALUES (?, ?, ?)`,
		msg.Sender, msg.Content, msg.CreatedAt)
	if err != nil {
		log.Println("Insert error:", err)
	}
}

func GetRecentMessages(db *sql.DB) []shared.Message {
	rows, err := db.Query(`SELECT sender, content, created_at FROM messages ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		log.Println("Query error:", err)
		return nil
	}
	defer rows.Close()

	var messages []shared.Message
	for rows.Next() {
		var msg shared.Message
		err := rows.Scan(&msg.Sender, &msg.Content, &msg.CreatedAt)
		if err == nil {
			messages = append([]shared.Message{msg}, messages...)
		}
	}
	return messages
}
