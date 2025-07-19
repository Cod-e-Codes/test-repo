package server

import (
	"database/sql"
	"log"
	"marchat/shared"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10 // send pings at 90% of pongWait
)

const adminSecret = "changeme" // Set this securely in production

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan interface{}
	db       *sql.DB
	username string
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		var msg shared.Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			// Check if this is a normal disconnect vs an actual error
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Client %s disconnected unexpectedly: %v", c.username, err)
			} else {
				log.Printf("Client %s disconnected", c.username)
			}
			break
		}
		// Handle :cleardb command
		if msg.Content == ":cleardb" || strings.HasPrefix(msg.Content, ":cleardb ") {
			parts := strings.Fields(msg.Content)
			if msg.Sender == "admin" && (len(parts) == 1 || (len(parts) == 2 && parts[1] == adminSecret)) {
				log.Println("[ADMIN] Clearing message database via WebSocket...")
				err := ClearMessages(c.db)
				if err != nil {
					log.Println("Failed to clear DB:", err)
				} else {
					log.Println("Message DB cleared.")
					c.hub.broadcast <- shared.Message{
						Sender:    "System",
						Content:   "Chat history cleared by admin.",
						CreatedAt: time.Now(),
					}
				}
			} else {
				log.Printf("Unauthorized cleardb attempt by %s\n", msg.Sender)
			}
			continue // Don't insert this as a normal message
		}
		msg.CreatedAt = time.Now()
		InsertMessage(c.db, msg)
		c.hub.broadcast <- msg
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// Channel closed, send close message
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			switch v := msg.(type) {
			case shared.Message:
				err := c.conn.WriteJSON(v)
				if err != nil {
					log.Printf("Failed to send message to %s: %v", c.username, err)
					return
				}
			case WSMessage:
				err := c.conn.WriteJSON(v)
				if err != nil {
					log.Printf("Failed to send system message to %s: %v", c.username, err)
					return
				}
			default:
				log.Printf("Unknown message type for client %s", c.username)
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Failed to send ping to %s: %v", c.username, err)
				return
			}
		}
	}
}
