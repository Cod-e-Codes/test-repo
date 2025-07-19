package server

import (
	"database/sql"
	"encoding/json"
	"log"
	"marchat/shared"
	"net/http"
	"sort"
	"strings"

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
	// Enforce message cap: keep only the most recent 1000 messages
	_, err = db.Exec(`DELETE FROM messages WHERE id NOT IN (SELECT id FROM messages ORDER BY id DESC LIMIT 1000)`)
	if err != nil {
		log.Println("Error enforcing message cap:", err)
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
	sort.Strings(usernames) // Sort alphabetically
	userList := UserList{Users: usernames}
	payload, _ := json.Marshal(userList)
	msg := WSMessage{Type: "userlist", Data: payload}
	for client := range h.clients {
		client.send <- msg
	}
}

type adminAuth struct {
	admins   map[string]struct{}
	adminKey string
}

func ServeWs(hub *Hub, db *sql.DB, adminList []string, adminKey string) http.HandlerFunc {
	auth := adminAuth{admins: make(map[string]struct{}), adminKey: adminKey}
	for _, u := range adminList {
		auth.admins[strings.ToLower(u)] = struct{}{}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("WebSocket upgrade error:", err)
			return
		}
		// Expect handshake as first message
		var hs shared.Handshake
		err = conn.ReadJSON(&hs)
		if err != nil {
			conn.WriteMessage(websocket.CloseMessage, []byte("Invalid handshake"))
			conn.Close()
			return
		}
		username := strings.TrimSpace(hs.Username)
		if username == "" {
			conn.WriteMessage(websocket.CloseMessage, []byte("Username required"))
			conn.Close()
			return
		}
		lu := strings.ToLower(username)
		isAdmin := false
		if hs.Admin {
			if _, ok := auth.admins[lu]; !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte("Not an admin user"))
				conn.Close()
				return
			}
			if hs.AdminKey != auth.adminKey {
				conn.WriteMessage(websocket.CloseMessage, []byte("Invalid admin key"))
				conn.Close()
				return
			}
			isAdmin = true
		}
		// Check for duplicate username
		for client := range hub.clients {
			if strings.EqualFold(client.username, username) {
				conn.WriteMessage(websocket.CloseMessage, []byte("Username already taken"))
				conn.Close()
				return
			}
		}
		client := &Client{hub: hub, conn: conn, send: make(chan interface{}, 256), db: db, username: username, isAdmin: isAdmin}
		log.Printf("Client %s connected (admin=%v)", username, isAdmin)
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
