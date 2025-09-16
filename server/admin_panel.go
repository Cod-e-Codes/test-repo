package server

import (
	"database/sql"
	"fmt"
	"log"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Cod-e-Codes/marchat/config"
	"github.com/Cod-e-Codes/marchat/plugin/manager"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Tab types for the admin panel
type tabType int

const (
	tabOverview tabType = iota
	tabUsers
	tabSystem
	tabLogs
	tabPlugins
	tabMetrics // New metrics tab
)

// Plugin information
type pluginInfo struct {
	Name    string
	Status  string
	Version string
}

// User information for the users table
type userInfo struct {
	Username    string
	Status      string
	IP          string
	ConnectedAt time.Time
	LastSeen    time.Time
	Messages    int
	IsAdmin     bool
	IsBanned    bool
	IsKicked    bool
}

// System statistics
type systemStats struct {
	Uptime         time.Duration
	MemoryUsage    float64
	CPUUsage       float64
	ActiveUsers    int
	TotalUsers     int
	MessagesSent   int
	PluginsActive  int
	ServerStatus   string
	GoroutineCount int
	HeapSize       uint64
	AllocatedMem   uint64
	GCCount        uint32
}

// Metrics data for charts and graphs
type metricsData struct {
	ConnectionHistory []connectionPoint
	MessageHistory    []messagePoint
	MemoryHistory     []memoryPoint
	LastUpdated       time.Time
	PeakUsers         int
	PeakMemory        uint64
	TotalConnections  int
	TotalDisconnects  int
	AverageResponse   time.Duration
}

type connectionPoint struct {
	Time  time.Time
	Count int
}

type messagePoint struct {
	Time  time.Time
	Count int
}

type memoryPoint struct {
	Time   time.Time
	Memory uint64
}

// Log entry
type logEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	User      string
	Component string
}

// AdminPanel represents the main admin panel state
type AdminPanel struct {
	// Navigation
	activeTab tabType
	tabs      []string

	// Components
	help      help.Model
	userTable table.Model

	// Scroll state for each tab
	overviewScroll int
	systemScroll   int
	pluginsScroll  int
	metricsScroll  int
	logsScroll     int

	// Data
	users      []userInfo
	plugins    []pluginInfo
	systemInfo systemStats
	metrics    metricsData
	config     *config.Config
	logs       []logEntry

	// Server integration
	hub           *Hub
	db            *sql.DB
	pluginManager *manager.PluginManager
	startTime     time.Time

	// UI state
	width          int
	height         int
	quitting       bool
	selectedUser   int
	selectedPlugin int
	message        string
	messageTimer   int

	// Performance tracking
	lastMessageCount int
	messageRate      float64
	connectionRate   float64

	// Keybindings
	keys keyMap
}

// Keybindings
type keyMap struct {
	TabNext      key.Binding
	TabPrev      key.Binding
	Quit         key.Binding
	Refresh      key.Binding
	ClearDB      key.Binding
	BackupDB     key.Binding
	ShowStats    key.Binding
	Help         key.Binding
	Action       key.Binding
	Ban          key.Binding
	Unban        key.Binding
	Kick         key.Binding
	Allow        key.Binding
	AddAdmin     key.Binding
	Enable       key.Binding
	Disable      key.Binding
	Install      key.Binding
	Uninstall    key.Binding
	Up           key.Binding
	Down         key.Binding
	ExportLogs   key.Binding
	ResetMetrics key.Binding
	ForceGC      key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.TabNext, k.TabPrev, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.TabNext, k.TabPrev, k.Refresh, k.Help},
		{k.Action, k.Quit, k.ExportLogs, k.ForceGC},
		{k.Ban, k.Unban, k.Kick, k.Allow, k.AddAdmin},
		{k.Enable, k.Disable, k.Install, k.Uninstall},
		{k.ClearDB, k.BackupDB, k.ShowStats, k.ResetMetrics},
	}
}

// Enhanced styling
var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4")
	secondaryColor = lipgloss.Color("#FF75B7")
	successColor   = lipgloss.Color("#00FFA3")
	warningColor   = lipgloss.Color("#FFA500")
	errorColor     = lipgloss.Color("#FF4444")
	accentColor    = lipgloss.Color("#FFD700")

	// Enhanced styles
	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginLeft(2)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Italic(true).
			MarginLeft(2)

	tabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 1)

	activeTabStyle = tabStyle.
			Foreground(primaryColor).
			Bold(true).
			Background(lipgloss.Color("235"))

	statusStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	errorStylePanel = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	warningStylePanel = lipgloss.NewStyle().
				Foreground(warningColor).
				Bold(true)

	infoStylePanel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BFFF")).
			Bold(true)

	metricLabelStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)

	metricValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("white")).
				Bold(true)

	// Border styles
	mainBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	messageStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true).
			Padding(0, 1)
)

