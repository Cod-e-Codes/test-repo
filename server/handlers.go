package server

import (
	"database/sql"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"

	"github.com/Cod-e-Codes/marchat/shared"

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

// getClientIP extracts the real IP address from the request
func getClientIP(r *http.Request) string {
	// Check for forwarded headers first (for proxy/reverse proxy scenarios)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if comma := strings.Index(xff, ","); comma != -1 {
			return strings.TrimSpace(xff[:comma])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Fall back to remote address
	if r.RemoteAddr != "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			return host
		}
		return r.RemoteAddr
	}
	return "unknown"
}

func CreateSchema(db *sql.DB) {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sender TEXT,
		content TEXT,
		created_at DATETIME,
		is_encrypted BOOLEAN DEFAULT 0,
		encrypted_data BLOB,
		nonce BLOB,
		recipient TEXT
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

// InsertEncryptedMessage stores an encrypted message in the database
func InsertEncryptedMessage(db *sql.DB, encryptedMsg *shared.EncryptedMessage) {
	_, err := db.Exec(`INSERT INTO messages (sender, content, created_at, is_encrypted, encrypted_data, nonce, recipient) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		encryptedMsg.Sender, encryptedMsg.Content, encryptedMsg.CreatedAt,
		encryptedMsg.IsEncrypted, encryptedMsg.Encrypted, encryptedMsg.Nonce, encryptedMsg.Recipient)
	if err != nil {
		log.Println("Insert encrypted message error:", err)
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
			if err := conn.WriteMessage(websocket.CloseMessage, []byte("Invalid handshake")); err != nil {
				log.Printf("WriteMessage error: %v", err)
			}
			conn.Close()
			return
		}
		username := strings.TrimSpace(hs.Username)
		if username == "" {
			if err := conn.WriteMessage(websocket.CloseMessage, []byte("Username required")); err != nil {
				log.Printf("WriteMessage error: %v", err)
			}
			conn.Close()
			return
		}
		lu := strings.ToLower(username)
		isAdmin := false
		if hs.Admin {
			if _, ok := auth.admins[lu]; !ok {
				if err := conn.WriteMessage(websocket.CloseMessage, []byte("Not an admin user")); err != nil {
					log.Printf("WriteMessage error: %v", err)
				}
				conn.Close()
				return
			}
			if hs.AdminKey != auth.adminKey {
				// Send auth_failed message before closing
				failMsg, _ := json.Marshal(map[string]string{"reason": "invalid admin key"})
				if err := conn.WriteJSON(WSMessage{Type: "auth_failed", Data: failMsg}); err != nil {
					log.Printf("WriteMessage error: %v", err)
				}
				conn.Close()
				return
			}
			isAdmin = true
		}
		// Check for duplicate username
		for client := range hub.clients {
			if strings.EqualFold(client.username, username) {
				if err := conn.WriteMessage(websocket.CloseMessage, []byte("Username already taken")); err != nil {
					log.Printf("WriteMessage error: %v", err)
				}
				conn.Close()
				return
			}
		}
		// Extract IP address
		ipAddr := getClientIP(r)

		// Check if user is banned
		if hub.IsUserBanned(username) {
			log.Printf("Banned user '%s' (IP: %s) attempted to connect", username, ipAddr)
			if err := conn.WriteMessage(websocket.CloseMessage, []byte("You are banned from this server")); err != nil {
				log.Printf("WriteMessage error: %v", err)
			}
			conn.Close()
			return
		}

		client := &Client{hub: hub, conn: conn, send: make(chan interface{}, 256), db: db, username: username, isAdmin: isAdmin, ipAddr: ipAddr}
		log.Printf("Client %s connected (admin=%v, IP: %s)", username, isAdmin, ipAddr)
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
