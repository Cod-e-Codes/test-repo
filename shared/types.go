package shared

import "time"

// MessageType distinguishes between text and file messages
// (add more types as needed)
type MessageType string

const (
	TextMessage     MessageType = "text"
	FileMessageType MessageType = "file"
)

type Message struct {
	Sender    string      `json:"sender"`
	Content   string      `json:"content"`
	CreatedAt time.Time   `json:"created_at"`
	Type      MessageType `json:"type,omitempty"`
	Encrypted bool        `json:"encrypted,omitempty"` // Indicates if content is encrypted
	// For file messages, Content is empty and File is set
	File *FileMeta `json:"file,omitempty"`
}

type FileMeta struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Data     []byte `json:"data"` // raw bytes (base64-encoded in JSON)
}

// Handshake is sent by the client on WebSocket connect for authentication
// Admin key is only sent if admin is true
// Username is always sent (case-insensitive match on server)
type Handshake struct {
	Username string `json:"username"`
	Admin    bool   `json:"admin"`
	AdminKey string `json:"admin_key,omitempty"`
}