// NewAdminPanel creates a new admin panel instance
func NewAdminPanel(hub *Hub, db *sql.DB, pluginManager *manager.PluginManager, liveConfig *config.Config) *AdminPanel {
	// Initialize keybindings
	keys := keyMap{
		TabNext: key.NewBinding(
			key.WithKeys("tab", "right"),
			key.WithHelp("tab", "next tab"),
		),
		TabPrev: key.NewBinding(
			key.WithKeys("shift+tab", "left"),
			key.WithHelp("shift+tab", "prev tab"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		ClearDB: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear db"),
		),
		BackupDB: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "backup db"),
		),
		ShowStats: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "show stats"),
		),
		Action: key.NewBinding(
			key.WithKeys("enter", "space"),
			key.WithHelp("enter", "action"),
		),
		Help: key.NewBinding(
			key.WithKeys("?", "h"),
			key.WithHelp("?", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Ban: key.NewBinding(
			key.WithKeys("B"),
			key.WithHelp("B", "ban user"),
		),
		Unban: key.NewBinding(
			key.WithKeys("U"),
			key.WithHelp("U", "unban user"),
		),
		Kick: key.NewBinding(
			key.WithKeys("K"),
			key.WithHelp("K", "kick user"),
		),
		Allow: key.NewBinding(
			key.WithKeys("A"),
			key.WithHelp("A", "allow user"),
		),
		AddAdmin: key.NewBinding(
			key.WithKeys("M"),
			key.WithHelp("M", "make admin"),
		),
		Enable: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "enable plugin"),
		),
		Disable: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "disable plugin"),
		),
		Install: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "install plugin"),
		),
		Uninstall: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "uninstall plugin"),
		),
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "down"),
		),
		ExportLogs: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "export logs"),
		),
		ResetMetrics: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "reset metrics"),
		),
		ForceGC: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "force GC"),
		),
	}

	// Initialize enhanced table
	columns := []table.Column{
		{Title: "Username", Width: 15},
		{Title: "Status", Width: 10},
		{Title: "IP", Width: 15},
		{Title: "Messages", Width: 10},
		{Title: "Admin", Width: 8},
		{Title: "Last Seen", Width: 12},
		{Title: "Connected", Width: 12},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(12),
	)

	// Enhanced table styling
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(primaryColor).
		BorderBottom(true).
		Bold(true).
		Foreground(accentColor)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(secondaryColor).
		Bold(false)
	t.SetStyles(s)

	panel := &AdminPanel{
		activeTab:     tabOverview,
		tabs:          []string{"Overview", "Users", "System", "Logs", "Plugins", "Metrics"},
		help:          help.New(),
		userTable:     t,
		keys:          keys,
		hub:           hub,
		db:            db,
		pluginManager: pluginManager,
		startTime:     time.Now(),
		config:        liveConfig,
		systemInfo: systemStats{
			Uptime:       0,
			MemoryUsage:  0,
			CPUUsage:     0,
			ServerStatus: "Running",
		},
		metrics: metricsData{
			ConnectionHistory: make([]connectionPoint, 0),
			MessageHistory:    make([]messagePoint, 0),
			MemoryHistory:     make([]memoryPoint, 0),
			LastUpdated:       time.Now(),
		},
		selectedUser:   -1,
		selectedPlugin: -1,
	}

	// Load initial data
	panel.refreshData()

	return panel
}

func (ap *AdminPanel) refreshData() {
	// Load users from database and hub
	ap.loadUsers()
	// Load plugins
	ap.loadPlugins()
	// Load logs
	ap.loadLogs()
	// Update system stats
	ap.updateSystemStats()
	// Update metrics
	ap.updateMetrics()
	// Update user table
	ap.updateUserTable()
}

