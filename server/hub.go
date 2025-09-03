package server

import (
	"database/sql"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Cod-e-Codes/marchat/plugin/manager"
	"github.com/Cod-e-Codes/marchat/shared"
	"github.com/gorilla/websocket"
)

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan interface{}
	register   chan *Client
	unregister chan *Client

	// Ban management
	bans     map[string]time.Time // username -> ban expiry time
	banMutex sync.RWMutex

	// Plugin management
	pluginManager        *manager.PluginManager
	pluginCommandHandler *PluginCommandHandler

	// Database reference for message state management
	db *sql.DB
}

func NewHub(pluginDir, dataDir, registryURL string, db *sql.DB) *Hub {
	pluginManager := manager.NewPluginManager(pluginDir, dataDir, registryURL)
	pluginCommandHandler := NewPluginCommandHandler(pluginManager)

	return &Hub{
		clients:              make(map[*Client]bool),
		broadcast:            make(chan interface{}),
		register:             make(chan *Client),
		unregister:           make(chan *Client),
		bans:                 make(map[string]time.Time),
		pluginManager:        pluginManager,
		pluginCommandHandler: pluginCommandHandler,
		db:                   db,
	}
}

// BanUser adds a user to the ban list
func (h *Hub) BanUser(username string, adminUsername string) {
	h.banMutex.Lock()
	defer h.banMutex.Unlock()

	// Ban for 24 hours by default
	h.bans[strings.ToLower(username)] = time.Now().Add(24 * time.Hour)
	log.Printf("[ADMIN] User '%s' banned by '%s' until %s", username, adminUsername, time.Now().Add(24*time.Hour).Format("2006-01-02 15:04:05"))

	// Record ban event in database
	if h.getDB() != nil {
		err := recordBanEvent(h.getDB(), strings.ToLower(username), adminUsername)
		if err != nil {
			log.Printf("Warning: failed to record ban event for user %s: %v", username, err)
		}
	}

	// Clear user's message state to ensure fresh history on unban
	if h.getDB() != nil {
		err := clearUserMessageState(h.getDB(), strings.ToLower(username))
		if err != nil {
			log.Printf("Warning: failed to clear message state for banned user %s: %v", username, err)
		}
	}

	// Kick the user if they're currently connected
	h.kickUser(username, "You have been banned by an administrator")
}

// UnbanUser removes a user from the ban list
func (h *Hub) UnbanUser(username string, adminUsername string) bool {
	h.banMutex.Lock()
	defer h.banMutex.Unlock()

	lowerUsername := strings.ToLower(username)
	if _, exists := h.bans[lowerUsername]; exists {
		delete(h.bans, lowerUsername)
		log.Printf("[ADMIN] User '%s' unbanned by '%s'", username, adminUsername)

		// Record unban event in database
		if h.getDB() != nil {
			err := recordUnbanEvent(h.getDB(), lowerUsername)
			if err != nil {
				log.Printf("Warning: failed to record unban event for user %s: %v", username, err)
			}
		}

		// Clear user's message state to ensure clean slate on reconnection
		if h.getDB() != nil {
			err := clearUserMessageState(h.getDB(), lowerUsername)
			if err != nil {
				log.Printf("Warning: failed to clear message state for unbanned user %s: %v", username, err)
			}
		}

		return true
	}
	log.Printf("[ADMIN] Unban attempt for '%s' by '%s' - user not found in ban list", username, adminUsername)
	return false
}

// IsUserBanned checks if a user is currently banned
func (h *Hub) IsUserBanned(username string) bool {
	h.banMutex.RLock()
	defer h.banMutex.RUnlock()

	lowerUsername := strings.ToLower(username)
	if banTime, exists := h.bans[lowerUsername]; exists {
		if time.Now().Before(banTime) {
			return true
		}
		// Ban has expired, remove it
		delete(h.bans, lowerUsername)
	}
	return false
}

