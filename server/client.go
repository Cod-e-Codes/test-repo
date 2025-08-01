package server

import (
	"database/sql"
	"log"
	"strings"
	"time"

	"github.com/Cod-e-Codes/marchat/shared"

	"github.com/gorilla/websocket"
)

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10 // send pings at 90% of pongWait
)

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan interface{}
	db       *sql.DB
	username string
	isAdmin  bool // new
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(1024*1024 + 512) // allow up to 1MB+ for file messages
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Printf("SetReadDeadline error: %v", err)
	}
	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			log.Printf("SetReadDeadline error: %v", err)
		}
		return nil
	})
	for {
		var msg shared.Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseAbnormalClosure) {
				log.Printf("Client %s disconnected unexpectedly: %v", c.username, err)
			} else {
				log.Printf("Client %s disconnected normally", c.username)
			}
			break
		}
		if msg.Type == shared.FileMessageType && msg.File != nil {
			// File message: enforce 1MB limit
			if msg.File.Size > 1024*1024 {
				log.Printf("Rejected file from %s: too large (%d bytes)", c.username, msg.File.Size)
				continue
			}
			// Broadcast file message, do not store in DB
			msg.CreatedAt = time.Now()
			c.hub.broadcast <- msg
			continue
		}
		// Handle :cleardb command
		if msg.Content == ":cleardb" {
			if c.isAdmin {
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
						Type:      shared.TextMessage,
					}
				}
			} else {
				log.Printf("Unauthorized cleardb attempt by %s\n", c.username)
			}
			continue // Don't insert this as a normal message
		}
		msg.CreatedAt = time.Now()
		if msg.Type == "" || msg.Type == shared.TextMessage {
			InsertMessage(c.db, msg)
		}
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
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					if !strings.Contains(err.Error(), "use of closed network connection") {
						log.Printf("WriteMessage error: %v", err)
					}
				}
				return
			}
			switch v := msg.(type) {
			case shared.Message:
				err := c.conn.WriteJSON(v)
				if err != nil {
					if !strings.Contains(err.Error(), "use of closed network connection") {
						log.Printf("Failed to send message to %s: %v", c.username, err)
					}
					return
				}
			case WSMessage:
				err := c.conn.WriteJSON(v)
				if err != nil {
					if !strings.Contains(err.Error(), "use of closed network connection") {
						log.Printf("Failed to send system message to %s: %v", c.username, err)
					}
					return
				}
			default:
				log.Printf("Unknown message type for client %s", c.username)
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				if !strings.Contains(err.Error(), "use of closed network connection") {
					log.Printf("Failed to send ping to %s: %v", c.username, err)
				}
				return
			}
		}
	}
}