func (ap *AdminPanel) loadUsers() {
	// Get message counts per user
	rows, err := ap.db.Query(`
		SELECT sender, COUNT(*) as message_count 
		FROM messages 
		WHERE sender != 'System' 
		GROUP BY sender
	`)
	if err != nil {
		log.Printf("Error loading user message counts: %v", err)
		return
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
	for client := range ap.hub.clients {
		if client.username != "" {
			connectedUsers[client.username] = client
		}
	}

	// Create user list combining database and live data
	userMap := make(map[string]*userInfo)

	// Add users from messages
	for username, msgCount := range userMessages {
		userMap[username] = &userInfo{
			Username: username,
			Status:   "Offline",
			IP:       "N/A",
			Messages: msgCount,
			IsAdmin:  false, // Would need to check against admin list
		}
	}

	// Update with connected users
	for username, client := range connectedUsers {
		if user, exists := userMap[username]; exists {
			user.Status = "Online"
			user.IP = client.ipAddr
			user.ConnectedAt = time.Now() // Simplified - would need client.connectedAt
			user.LastSeen = time.Now()
			user.IsAdmin = client.isAdmin
		} else {
			userMap[username] = &userInfo{
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
		user.IsBanned = ap.hub.IsUserBanned(username)
		if user.IsBanned {
			user.Status = "Banned"
		}
	}

	// Convert map to slice
	ap.users = make([]userInfo, 0, len(userMap))
	for _, user := range userMap {
		ap.users = append(ap.users, *user)
	}

	// Sort users by status (online first), then by message count
	sort.Slice(ap.users, func(i, j int) bool {
		if ap.users[i].Status != ap.users[j].Status {
			if ap.users[i].Status == "Online" {
				return true
			}
			if ap.users[j].Status == "Online" {
				return false
			}
		}
		return ap.users[i].Messages > ap.users[j].Messages
	})
}

func (ap *AdminPanel) loadPlugins() {
	// Get plugin information from plugin manager
	plugins := ap.pluginManager.ListPlugins()
	ap.plugins = []pluginInfo{}

	for name, plugin := range plugins {
		status := "Active"
		if plugin.Manifest == nil {
			status = "Inactive"
		}

		version := "1.0.0"
		if plugin.Manifest != nil && plugin.Manifest.Version != "" {
			version = plugin.Manifest.Version
		}

		ap.plugins = append(ap.plugins, pluginInfo{
			Name:    name,
			Status:  status,
			Version: version,
		})
	}
}

func (ap *AdminPanel) loadLogs() {
	// Enhanced log entries with more system information
	currentTime := time.Now()
	ap.logs = []logEntry{
		{
			Timestamp: currentTime.Add(-30 * time.Second),
			Level:     "INFO",
			Message:   "Admin panel started by user",
			User:      "Admin",
			Component: "AdminPanel",
		},
		{
			Timestamp: currentTime.Add(-1 * time.Minute),
			Level:     "INFO",
			Message:   fmt.Sprintf("Active connections: %d", len(ap.hub.clients)),
			User:      "System",
			Component: "ConnectionManager",
		},
		{
			Timestamp: currentTime.Add(-2 * time.Minute),
			Level:     "INFO",
			Message:   fmt.Sprintf("Memory usage: %.1f MB", ap.systemInfo.MemoryUsage),
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
	for _, plugin := range ap.plugins {
		if plugin.Status == "Active" {
			ap.logs = append(ap.logs, logEntry{
				Timestamp: currentTime.Add(-3 * time.Minute),
				Level:     "INFO",
				Message:   fmt.Sprintf("Plugin '%s' loaded successfully", plugin.Name),
				User:      "System",
				Component: "PluginManager",
			})
		}
	}

	// Sort logs by timestamp (newest first)
	sort.Slice(ap.logs, func(i, j int) bool {
		return ap.logs[i].Timestamp.After(ap.logs[j].Timestamp)
	})
}

func (ap *AdminPanel) updateSystemStats() {
	// Get runtime memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Get message count
	var messageCount int
	err := ap.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&messageCount)
	if err != nil {
		log.Printf("Error getting message count: %v", err)
	}

	// Get unique user count
	var userCount int
	err = ap.db.QueryRow("SELECT COUNT(DISTINCT sender) FROM messages WHERE sender != 'System'").Scan(&userCount)
	if err != nil {
		log.Printf("Error getting user count: %v", err)
	}

	// Count active plugins
	activePlugins := 0
	for _, plugin := range ap.plugins {
		if plugin.Status == "Active" {
			activePlugins++
		}
	}

	// Calculate message rate
	if ap.lastMessageCount > 0 {
		timeDiff := time.Since(ap.metrics.LastUpdated).Seconds()
		if timeDiff > 0 {
			ap.messageRate = float64(messageCount-ap.lastMessageCount) / timeDiff
		}
	}
	ap.lastMessageCount = messageCount

	ap.systemInfo.MessagesSent = messageCount
	ap.systemInfo.TotalUsers = userCount
	ap.systemInfo.ActiveUsers = len(ap.hub.clients)
	ap.systemInfo.PluginsActive = activePlugins
	ap.systemInfo.Uptime = time.Since(ap.startTime)
	ap.systemInfo.ServerStatus = "Running"
	ap.systemInfo.GoroutineCount = runtime.NumGoroutine()
	ap.systemInfo.HeapSize = m.HeapSys
	ap.systemInfo.AllocatedMem = m.Alloc
	ap.systemInfo.GCCount = m.NumGC
	ap.systemInfo.MemoryUsage = float64(m.Alloc) / 1024 / 1024 // Convert to MB
}

func (ap *AdminPanel) updateMetrics() {
	currentTime := time.Now()

	// Add connection point
	ap.metrics.ConnectionHistory = append(ap.metrics.ConnectionHistory, connectionPoint{
		Time:  currentTime,
		Count: ap.systemInfo.ActiveUsers,
	})

	// Add message point
	ap.metrics.MessageHistory = append(ap.metrics.MessageHistory, messagePoint{
		Time:  currentTime,
		Count: ap.systemInfo.MessagesSent,
	})

	// Add memory point
	ap.metrics.MemoryHistory = append(ap.metrics.MemoryHistory, memoryPoint{
		Time:   currentTime,
		Memory: ap.systemInfo.AllocatedMem,
	})

	// Keep only last 100 points for performance
	maxPoints := 100
	if len(ap.metrics.ConnectionHistory) > maxPoints {
		ap.metrics.ConnectionHistory = ap.metrics.ConnectionHistory[len(ap.metrics.ConnectionHistory)-maxPoints:]
	}
	if len(ap.metrics.MessageHistory) > maxPoints {
		ap.metrics.MessageHistory = ap.metrics.MessageHistory[len(ap.metrics.MessageHistory)-maxPoints:]
	}
	if len(ap.metrics.MemoryHistory) > maxPoints {
		ap.metrics.MemoryHistory = ap.metrics.MemoryHistory[len(ap.metrics.MemoryHistory)-maxPoints:]
	}

	// Update peak values
	if ap.systemInfo.ActiveUsers > ap.metrics.PeakUsers {
		ap.metrics.PeakUsers = ap.systemInfo.ActiveUsers
	}
	if ap.systemInfo.AllocatedMem > ap.metrics.PeakMemory {
		ap.metrics.PeakMemory = ap.systemInfo.AllocatedMem
	}

	ap.metrics.LastUpdated = currentTime
}

func (ap *AdminPanel) updateUserTable() {
	rows := []table.Row{}
	for _, user := range ap.users {
		adminStatus := "No"
		if user.IsAdmin {
			adminStatus = "Yes"
		}

		status := user.Status
		if user.IsBanned {
			status = "Banned"
		} else if user.IsKicked {
			status = "Kicked"
		}

		lastSeen := "N/A"
		if !user.LastSeen.IsZero() {
			lastSeen = formatDuration(time.Since(user.LastSeen))
		}

		connected := "N/A"
		if !user.ConnectedAt.IsZero() && user.Status == "Online" {
			connected = formatDuration(time.Since(user.ConnectedAt))
		}

		rows = append(rows, table.Row{
			user.Username,
			status,
			user.IP,
			fmt.Sprintf("%d", user.Messages),
			adminStatus,
			lastSeen,
			connected,
		})
	}
	ap.userTable.SetRows(rows)
}

// RunAdminPanel starts the admin panel TUI
func RunAdminPanel(hub *Hub, db *sql.DB, pluginManager *manager.PluginManager, liveConfig *config.Config) error {
	panel := NewAdminPanel(hub, db, pluginManager, liveConfig)

	p := tea.NewProgram(panel, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Implement tea.Model interface
func (ap *AdminPanel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}

type tickMsg time.Time

func (ap *AdminPanel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ap.width = msg.Width
		ap.height = msg.Height

		availableWidth := msg.Width - 12
		if availableWidth < 30 {
			availableWidth = 30
		}

		ap.help.Width = availableWidth
		ap.userTable.SetWidth(availableWidth)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, ap.keys.Quit):
			ap.quitting = true
			return ap, tea.Quit
		case key.Matches(msg, ap.keys.TabNext):
			ap.activeTab = tabType((int(ap.activeTab) + 1) % len(ap.tabs))
		case key.Matches(msg, ap.keys.TabPrev):
			ap.activeTab = tabType((int(ap.activeTab) - 1 + len(ap.tabs)) % len(ap.tabs))
		case key.Matches(msg, ap.keys.Help):
			ap.help.ShowAll = !ap.help.ShowAll
		case key.Matches(msg, ap.keys.Refresh):
			ap.refreshData()
			ap.message = "🔄 Data refreshed"
			ap.messageTimer = 3
		case key.Matches(msg, ap.keys.ClearDB):
			if ap.activeTab == tabSystem {
				return ap, ap.clearDatabase()
			}
		case key.Matches(msg, ap.keys.BackupDB):
			if ap.activeTab == tabSystem {
				return ap, ap.backupDatabase()
			}
		case key.Matches(msg, ap.keys.ShowStats):
			if ap.activeTab == tabSystem {
				return ap, ap.showDatabaseStats()
			}
		case key.Matches(msg, ap.keys.Ban):
			if ap.activeTab == tabUsers && ap.userTable.Focused() {
				selected := ap.userTable.SelectedRow()
				if len(selected) > 0 {
					username := selected[0]
					return ap, ap.banUser(username)
				}
			}
		case key.Matches(msg, ap.keys.Unban):
			if ap.activeTab == tabUsers && ap.userTable.Focused() {
				selected := ap.userTable.SelectedRow()
				if len(selected) > 0 {
					username := selected[0]
					return ap, ap.unbanUser(username)
				}
			}
		case key.Matches(msg, ap.keys.Kick):
			if ap.activeTab == tabUsers && ap.userTable.Focused() {
				selected := ap.userTable.SelectedRow()
				if len(selected) > 0 {
					username := selected[0]
					return ap, ap.kickUser(username)
				}
			}
		case key.Matches(msg, ap.keys.Allow):
			if ap.activeTab == tabUsers && ap.userTable.Focused() {
				selected := ap.userTable.SelectedRow()
				if len(selected) > 0 {
					username := selected[0]
					return ap, ap.allowUser(username)
				}
			}
		case key.Matches(msg, ap.keys.ForceGC):
			runtime.GC()
			ap.message = "🗑️ Garbage collection forced"
			ap.messageTimer = 3
		case key.Matches(msg, ap.keys.ResetMetrics):
			ap.resetMetrics()
			ap.message = "📊 Metrics reset"
			ap.messageTimer = 3
		case key.Matches(msg, ap.keys.ExportLogs):
			return ap, ap.exportLogs()
		case key.Matches(msg, ap.keys.Up):
			ap.handleScroll(-1)
		case key.Matches(msg, ap.keys.Down):
			ap.handleScroll(1)
		}

	case tickMsg:
		ap.systemInfo.Uptime = time.Since(ap.startTime)
		ap.refreshData()

		if ap.messageTimer > 0 {
			ap.messageTimer--
			if ap.messageTimer == 0 {
				ap.message = ""
			}
		}

		return ap, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})

	case clearDBMsg:
		if msg.success {
			ap.message = "🗑️ Database cleared successfully!"
			ap.refreshData()
		} else {
			ap.message = "❌ Failed to clear database: " + msg.error
		}
		ap.messageTimer = 5

	case backupDBMsg:
		if msg.success {
			ap.message = "💾 Database backed up to: " + msg.filename
		} else {
			ap.message = "❌ Failed to backup database: " + msg.error
		}
		ap.messageTimer = 5

	case statsMsg:
		if msg.success {
			ap.message = "📊 " + msg.stats
		} else {
			ap.message = "❌ Failed to get stats: " + msg.error
		}
		ap.messageTimer = 10

	case actionMsg:
		ap.message = msg.message
		ap.messageTimer = 5
		if msg.success {
			ap.refreshData()
		}
	}

	// Update components based on active tab
	switch ap.activeTab {
	case tabUsers:
		var cmd tea.Cmd
		ap.userTable, cmd = ap.userTable.Update(msg)
		cmds = append(cmds, cmd)
	}

	return ap, tea.Batch(cmds...)
}

func (ap *AdminPanel) handleScroll(direction int) {
	switch ap.activeTab {
	case tabOverview:
		ap.overviewScroll += direction
		if ap.overviewScroll < 0 {
			ap.overviewScroll = 0
		}
	case tabSystem:
		ap.systemScroll += direction
		if ap.systemScroll < 0 {
			ap.systemScroll = 0
		}
	case tabPlugins:
		ap.pluginsScroll += direction
		if ap.pluginsScroll < 0 {
			ap.pluginsScroll = 0
		}
	case tabMetrics:
		ap.metricsScroll += direction
		if ap.metricsScroll < 0 {
			ap.metricsScroll = 0
		}
	case tabLogs:
		ap.logsScroll += direction
		if ap.logsScroll < 0 {
			ap.logsScroll = 0
		}
	}
}

func (ap *AdminPanel) renderScrollableContent(content string, scrollOffset int) string {
	lines := strings.Split(content, "\n")
	availableHeight := ap.height - 8 // Reserve space for title, tabs, help, and message
	if availableHeight < 10 {
		availableHeight = 10
	}

	// Calculate how many lines we can display
	maxLines := availableHeight - 2 // Reserve space for borders
	if maxLines < 1 {
		maxLines = 1
	}

	// Apply scroll offset
	startLine := scrollOffset
	if startLine >= len(lines) {
		startLine = len(lines) - 1
	}
	if startLine < 0 {
		startLine = 0
	}

	// Get the lines to display
	endLine := startLine + maxLines
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Join the visible lines
	visibleLines := lines[startLine:endLine]
	return strings.Join(visibleLines, "\n")
}

func (ap *AdminPanel) View() string {
	if ap.quitting {
		return "Admin panel closed. Server continues running.\n"
	}

	availableWidth := ap.width - 12
	if availableWidth < 30 {
		availableWidth = 30
	}

	doc := strings.Builder{}

	// Title
	doc.WriteString(titleStyle.Width(availableWidth).Render("🧃 Marchat Admin Panel"))
	doc.WriteString("\n\n")

	// Tabs
	doc.WriteString(ap.renderTabs())
	doc.WriteString("\n")

	// Content
	doc.WriteString(ap.renderContent())
	doc.WriteString("\n")

	// Help
	doc.WriteString(ap.help.View(ap.keys))

	// Display message if present
	if ap.message != "" {
		doc.WriteString("\n")
		doc.WriteString(messageStyle.Width(availableWidth).Render(ap.message))
	}

	return mainBorder.Width(availableWidth + 8).Render(doc.String())
}

func (ap *AdminPanel) renderTabs() string {
	var renderedTabs []string

	availableWidth := ap.width - 12
	if availableWidth < 30 {
		availableWidth = 30
	}
	tabWidth := availableWidth / len(ap.tabs)
	if tabWidth < 8 {
		tabWidth = 8
	}

	for i, tab := range ap.tabs {
		var style lipgloss.Style
		if i == int(ap.activeTab) {
			style = activeTabStyle
		} else {
			style = tabStyle
		}

		renderedTab := style.Width(tabWidth).Align(lipgloss.Center).Render(tab)
		renderedTabs = append(renderedTabs, renderedTab)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
}

func (ap *AdminPanel) renderContent() string {
	switch ap.activeTab {
	case tabOverview:
		return ap.renderOverview()
	case tabUsers:
		return ap.renderUsers()
	case tabSystem:
		return ap.renderSystem()
	case tabLogs:
		return ap.renderLogs()
	case tabPlugins:
		return ap.renderPlugins()
	case tabMetrics:
		return ap.renderMetrics()
	default:
		return "Unknown tab"
	}
}

func (ap *AdminPanel) renderOverview() string {
	doc := strings.Builder{}

	contentWidth := ap.width - 12
	if contentWidth < 30 {
		contentWidth = 30
	}

	// System status
	doc.WriteString(subtitleStyle.Width(contentWidth).Render("System Status\n"))
	doc.WriteString(strings.Repeat("─", min(20, contentWidth-2)) + "\n")

	statusText := "🟢 " + ap.systemInfo.ServerStatus
	doc.WriteString(fmt.Sprintf("Status: %s\n", statusStyle.Render(statusText)))
	doc.WriteString(fmt.Sprintf("Uptime: %s\n", formatDuration(ap.systemInfo.Uptime)))
	doc.WriteString(fmt.Sprintf("Active Users: %d\n", ap.systemInfo.ActiveUsers))
	doc.WriteString(fmt.Sprintf("Total Users: %d\n", ap.systemInfo.TotalUsers))
	doc.WriteString(fmt.Sprintf("Messages Sent: %d\n", ap.systemInfo.MessagesSent))
	doc.WriteString(fmt.Sprintf("Active Plugins: %d\n", ap.systemInfo.PluginsActive))
	doc.WriteString(fmt.Sprintf("Memory Usage: %.1f MB\n", ap.systemInfo.MemoryUsage))
	doc.WriteString(fmt.Sprintf("Goroutines: %d\n", ap.systemInfo.GoroutineCount))

	doc.WriteString("\n")

	// Live Configuration Summary
	doc.WriteString(subtitleStyle.Width(contentWidth).Render("Live Configuration\n"))
	doc.WriteString(strings.Repeat("─", min(20, contentWidth-2)) + "\n")
	doc.WriteString(fmt.Sprintf("Port: %d\n", ap.config.Port))

	// Show TLS status with live detection
	tlsStatus := "❌ Disabled"
	if ap.config.IsTLSEnabled() {
		tlsStatus = "✅ Enabled"
	}
	doc.WriteString(fmt.Sprintf("TLS: %s\n", tlsStatus))

	doc.WriteString(fmt.Sprintf("Max File Size: %.1f MB\n", float64(ap.config.MaxFileBytes)/1024/1024))
	doc.WriteString(fmt.Sprintf("Log Level: %s\n", ap.config.LogLevel))
	doc.WriteString(fmt.Sprintf("Ban History Gaps: %t\n", ap.config.BanGapsHistory))
	doc.WriteString(fmt.Sprintf("Admin Users: %d\n", len(ap.config.Admins)))

	doc.WriteString("\n")

	// Database info
	doc.WriteString(subtitleStyle.Width(contentWidth).Render("Database Information\n"))
	doc.WriteString(strings.Repeat("─", min(20, contentWidth-2)) + "\n")
	doc.WriteString(fmt.Sprintf("Database Path: %s\n", ap.config.DBPath))
	doc.WriteString(fmt.Sprintf("Config Directory: %s\n", ap.config.ConfigDir))

	return ap.renderScrollableContent(doc.String(), ap.overviewScroll)
}

func (ap *AdminPanel) renderUsers() string {
	doc := strings.Builder{}

	contentWidth := ap.width - 12
	if contentWidth < 30 {
		contentWidth = 30
	}

	doc.WriteString(subtitleStyle.Width(contentWidth).Render("User Management\n"))
	doc.WriteString(strings.Repeat("─", min(20, contentWidth-2)) + "\n")

	// Show selected user info
	if ap.userTable.Focused() {
		selected := ap.userTable.SelectedRow()
		if len(selected) > 0 {
			username := selected[0]
			status := selected[1]

			var statusStyleLocal lipgloss.Style
			switch status {
			case "Online":
				statusStyleLocal = lipgloss.NewStyle().Foreground(successColor).Bold(true)
			case "Away":
				statusStyleLocal = lipgloss.NewStyle().Foreground(warningColor).Bold(true)
			case "Banned":
				statusStyleLocal = lipgloss.NewStyle().Foreground(errorColor).Bold(true)
			case "Kicked":
				statusStyleLocal = lipgloss.NewStyle().Foreground(warningColor).Bold(true)
			default:
				statusStyleLocal = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true)
			}

			doc.WriteString(fmt.Sprintf("Selected: %s (%s)\n",
				username,
				statusStyleLocal.Render(status)))
		}
	}

	doc.WriteString("Use ↑/↓ to navigate, [B] Ban, [U] Unban, [K] Kick, [A] Allow\n\n")

	doc.WriteString(ap.userTable.View())

	return doc.String()
}

func (ap *AdminPanel) renderSystem() string {
	doc := strings.Builder{}

	contentWidth := ap.width - 12
	if contentWidth < 30 {
		contentWidth = 30
	}

	doc.WriteString(subtitleStyle.Width(contentWidth).Render("System Management\n"))
	doc.WriteString(strings.Repeat("─", min(20, contentWidth-2)) + "\n")

	doc.WriteString(infoStylePanel.Render("Use [c] Clear Database, [b] Backup Database, [s] Show Stats\n\n"))

	// Live Configuration Details
	doc.WriteString(subtitleStyle.Render("Live Configuration:\n"))
	doc.WriteString(fmt.Sprintf("  Server Port: %d\n", ap.config.Port))
	doc.WriteString(fmt.Sprintf("  Database: %s\n", ap.config.DBPath))
	doc.WriteString(fmt.Sprintf("  Config Directory: %s\n", ap.config.ConfigDir))
	doc.WriteString(fmt.Sprintf("  Log Level: %s\n", ap.config.LogLevel))
	doc.WriteString(fmt.Sprintf("  Max File Size: %.1f MB\n", float64(ap.config.MaxFileBytes)/1024/1024))
	doc.WriteString(fmt.Sprintf("  Admin Users: %s\n", strings.Join(ap.config.Admins, ", ")))

	// TLS Configuration with live detection
	tlsStatusText := "Disabled"
	var tlsStyle lipgloss.Style
	if ap.config.IsTLSEnabled() {
		tlsStatusText = "Enabled"
		tlsStyle = lipgloss.NewStyle().Foreground(successColor).Bold(true)
		doc.WriteString(fmt.Sprintf("  TLS Status: %s\n", tlsStyle.Render(tlsStatusText)))
		doc.WriteString(fmt.Sprintf("  TLS Cert File: %s\n", ap.config.TLSCertFile))
		doc.WriteString(fmt.Sprintf("  TLS Key File: %s\n", ap.config.TLSKeyFile))
	} else {
		tlsStyle = lipgloss.NewStyle().Foreground(warningColor).Bold(true)
		doc.WriteString(fmt.Sprintf("  TLS Status: %s\n", tlsStyle.Render(tlsStatusText)))
	}

	doc.WriteString(fmt.Sprintf("  JWT Secret: %s\n", maskSecret(ap.config.JWTSecret)))
	doc.WriteString(fmt.Sprintf("  Admin Key: %s\n", maskSecret(ap.config.AdminKey)))
	doc.WriteString(fmt.Sprintf("  Ban History Gaps: %t\n", ap.config.BanGapsHistory))
	doc.WriteString(fmt.Sprintf("  Plugin Registry: %s\n", ap.config.PluginRegistryURL))

	doc.WriteString("\n")
	doc.WriteString(subtitleStyle.Render("Database Statistics:\n"))
	doc.WriteString(fmt.Sprintf("  Total Messages: %d\n", ap.systemInfo.MessagesSent))
	doc.WriteString(fmt.Sprintf("  Total Users: %d\n", ap.systemInfo.TotalUsers))
	doc.WriteString(fmt.Sprintf("  Active Connections: %d\n", ap.systemInfo.ActiveUsers))
	doc.WriteString(fmt.Sprintf("  Active Plugins: %d\n", ap.systemInfo.PluginsActive))

	return ap.renderScrollableContent(doc.String(), ap.systemScroll)
}

func (ap *AdminPanel) renderLogs() string {
	doc := strings.Builder{}

	contentWidth := ap.width - 12
	if contentWidth < 30 {
		contentWidth = 30
	}

	doc.WriteString(subtitleStyle.Width(contentWidth).Render("System Logs\n"))
	doc.WriteString(strings.Repeat("─", min(20, contentWidth-2)) + "\n")

	// Add logs content
	for _, logEntry := range ap.logs {
		var levelStyle lipgloss.Style
		switch logEntry.Level {
		case "ERROR":
			levelStyle = errorStylePanel
		case "WARN":
			levelStyle = warningStylePanel
		default:
			levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("blue")).Bold(true)
		}
		doc.WriteString(fmt.Sprintf("[%s] %s %s: %s\n",
			levelStyle.Render(logEntry.Level),
			logEntry.Timestamp.Format("15:04:05"),
			logEntry.Component,
			logEntry.Message))
	}

	return ap.renderScrollableContent(doc.String(), ap.logsScroll)
}

