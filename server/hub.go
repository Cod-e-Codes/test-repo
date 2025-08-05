package server

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Cod-e-Codes/marchat/plugin/manager"
	"github.com/Cod-e-Codes/marchat/shared"
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
}

func NewHub(pluginDir, dataDir, registryURL string) *Hub {
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
	}
}

// BanUser adds a user to the ban list
func (h *Hub) BanUser(username string, adminUsername string) {
	h.banMutex.Lock()
	defer h.banMutex.Unlock()

	// Ban for 24 hours by default
	h.bans[strings.ToLower(username)] = time.Now().Add(24 * time.Hour)
	log.Printf("[ADMIN] User '%s' banned by '%s' until %s", username, adminUsername, time.Now().Add(24*time.Hour).Format("2006-01-02 15:04:05"))

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

func (h *Hub) Run() {
	// Start ban cleanup goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Hour) // Clean up every hour
		defer ticker.Stop()
		for range ticker.C {
			h.CleanupExpiredBans()
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
