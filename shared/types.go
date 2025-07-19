package shared

import "time"

type Message struct {
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Handshake is sent by the client on WebSocket connect for authentication
// Admin key is only sent if admin is true
// Username is always sent (case-insensitive match on server)
type Handshake struct {
	Username string `json:"username"`
	Admin    bool   `json:"admin"`
	AdminKey string `json:"admin_key,omitempty"`
}