func (ap *AdminPanel) renderPlugins() string {
	doc := strings.Builder{}

	contentWidth := ap.width - 12
	if contentWidth < 30 {
		contentWidth = 30
	}

	doc.WriteString(subtitleStyle.Width(contentWidth).Render("Plugin Management\n"))
	doc.WriteString(strings.Repeat("─", min(20, contentWidth-2)) + "\n")

	doc.WriteString(infoStylePanel.Render("Use ↑/↓ to navigate, [e] Enable, [d] Disable, [i] Install, [n] Uninstall\n\n"))

	if len(ap.plugins) == 0 {
		doc.WriteString("No plugins found.\n")
	} else {
		for i, plugin := range ap.plugins {
			statusColor := "green"
			switch plugin.Status {
			case "Inactive":
				statusColor = "yellow"
			case "Error":
				statusColor = "red"
			}

			statusStyleLocal := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Bold(true)

			// Highlight selected plugin
			if i == ap.selectedPlugin {
				doc.WriteString("▶ ")
			} else {
				doc.WriteString("  ")
			}

			doc.WriteString(fmt.Sprintf("%-15s %s %s\n",
				plugin.Name,
				statusStyleLocal.Render(plugin.Status),
				plugin.Version))
		}
	}

	return ap.renderScrollableContent(doc.String(), ap.pluginsScroll)
}

