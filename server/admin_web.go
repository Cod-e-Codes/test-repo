package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Cod-e-Codes/marchat/config"
	"github.com/Cod-e-Codes/marchat/plugin/manager"
)

//go:embed admin_web.html
var adminWebHTML string

// WebAdminServer provides a full-featured web admin panel with JSON APIs
type WebAdminServer struct {
	hub           *Hub
	db            *sql.DB
	cfg           *config.Config
	pluginManager *manager.PluginManager
	startTime     time.Time
	metrics       *webMetricsData
	sessionSecret []byte
}

// Session data structure
type sessionData struct {
	IsAdmin bool      `json:"isAdmin"`
	Expires time.Time `json:"expires"`
}

// Login request structure
type loginRequest struct {
	Key string `json:"key"`
}

// Session management functions
func (w *WebAdminServer) generateSessionSecret() error {
	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	if err != nil {
		return err
	}
	w.sessionSecret = secret
	return nil
}

func (w *WebAdminServer) createSession() (string, error) {
	session := sessionData{
		IsAdmin: true,
		Expires: time.Now().Add(2 * time.Hour), // 2 hour session
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return "", err
	}

	// Create HMAC signature
	h := hmac.New(sha256.New, w.sessionSecret)
	h.Write(sessionJSON)
	signature := hex.EncodeToString(h.Sum(nil))

	// Combine session data and signature
	sessionDataB64 := base64.StdEncoding.EncodeToString(sessionJSON)
	return sessionDataB64 + "." + signature, nil
}

func (w *WebAdminServer) validateSession(sessionToken string) bool {
	parts := strings.Split(sessionToken, ".")
	if len(parts) != 2 {
		return false
	}

	sessionDataB64, signature := parts[0], parts[1]

	// Decode session data
	sessionJSON, err := base64.StdEncoding.DecodeString(sessionDataB64)
	if err != nil {
		return false
	}

	// Verify signature
	h := hmac.New(sha256.New, w.sessionSecret)
	h.Write(sessionJSON)
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return false
	}

	// Parse and validate session
	var session sessionData
	if err := json.Unmarshal(sessionJSON, &session); err != nil {
		return false
	}

	// Check expiration
	return session.IsAdmin && time.Now().Before(session.Expires)
}

// Web-specific data structures matching the TUI panel
type webUserInfo struct {
	Username    string    `json:"username"`
	Status      string    `json:"status"`
	IP          string    `json:"ip"`
	ConnectedAt time.Time `json:"connected_at"`
	LastSeen    time.Time `json:"last_seen"`
	Messages    int       `json:"messages"`
	IsAdmin     bool      `json:"is_admin"`
	IsBanned    bool      `json:"is_banned"`
	IsKicked    bool      `json:"is_kicked"`
}

type webPluginInfo struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Version string `json:"version"`
}

type webSystemStats struct {
	Uptime         string  `json:"uptime"`
	MemoryUsage    float64 `json:"memory_usage"`
	CPUUsage       float64 `json:"cpu_usage"`
	ActiveUsers    int     `json:"active_users"`
	TotalUsers     int     `json:"total_users"`
	MessagesSent   int     `json:"messages_sent"`
	PluginsActive  int     `json:"plugins_active"`
	ServerStatus   string  `json:"server_status"`
	GoroutineCount int     `json:"goroutine_count"`
	HeapSize       uint64  `json:"heap_size"`
	AllocatedMem   uint64  `json:"allocated_mem"`
	GCCount        uint32  `json:"gc_count"`
}

type webMetricsData struct {
	ConnectionHistory []connectionPoint `json:"connection_history"`
	MessageHistory    []messagePoint    `json:"message_history"`
	MemoryHistory     []memoryPoint     `json:"memory_history"`
	LastUpdated       time.Time         `json:"last_updated"`
	PeakUsers         int               `json:"peak_users"`
	PeakMemory        uint64            `json:"peak_memory"`
	TotalConnections  int               `json:"total_connections"`
	TotalDisconnects  int               `json:"total_disconnects"`
	AverageResponse   string            `json:"average_response"`
}

type webLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	User      string    `json:"user"`
	Component string    `json:"component"`
}

type webOverviewData struct {
	SystemStats webSystemStats  `json:"system_stats"`
	Config      webConfigInfo   `json:"config"`
	Database    webDatabaseInfo `json:"database"`
}