// kickUser forcibly disconnects a user by username
func (h *Hub) kickUser(username string, reason string) {
	for client := range h.clients {
		if strings.EqualFold(client.username, username) {
			log.Printf("[ADMIN] Kicking user '%s' (IP: %s) - Reason: %s", username, client.ipAddr, reason)

			// Send kick message to the user
			kickMsg := shared.Message{
				Sender:    "System",
				Content:   "You have been kicked by an administrator: " + reason,
				CreatedAt: time.Now(),
				Type:      shared.TextMessage,
			}
			client.send <- kickMsg

			// Close the connection
			client.conn.Close()
			return
		}
	}
	log.Printf("[ADMIN] Kick attempt for '%s' - user not found", username)
}

// KickUser kicks a user by username (admin command)
func (h *Hub) KickUser(username string, adminUsername string) {
	log.Printf("[ADMIN] User '%s' kicked by '%s'", username, adminUsername)
	h.kickUser(username, "Kicked by administrator")
}

// CleanupExpiredBans removes expired bans from the ban list
func (h *Hub) CleanupExpiredBans() {
	h.banMutex.Lock()
	defer h.banMutex.Unlock()

	now := time.Now()
	for username, banTime := range h.bans {
		if now.After(banTime) {
			delete(h.bans, username)
			log.Printf("[SYSTEM] Expired ban removed for user: %s", username)
		}
	}
}

// CleanupStaleConnections removes clients with broken connections
func (h *Hub) CleanupStaleConnections() {
	var staleClients []*Client

	// Check all clients for broken connections
	for client := range h.clients {
		// Try to ping the client to check if connection is alive
		if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			log.Printf("[CLEANUP] Found stale connection for user '%s' (IP: %s): %v", client.username, client.ipAddr, err)
			staleClients = append(staleClients, client)
		}
	}

	// Remove stale clients
	for _, client := range staleClients {
		log.Printf("[CLEANUP] Removing stale connection for user '%s' (IP: %s)", client.username, client.ipAddr)
		delete(h.clients, client)
		close(client.send)
		client.conn.Close()
	}

	if len(staleClients) > 0 {
		log.Printf("[CLEANUP] Removed %d stale connections", len(staleClients))
		h.broadcastUserList()
	}
}

// ForceDisconnectUser forcibly removes a user from the clients map (admin command for stale connections)
func (h *Hub) ForceDisconnectUser(username string, adminUsername string) bool {
	for client := range h.clients {
		if strings.EqualFold(client.username, username) {
			log.Printf("[ADMIN] Force disconnecting user '%s' (IP: %s) by admin '%s'", username, client.ipAddr, adminUsername)

			// Try to close gracefully first
			client.conn.Close()

			// Remove from clients map
			delete(h.clients, client)
			close(client.send)

			h.broadcastUserList()
			return true
		}
	}
	log.Printf("[ADMIN] Force disconnect attempt for '%s' by '%s' - user not found", username, adminUsername)
	return false
}

func (h *Hub) Run() {
	// Start ban cleanup goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Hour) // Clean up every hour
		defer ticker.Stop()
		for range ticker.C {
			h.CleanupExpiredBans()
		}
	}()

	// Start stale connection cleanup goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // Check for stale connections every 5 minutes
		defer ticker.Stop()
		for range ticker.C {
			h.CleanupStaleConnections()
		}
	}()

	// Start plugin message handler goroutine
	go func() {
		for msg := range h.pluginManager.GetMessageChannel() {
			// Convert plugin message to shared message
			sharedMsg := shared.Message{
				Sender:    msg.Sender,
				Content:   msg.Content,
				CreatedAt: msg.CreatedAt,
				Type:      shared.TextMessage,
			}

			// Broadcast plugin message to all clients
			for client := range h.clients {
				select {
				case client.send <- sharedMsg:
				default:
					log.Printf("Dropping client %s due to full send channel\n", client.username)
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}()

	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Printf("Client %s registered (IP: %s)", client.username, client.ipAddr)
			h.broadcastUserList() // Broadcast after register
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Printf("Client %s unregistered (IP: %s)", client.username, client.ipAddr)
			}
			h.broadcastUserList()
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					log.Printf("Dropping client %s due to full send channel\n", client.username)
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// getDB returns the database reference
func (h *Hub) getDB() *sql.DB {
	return h.db
}