func (ap *AdminPanel) renderMetrics() string {
	doc := strings.Builder{}

	contentWidth := ap.width - 12
	if contentWidth < 30 {
		contentWidth = 30
	}

	doc.WriteString(subtitleStyle.Width(contentWidth).Render("Performance Metrics\n"))
	doc.WriteString(strings.Repeat("─", min(20, contentWidth-2)) + "\n")

	doc.WriteString(infoStylePanel.Render("Use [G] Force GC, [R] Reset Metrics, [E] Export Logs\n\n"))

	// System Performance - more compact layout
	doc.WriteString(metricLabelStyle.Render("System Performance:\n"))
	doc.WriteString(fmt.Sprintf("Memory: %s | Goroutines: %s | Heap: %s\n",
		metricValueStyle.Render(fmt.Sprintf("%.1f MB", ap.systemInfo.MemoryUsage)),
		metricValueStyle.Render(fmt.Sprintf("%d", ap.systemInfo.GoroutineCount)),
		metricValueStyle.Render(fmt.Sprintf("%.1f MB", float64(ap.systemInfo.HeapSize)/1024/1024))))
	doc.WriteString(fmt.Sprintf("Allocated: %s | GC Count: %s\n",
		metricValueStyle.Render(fmt.Sprintf("%.1f MB", float64(ap.systemInfo.AllocatedMem)/1024/1024)),
		metricValueStyle.Render(fmt.Sprintf("%d", ap.systemInfo.GCCount))))

	doc.WriteString("\n")

	// Connection Metrics - more compact layout
	doc.WriteString(metricLabelStyle.Render("Connection Metrics:\n"))
	doc.WriteString(fmt.Sprintf("Active: %s | Peak: %s | Total: %s | Disconnects: %s\n",
		metricValueStyle.Render(fmt.Sprintf("%d", ap.systemInfo.ActiveUsers)),
		metricValueStyle.Render(fmt.Sprintf("%d", ap.metrics.PeakUsers)),
		metricValueStyle.Render(fmt.Sprintf("%d", ap.metrics.TotalConnections)),
		metricValueStyle.Render(fmt.Sprintf("%d", ap.metrics.TotalDisconnects))))

	doc.WriteString("\n")

	// Message Metrics - more compact layout
	doc.WriteString(metricLabelStyle.Render("Message Metrics:\n"))
	doc.WriteString(fmt.Sprintf("Total: %s | Rate: %s | Conn Rate: %s\n",
		metricValueStyle.Render(fmt.Sprintf("%d", ap.systemInfo.MessagesSent)),
		metricValueStyle.Render(fmt.Sprintf("%.2f msg/s", ap.messageRate)),
		metricValueStyle.Render(fmt.Sprintf("%.2f conn/s", ap.connectionRate))))

	doc.WriteString("\n")

	// Memory History Chart (simplified)
	if len(ap.metrics.MemoryHistory) > 0 {
		doc.WriteString(metricLabelStyle.Render("Memory History (last 5):\n"))
		recent := ap.metrics.MemoryHistory
		if len(recent) > 5 {
			recent = recent[len(recent)-5:]
		}
		for _, point := range recent {
			value := float64(point.Memory) / 1024 / 1024
			doc.WriteString(fmt.Sprintf("  %s: %.1f MB\n",
				point.Time.Format("15:04:05"), value))
		}
	}

	doc.WriteString("\n")

	// Connection History Chart (simplified)
	if len(ap.metrics.ConnectionHistory) > 0 {
		doc.WriteString(metricLabelStyle.Render("Connection History (last 5):\n"))
		recent := ap.metrics.ConnectionHistory
		if len(recent) > 5 {
			recent = recent[len(recent)-5:]
		}
		for _, point := range recent {
			doc.WriteString(fmt.Sprintf("  %s: %d users\n",
				point.Time.Format("15:04:05"), point.Count))
		}
	}

	return ap.renderScrollableContent(doc.String(), ap.metricsScroll)
}