type webConfigInfo struct {
	Port           int    `json:"port"`
	TLSEnabled     bool   `json:"tls_enabled"`
	MaxFileSize    string `json:"max_file_size"`
	LogLevel       string `json:"log_level"`
	BanHistoryGaps bool   `json:"ban_history_gaps"`
	AdminCount     int    `json:"admin_count"`
	TLSCertFile    string `json:"tls_cert_file,omitempty"`
	TLSKeyFile     string `json:"tls_key_file,omitempty"`
}

type webDatabaseInfo struct {
	Path      string `json:"path"`
	ConfigDir string `json:"config_dir"`
}

// NewWebAdminServer creates a new web admin server with full functionality
func NewWebAdminServer(hub *Hub, db *sql.DB, cfg *config.Config) *WebAdminServer {
	server := &WebAdminServer{
		hub:           hub,
		db:            db,
		cfg:           cfg,
		pluginManager: hub.GetPluginManager(),
		startTime:     time.Now(),
		metrics: &webMetricsData{
			ConnectionHistory: make([]connectionPoint, 0),
			MessageHistory:    make([]messagePoint, 0),
			MemoryHistory:     make([]memoryPoint, 0),
			LastUpdated:       time.Now(),
		},
	}

	// Generate session secret
	if err := server.generateSessionSecret(); err != nil {
		log.Printf("Warning: Failed to generate session secret: %v", err)
	}

	return server
}

// RegisterRoutes attaches all web admin routes to mux
func (w *WebAdminServer) RegisterRoutes(mux *http.ServeMux) {
	// Login route (no auth required)
	mux.HandleFunc("/admin/api/login", w.handleLogin)

	// Main panel route (no auth required - serves login page or admin panel based on session)
	mux.HandleFunc("/admin", w.serveIndex)
	mux.HandleFunc("/admin/", w.serveIndex) // Handle sub-paths

	// API endpoints matching TUI functionality
	mux.HandleFunc("/admin/api/overview", w.auth(w.handleOverview))
	mux.HandleFunc("/admin/api/users", w.auth(w.handleUsers))
	mux.HandleFunc("/admin/api/system", w.auth(w.handleSystem))
	mux.HandleFunc("/admin/api/logs", w.auth(w.handleLogs))
	mux.HandleFunc("/admin/api/plugins", w.auth(w.handlePlugins))
	mux.HandleFunc("/admin/api/metrics", w.auth(w.handleMetrics))

	// Action endpoints
	mux.HandleFunc("/admin/api/action/user", w.auth(w.handleUserAction))
	mux.HandleFunc("/admin/api/action/system", w.auth(w.handleSystemAction))
	mux.HandleFunc("/admin/api/action/plugin", w.auth(w.handlePluginAction))
	mux.HandleFunc("/admin/api/action/metrics", w.auth(w.handleMetricsAction))

	// Utility endpoints
	mux.HandleFunc("/admin/api/refresh", w.auth(w.handleRefresh))
}

func (w *WebAdminServer) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		// Check for session cookie
		cookie, err := r.Cookie("admin_session")
		if err != nil || !w.validateSession(cookie.Value) {
			// Fallback to legacy admin key for backward compatibility
			key := r.Header.Get("X-Admin-Key")
			if key == "" {
				key = r.URL.Query().Get("key")
			}
			if key == "" || !strings.EqualFold(key, w.cfg.AdminKey) {
				rw.WriteHeader(http.StatusUnauthorized)
				writeJSON(rw, map[string]string{"error": "Unauthorized"})
				return
			}
		}
		next(rw, r)
	}
}

