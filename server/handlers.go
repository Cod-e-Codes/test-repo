package server

import (
	"database/sql"
	"encoding/json"
	"log"
	"marchat/shared"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type UserList struct {
	Users []string `json:"users"`
}

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
	rows, err := db.Query(`SELECT sender, content, created_at FROM messages ORDER BY created_at ASC LIMIT 50`)
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
			messages = append(messages, msg)
		}
	}
	return messages
}

func ClearMessages(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM messages`)
	return err
}

func (h *Hub) broadcastUserList() {
	usernames := []string{}
	for client := range h.clients {
		if client.username != "" {
			usernames = append(usernames, client.username)
		}
	}
	userList := UserList{Users: usernames}
	payload, _ := json.Marshal(userList)
	msg := WSMessage{Type: "userlist", Data: payload}
	for client := range h.clients {
		client.send <- msg
	}
}

func ServeWs(hub *Hub, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Query().Get("username")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("WebSocket upgrade error:", err)
			return
		}
		client := &Client{hub: hub, conn: conn, send: make(chan interface{}, 256), db: db, username: username}
		hub.register <- client

		// Send recent messages to new client
		msgs := GetRecentMessages(db)
		for _, msg := range msgs {
			client.send <- msg
		}
		hub.broadcastUserList()

		// Start read/write pumps
		go client.writePump()
		go client.readPump()
	}
}