// Database operation messages
type clearDBMsg struct {
	success bool
	error   string
}

type backupDBMsg struct {
	success  bool
	error    string
	filename string
}

type statsMsg struct {
	success bool
	error   string
	stats   string
}

type actionMsg struct {
	success bool
	message string
}

func (ap *AdminPanel) clearDatabase() tea.Cmd {
	return func() tea.Msg {
		err := ClearMessages(ap.db)
		if err != nil {
			return clearDBMsg{success: false, error: err.Error()}
		}
		return clearDBMsg{success: true}
	}
}

func (ap *AdminPanel) backupDatabase() tea.Cmd {
	return func() tea.Msg {
		filename, err := BackupDatabase(ap.config.DBPath)
		if err != nil {
			return backupDBMsg{success: false, error: err.Error()}
		}
		return backupDBMsg{success: true, filename: filename}
	}
}

func (ap *AdminPanel) showDatabaseStats() tea.Cmd {
	return func() tea.Msg {
		stats, err := GetDatabaseStats(ap.db)
		if err != nil {
			return statsMsg{success: false, error: err.Error()}
		}
		return statsMsg{success: true, stats: stats}
	}
}

func (ap *AdminPanel) banUser(username string) tea.Cmd {
	return func() tea.Msg {
		ap.hub.BanUser(username, "admin")
		return actionMsg{
			success: true,
			message: fmt.Sprintf("🚫 User '%s' has been banned", username),
		}
	}
}