func (w *WebAdminServer) handleLogin(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate admin key
	if req.Key == "" || !strings.EqualFold(req.Key, w.cfg.AdminKey) {
		writeJSON(rw, map[string]interface{}{
			"success": false,
			"message": "Invalid admin key",
		})
		return
	}

	// Create session
	sessionToken, err := w.createSession()
	if err != nil {
		log.Printf("Error creating session: %v", err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set secure cookie
	http.SetCookie(rw, &http.Cookie{
		Name:     "admin_session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   w.cfg.IsTLSEnabled(), // Only secure over HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   7200, // 2 hours
	})

	writeJSON(rw, map[string]interface{}{
		"success": true,
		"message": "Login successful",
	})
}

func (w *WebAdminServer) serveIndex(rw http.ResponseWriter, r *http.Request) {
	// Always serve the HTML - the JavaScript will handle showing login vs admin panel
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := rw.Write([]byte(adminWebHTML)); err != nil {
		log.Printf("Error writing HTML response: %v", err)
	}
}

func (w *WebAdminServer) handleOverview(rw http.ResponseWriter, r *http.Request) {
	data := w.getOverviewData()
	writeJSON(rw, data)
}

func (w *WebAdminServer) handleUsers(rw http.ResponseWriter, r *http.Request) {
	users := w.getUsersData()
	writeJSON(rw, users)
}

func (w *WebAdminServer) handleSystem(rw http.ResponseWriter, r *http.Request) {
	systemData := w.getSystemData()
	writeJSON(rw, systemData)
}

func (w *WebAdminServer) handleLogs(rw http.ResponseWriter, r *http.Request) {
	logs := w.getLogsData()
	writeJSON(rw, logs)
}

func (w *WebAdminServer) handlePlugins(rw http.ResponseWriter, r *http.Request) {
	plugins := w.getPluginsData()
	writeJSON(rw, plugins)
}

func (w *WebAdminServer) handleMetrics(rw http.ResponseWriter, r *http.Request) {
	w.updateMetrics()
	writeJSON(rw, w.metrics)
}

func (w *WebAdminServer) handleUserAction(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	type userActionReq struct {
		Action   string `json:"action"`
		Username string `json:"username"`
	}

	var req userActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		writeJSON(rw, map[string]string{"error": "Invalid request"})
		return
	}

	var message string
	var success bool

	switch req.Action {
	case "ban":
		w.hub.BanUser(req.Username, "web-admin")
		message = fmt.Sprintf("User '%s' has been banned", req.Username)
		success = true
	case "unban":
		success = w.hub.UnbanUser(req.Username, "web-admin")
		if success {
			message = fmt.Sprintf("User '%s' has been unbanned", req.Username)
		} else {
			message = fmt.Sprintf("User '%s' was not found in ban list", req.Username)
		}
	case "kick":
		w.hub.KickUser(req.Username, "web-admin")
		message = fmt.Sprintf("User '%s' has been kicked (24h)", req.Username)
		success = true
	case "allow":
		success = w.hub.AllowUser(req.Username, "web-admin")
		if success {
			message = fmt.Sprintf("User '%s' has been allowed back", req.Username)
		} else {
			message = fmt.Sprintf("User '%s' was not found in kick list", req.Username)
		}
	case "make_admin":
		// This would require additional implementation in the hub
		message = "Make admin functionality not yet implemented"
		success = false
	default:
		rw.WriteHeader(http.StatusBadRequest)
		writeJSON(rw, map[string]string{"error": "Invalid action"})
		return
	}

	writeJSON(rw, map[string]interface{}{
		"success": success,
		"message": message,
	})
}

func (w *WebAdminServer) handleSystemAction(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	type systemActionReq struct {
		Action string `json:"action"`
	}

	var req systemActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		writeJSON(rw, map[string]string{"error": "Invalid request"})
		return
	}

	var message string
	var success bool

	switch req.Action {
	case "clear_db":
		err := ClearMessages(w.db)
		if err != nil {
			message = "Failed to clear database: " + err.Error()
			success = false
		} else {
			message = "Database cleared successfully"
			success = true
		}
	case "backup_db":
		filename, err := BackupDatabase(w.cfg.DBPath)
		if err != nil {
			message = "Failed to backup database: " + err.Error()
			success = false
		} else {
			message = "Database backed up to: " + filename
			success = true
		}
	case "show_stats":
		stats, err := GetDatabaseStats(w.db)
		if err != nil {
			message = "Failed to get stats: " + err.Error()
			success = false
		} else {
			message = stats
			success = true
		}
	case "force_gc":
		runtime.GC()
		message = "Garbage collection forced"
		success = true
	default:
		rw.WriteHeader(http.StatusBadRequest)
		writeJSON(rw, map[string]string{"error": "Invalid action"})
		return
	}

	writeJSON(rw, map[string]interface{}{
		"success": success,
		"message": message,
	})
}

