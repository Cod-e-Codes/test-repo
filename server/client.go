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
	hub                  *Hub
	conn                 *websocket.Conn
	send                 chan interface{}
	db                   *sql.DB
	username             string
	isAdmin              bool
	ipAddr               string // Store IP address for logging and ban enforcement
	pluginCommandHandler *PluginCommandHandler
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
		// Handle admin commands
		if strings.HasPrefix(msg.Content, ":") {
			log.Printf("Command received from %s: %s (admin=%v)", c.username, msg.Content, c.isAdmin)
			if c.isAdmin {
				c.handleAdminCommand(msg.Content)
			} else {
				log.Printf("Unauthorized admin command attempt by %s: %s", c.username, msg.Content)
			}
			continue // Don't insert admin commands as normal messages
		}
		msg.CreatedAt = time.Now()
		if msg.Type == "" || msg.Type == shared.TextMessage {
			InsertMessage(c.db, msg)
		}
		c.hub.broadcast <- msg
	}
}

// parseCommandWithQuotes parses a command string, respecting quoted arguments
func parseCommandWithQuotes(command string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	escapeNext := false

	for _, char := range command {
		if escapeNext {
			current.WriteRune(char)
			escapeNext = false
			continue
		}

		if char == '\\' {
			escapeNext = true
			continue
		}

		if char == '"' {
			inQuotes = !inQuotes
			continue
		}

		if char == ' ' && !inQuotes {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(char)
		}
	}

	// Add the last part
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// handleAdminCommand processes admin commands
func (c *Client) handleAdminCommand(command string) {
	// Parse command with proper quote handling
	parts := parseCommandWithQuotes(command)
	if len(parts) == 0 {
		return
	}

	// First, try to handle plugin commands
	if c.pluginCommandHandler != nil {
		cmd := strings.TrimPrefix(parts[0], ":")
		args := parts[1:]

		log.Printf("[DEBUG] Trying plugin command: %s with args: %v", cmd, args)
		response, err := c.pluginCommandHandler.HandlePluginCommand(cmd, args, c.isAdmin)
		if err == nil {
			// Plugin command was handled successfully
			log.Printf("[DEBUG] Plugin command handled successfully: %s", response)
			c.send <- shared.Message{
				Sender:    "System",
				Content:   response,
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
			return
		} else {
			log.Printf("[DEBUG] Plugin command failed: %v", err)
		}
	} else {
		log.Printf("[DEBUG] No plugin command handler available")
	}

	// Fall back to built-in admin commands
	switch parts[0] {
	case ":cleardb":
		log.Printf("[ADMIN] Clearing message database via WebSocket by %s...", c.username)
		err := ClearMessages(c.db)
		if err != nil {
			log.Printf("Failed to clear DB: %v", err)
		} else {
			log.Printf("Message DB cleared by %s", c.username)
			c.hub.broadcast <- shared.Message{
				Sender:    "System",
				Content:   "Chat history cleared by admin.",
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
		}

	case ":kick":
		if len(parts) < 2 {
			c.send <- shared.Message{
				Sender:    "System",
				Content:   "Usage: :kick <username>",
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
			return
		}
		targetUsername := parts[1]
		c.hub.KickUser(targetUsername, c.username)
		c.send <- shared.Message{
			Sender:    "System",
			Content:   "User '" + targetUsername + "' has been kicked (24 hour temporary ban).",
			CreatedAt: time.Now(),
			Type:      shared.TextMessage,
		}

	case ":ban":
		if len(parts) < 2 {
			c.send <- shared.Message{
				Sender:    "System",
				Content:   "Usage: :ban <username>",
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
			return
		}
		targetUsername := parts[1]
		c.hub.BanUser(targetUsername, c.username)
		c.send <- shared.Message{
			Sender:    "System",
			Content:   "User '" + targetUsername + "' has been permanently banned.",
			CreatedAt: time.Now(),
			Type:      shared.TextMessage,
		}

	case ":unban":
		if len(parts) < 2 {
			c.send <- shared.Message{
				Sender:    "System",
				Content:   "Usage: :unban <username>",
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
			return
		}
		targetUsername := parts[1]
		unbanned := c.hub.UnbanUser(targetUsername, c.username)
		if unbanned {
			c.send <- shared.Message{
				Sender:    "System",
				Content:   "User '" + targetUsername + "' has been unbanned.",
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
		} else {
			c.send <- shared.Message{
				Sender:    "System",
				Content:   "User '" + targetUsername + "' was not found in the ban list.",
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
		}

	case ":allow":
		if len(parts) < 2 {
			c.send <- shared.Message{
				Sender:    "System",
				Content:   "Usage: :allow <username>",
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
			return
		}
		targetUsername := parts[1]
		allowed := c.hub.AllowUser(targetUsername, c.username)
		if allowed {
			c.send <- shared.Message{
				Sender:    "System",
				Content:   "User '" + targetUsername + "' has been allowed back (kick override).",
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
		} else {
			c.send <- shared.Message{
				Sender:    "System",
				Content:   "User '" + targetUsername + "' was not found in the kick list.",
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
		}

	case ":cleanup":
		log.Printf("[ADMIN] Manual stale connection cleanup initiated by %s", c.username)
		c.hub.CleanupStaleConnections()
		c.send <- shared.Message{
			Sender:    "System",
			Content:   "Stale connection cleanup completed.",
			CreatedAt: time.Now(),
			Type:      shared.TextMessage,
		}

	case ":forcedisconnect":
		if len(parts) < 2 {
			c.send <- shared.Message{
				Sender:    "System",
				Content:   "Usage: :forcedisconnect <username>",
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
			return
		}
		targetUsername := parts[1]
		disconnected := c.hub.ForceDisconnectUser(targetUsername, c.username)
		if disconnected {
			c.send <- shared.Message{
				Sender:    "System",
				Content:   "User '" + targetUsername + "' has been forcibly disconnected.",
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
		} else {
			c.send <- shared.Message{
				Sender:    "System",
				Content:   "User '" + targetUsername + "' was not found in active connections.",
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
		}

	default:
		log.Printf("[ADMIN] Unknown admin command by %s: %s", c.username, command)
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