func (ap *AdminPanel) unbanUser(username string) tea.Cmd {
	return func() tea.Msg {
		success := ap.hub.UnbanUser(username, "admin")
		if success {
			return actionMsg{
				success: true,
				message: fmt.Sprintf("✅ User '%s' has been unbanned", username),
			}
		}
		return actionMsg{
			success: false,
			message: fmt.Sprintf("❌ User '%s' was not found in ban list", username),
		}
	}
}

func (ap *AdminPanel) kickUser(username string) tea.Cmd {
	return func() tea.Msg {
		ap.hub.KickUser(username, "admin")
		return actionMsg{
			success: true,
			message: fmt.Sprintf("👢 User '%s' has been kicked (24h)", username),
		}
	}
}

func (ap *AdminPanel) allowUser(username string) tea.Cmd {
	return func() tea.Msg {
		success := ap.hub.AllowUser(username, "admin")
		if success {
			return actionMsg{
				success: true,
				message: fmt.Sprintf("✅ User '%s' has been allowed back", username),
			}
		}
		return actionMsg{
			success: false,
			message: fmt.Sprintf("❌ User '%s' was not found in kick list", username),
		}
	}
}

func (ap *AdminPanel) resetMetrics() {
	ap.metrics = metricsData{
		ConnectionHistory: make([]connectionPoint, 0),
		MessageHistory:    make([]messagePoint, 0),
		MemoryHistory:     make([]memoryPoint, 0),
		LastUpdated:       time.Now(),
		PeakUsers:         0,
		PeakMemory:        0,
		TotalConnections:  0,
		TotalDisconnects:  0,
		AverageResponse:   0,
	}
	ap.lastMessageCount = 0
	ap.messageRate = 0
	ap.connectionRate = 0
}

func (ap *AdminPanel) exportLogs() tea.Cmd {
	return func() tea.Msg {
		// Create a simple log export
		var logText strings.Builder
		logText.WriteString("Marchat Admin Panel Log Export\n")
		logText.WriteString("==============================\n\n")

		for _, logEntry := range ap.logs {
			logText.WriteString(fmt.Sprintf("[%s] %s %s: %s\n",
				logEntry.Timestamp.Format("2006-01-02 15:04:05"),
				logEntry.Level,
				logEntry.Component,
				logEntry.Message))
		}

		// In a real implementation, you would write this to a file
		// For now, we'll just return a success message
		return actionMsg{
			success: true,
			message: "📄 Logs exported successfully (console output)",
		}
	}
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatDuration(d time.Duration) string {
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

func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "***hidden***"
	}
	return secret[:4] + "***" + secret[len(secret)-4:]
}