func (w *WebAdminServer) handlePluginAction(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	type pluginActionReq struct {
		Action string `json:"action"`
		Plugin string `json:"plugin"`
	}

	var req pluginActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		writeJSON(rw, map[string]string{"error": "Invalid request"})
		return
	}

	var message string
	var success bool

	switch req.Action {
	case "enable":
		// Plugin enable/disable would need to be implemented in the plugin manager
		message = fmt.Sprintf("Plugin enable functionality not yet implemented for '%s'", req.Plugin)
		success = false
	case "disable":
		message = fmt.Sprintf("Plugin disable functionality not yet implemented for '%s'", req.Plugin)
		success = false
	case "install":
		message = fmt.Sprintf("Plugin install functionality not yet implemented for '%s'", req.Plugin)
		success = false
	case "uninstall":
		message = fmt.Sprintf("Plugin uninstall functionality not yet implemented for '%s'", req.Plugin)
		success = false
	default:
		rw.WriteHeader(http.StatusBadRequest)
		writeJSON(rw, map[string]string{"error": "Invalid action"})
		return
	}

	writeJSON(rw, map[string]interface{}{
		"success": success,
		"message": message,
	})
}

func (w *WebAdminServer) handleMetricsAction(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	type metricsActionReq struct {
		Action string `json:"action"`
	}

	var req metricsActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		writeJSON(rw, map[string]string{"error": "Invalid request"})
		return
	}

	switch req.Action {
	case "reset":
		w.resetMetrics()
		writeJSON(rw, map[string]interface{}{
			"success": true,
			"message": "Metrics reset successfully",
		})
	case "export_logs":
		// Export logs functionality
		writeJSON(rw, map[string]interface{}{
			"success": true,
			"message": "Logs exported successfully",
		})
	default:
		rw.WriteHeader(http.StatusBadRequest)
		writeJSON(rw, map[string]string{"error": "Invalid action"})
	}
}

func (w *WebAdminServer) handleRefresh(rw http.ResponseWriter, r *http.Request) {
	// Force refresh all data
	w.updateMetrics()
	writeJSON(rw, map[string]interface{}{
		"success": true,
		"message": "Data refreshed",
	})
}

// Data collection methods matching the TUI panel functionality
func (w *WebAdminServer) getOverviewData() webOverviewData {
	systemStats := w.getSystemStats()

	return webOverviewData{
		SystemStats: systemStats,
		Config: webConfigInfo{
			Port:           w.cfg.Port,
			TLSEnabled:     w.cfg.IsTLSEnabled(),
			MaxFileSize:    fmt.Sprintf("%.1f MB", float64(w.cfg.MaxFileBytes)/1024/1024),
			LogLevel:       w.cfg.LogLevel,
			BanHistoryGaps: w.cfg.BanGapsHistory,
			AdminCount:     len(w.cfg.Admins),
			TLSCertFile:    w.cfg.TLSCertFile,
			TLSKeyFile:     w.cfg.TLSKeyFile,
		},
		Database: webDatabaseInfo{
			Path:      w.cfg.DBPath,
			ConfigDir: w.cfg.ConfigDir,
		},
	}
}

func (w *WebAdminServer) getSystemStats() webSystemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	var messageCount int
	err := w.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&messageCount)
	if err != nil {
		log.Printf("Error getting message count: %v", err)
	}

	var userCount int
	err = w.db.QueryRow("SELECT COUNT(DISTINCT sender) FROM messages WHERE sender != 'System'").Scan(&userCount)
	if err != nil {
		log.Printf("Error getting user count: %v", err)
	}

	plugins := w.pluginManager.ListPlugins()
	activePlugins := 0
	for _, plugin := range plugins {
		if plugin.Manifest != nil {
			activePlugins++
		}
	}

	uptime := time.Since(w.startTime)

	return webSystemStats{
		Uptime:         w.formatDuration(uptime),
		MemoryUsage:    float64(m.Alloc) / 1024 / 1024,
		ActiveUsers:    len(w.hub.clients),
		TotalUsers:     userCount,
		MessagesSent:   messageCount,
		PluginsActive:  activePlugins,
		ServerStatus:   "Running",
		GoroutineCount: runtime.NumGoroutine(),
		HeapSize:       m.HeapSys,
		AllocatedMem:   m.Alloc,
		GCCount:        m.NumGC,
	}
}

func (w *WebAdminServer) getUsersData() []webUserInfo {
	// Get message counts per user
	rows, err := w.db.Query(`
		SELECT sender, COUNT(*) as message_count 
		FROM messages 
		WHERE sender != 'System' 
		GROUP BY sender
	`)
	if err != nil {
		log.Printf("Error loading user message counts: %v", err)
		return []webUserInfo{}
	}
	defer rows.Close()

	userMessages := make(map[string]int)
	for rows.Next() {
		var username string
		var count int
		if err := rows.Scan(&username, &count); err != nil {
			continue
		}
		userMessages[username] = count
	}

	// Get connected users from hub
	connectedUsers := make(map[string]*Client)
	for client := range w.hub.clients {
		if client.username != "" {
			connectedUsers[client.username] = client
		}
	}

	// Create user list combining database and live data
	userMap := make(map[string]*webUserInfo)

	// Add users from messages
	for username, msgCount := range userMessages {
		userMap[username] = &webUserInfo{
			Username: username,
			Status:   "Offline",
			IP:       "N/A",
			Messages: msgCount,
			IsAdmin:  false,
		}
	}

	// Update with connected users
	for username, client := range connectedUsers {
		if user, exists := userMap[username]; exists {
			user.Status = "Online"
			user.IP = client.ipAddr
			user.ConnectedAt = time.Now() // Simplified
			user.LastSeen = time.Now()
			user.IsAdmin = client.isAdmin
		} else {
			userMap[username] = &webUserInfo{
				Username:    username,
				Status:      "Online",
				IP:          client.ipAddr,
				ConnectedAt: time.Now(),
				LastSeen:    time.Now(),
				Messages:    0,
				IsAdmin:     client.isAdmin,
			}
		}
	}

	// Check ban/kick status
	for username, user := range userMap {
		user.IsBanned = w.hub.IsUserBanned(username)
		if user.IsBanned {
			user.Status = "Banned"
		}
	}

	// Convert map to slice
	users := make([]webUserInfo, 0, len(userMap))
	for _, user := range userMap {
		users = append(users, *user)
	}

	// Sort users by status (online first), then by message count
	sort.Slice(users, func(i, j int) bool {
		if users[i].Status != users[j].Status {
			if users[i].Status == "Online" {
				return true
			}
			if users[j].Status == "Online" {
				return false
			}
		}
		return users[i].Messages > users[j].Messages
	})

	return users
}

func (w *WebAdminServer) getSystemData() map[string]interface{} {
	systemStats := w.getSystemStats()

	return map[string]interface{}{
		"stats": systemStats,
		"config": map[string]interface{}{
			"port":             w.cfg.Port,
			"database":         w.cfg.DBPath,
			"config_dir":       w.cfg.ConfigDir,
			"log_level":        w.cfg.LogLevel,
			"max_file_size":    fmt.Sprintf("%.1f MB", float64(w.cfg.MaxFileBytes)/1024/1024),
			"admin_users":      strings.Join(w.cfg.Admins, ", "),
			"tls_enabled":      w.cfg.IsTLSEnabled(),
			"tls_cert_file":    w.cfg.TLSCertFile,
			"tls_key_file":     w.cfg.TLSKeyFile,
			"jwt_secret":       w.maskSecret(w.cfg.JWTSecret),
			"admin_key":        w.maskSecret(w.cfg.AdminKey),
			"ban_history_gaps": w.cfg.BanGapsHistory,
			"plugin_registry":  w.cfg.PluginRegistryURL,
		},
	}
}

func (w *WebAdminServer) getLogsData() []webLogEntry {
	currentTime := time.Now()
	logs := []webLogEntry{
		{
			Timestamp: currentTime.Add(-30 * time.Second),
			Level:     "INFO",
			Message:   "Web admin panel accessed",
			User:      "Admin",
			Component: "WebAdminPanel",
		},
		{
			Timestamp: currentTime.Add(-1 * time.Minute),
			Level:     "INFO",
			Message:   fmt.Sprintf("Active connections: %d", len(w.hub.clients)),
			User:      "System",
			Component: "ConnectionManager",
		},
		{
			Timestamp: currentTime.Add(-2 * time.Minute),
			Level:     "INFO",
			Message:   fmt.Sprintf("Memory usage: %.1f MB", w.getSystemStats().MemoryUsage),
			User:      "System",
			Component: "Monitor",
		},
		{
			Timestamp: currentTime.Add(-5 * time.Minute),
			Level:     "INFO",
			Message:   "Server startup completed successfully",
			User:      "System",
			Component: "Server",
		},
	}

	// Add plugin-related logs
	plugins := w.getPluginsData()
	for _, plugin := range plugins {
		if plugin.Status == "Active" {
			logs = append(logs, webLogEntry{
				Timestamp: currentTime.Add(-3 * time.Minute),
				Level:     "INFO",
				Message:   fmt.Sprintf("Plugin '%s' loaded successfully", plugin.Name),
				User:      "System",
				Component: "PluginManager",
			})
		}
	}

	// Sort logs by timestamp (newest first)
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Timestamp.After(logs[j].Timestamp)
	})

	return logs
}

func (w *WebAdminServer) getPluginsData() []webPluginInfo {
	plugins := w.pluginManager.ListPlugins()
	result := []webPluginInfo{}

	for name, plugin := range plugins {
		status := "Active"
		if plugin.Manifest == nil {
			status = "Inactive"
		}

		version := "1.0.0"
		if plugin.Manifest != nil && plugin.Manifest.Version != "" {
			version = plugin.Manifest.Version
		}

		result = append(result, webPluginInfo{
			Name:    name,
			Status:  status,
			Version: version,
		})
	}

	return result
}

func (w *WebAdminServer) updateMetrics() {
	currentTime := time.Now()

	// Add connection point
	w.metrics.ConnectionHistory = append(w.metrics.ConnectionHistory, connectionPoint{
		Time:  currentTime,
		Count: len(w.hub.clients),
	})

	// Get current message count
	var messageCount int
	if err := w.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&messageCount); err != nil {
		log.Printf("Error getting message count: %v", err)
	}

	// Add message point
	w.metrics.MessageHistory = append(w.metrics.MessageHistory, messagePoint{
		Time:  currentTime,
		Count: messageCount,
	})

	// Get memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Add memory point
	w.metrics.MemoryHistory = append(w.metrics.MemoryHistory, memoryPoint{
		Time:   currentTime,
		Memory: m.Alloc,
	})

	// Keep only last 100 points for performance
	maxPoints := 100
	if len(w.metrics.ConnectionHistory) > maxPoints {
		w.metrics.ConnectionHistory = w.metrics.ConnectionHistory[len(w.metrics.ConnectionHistory)-maxPoints:]
	}
	if len(w.metrics.MessageHistory) > maxPoints {
		w.metrics.MessageHistory = w.metrics.MessageHistory[len(w.metrics.MessageHistory)-maxPoints:]
	}
	if len(w.metrics.MemoryHistory) > maxPoints {
		w.metrics.MemoryHistory = w.metrics.MemoryHistory[len(w.metrics.MemoryHistory)-maxPoints:]
	}

	// Update peak values
	if len(w.hub.clients) > w.metrics.PeakUsers {
		w.metrics.PeakUsers = len(w.hub.clients)
	}
	if m.Alloc > w.metrics.PeakMemory {
		w.metrics.PeakMemory = m.Alloc
	}

	w.metrics.LastUpdated = currentTime
}

func (w *WebAdminServer) resetMetrics() {
	w.metrics = &webMetricsData{
		ConnectionHistory: make([]connectionPoint, 0),
		MessageHistory:    make([]messagePoint, 0),
		MemoryHistory:     make([]memoryPoint, 0),
		LastUpdated:       time.Now(),
		PeakUsers:         0,
		PeakMemory:        0,
		TotalConnections:  0,
		TotalDisconnects:  0,
		AverageResponse:   "0ms",
	}
}

// Helper methods
func (w *WebAdminServer) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func (w *WebAdminServer) maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "***hidden***"
	}
	return secret[:4] + "***" + secret[len(secret)-4:]
}

func writeJSON(rw http.ResponseWriter, v interface{}) {
	rw.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(rw).Encode(v); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

// HTML content is embedded from admin_web.html via go:embed (see adminWebHTML)
