package server

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/Cod-e-Codes/marchat/plugin/manager"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
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
	Uptime        time.Duration
	MemoryUsage   float64
	CPUUsage      float64
	ActiveUsers   int
	TotalUsers    int
	MessagesSent  int
	PluginsActive int
	ServerStatus  string
}

// Configuration data
type configData struct {
	Port        int
	AdminKey    string
	DBPath      string
	LogLevel    string
	MaxMessages int
	ConfigDir   string
}

// Log entry
type logEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	User      string
}

// AdminPanel represents the main admin panel state
type AdminPanel struct {
	// Navigation
	activeTab tabType
	tabs      []string

	// Components
	help      help.Model
	userTable table.Model
	logViewer textarea.Model

	// Data
	users      []userInfo
	plugins    []pluginInfo
	systemInfo systemStats
	config     configData
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

	// Keybindings
	keys keyMap
}

// Keybindings
type keyMap struct {
	TabNext   key.Binding
	TabPrev   key.Binding
	Quit      key.Binding
	Refresh   key.Binding
	ClearDB   key.Binding
	BackupDB  key.Binding
	ShowStats key.Binding
	Help      key.Binding
	Action    key.Binding
	Ban       key.Binding
	Unban     key.Binding
	Kick      key.Binding
	Allow     key.Binding
	AddAdmin  key.Binding
	Enable    key.Binding
	Disable   key.Binding
	Install   key.Binding
	Uninstall key.Binding
	Up        key.Binding
	Down      key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.TabNext, k.TabPrev, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.TabNext, k.TabPrev, k.Refresh},
		{k.Action, k.Help, k.Quit},
		{k.Ban, k.Unban, k.Kick, k.Allow, k.AddAdmin},
		{k.Enable, k.Disable, k.Install, k.Uninstall},
		{k.ClearDB, k.BackupDB, k.ShowStats},
	}
}

// Styling
var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4")
	secondaryColor = lipgloss.Color("#FF75B7")
	successColor   = lipgloss.Color("#00FFA3")
	warningColor   = lipgloss.Color("#FFA500")
	errorColor     = lipgloss.Color("#FF4444")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginLeft(2)

	tabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 1)

	activeTabStyle = tabStyle.
			Foreground(primaryColor).
			Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	errorStylePanel = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	warningStylePanel = lipgloss.NewStyle().
				Foreground(warningColor).
				Bold(true)

	// Border styles
	mainBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	contentStyle = lipgloss.NewStyle().
			Padding(1, 0)

	messageStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true).
			Padding(0, 1)
)

// NewAdminPanel creates a new admin panel instance
func NewAdminPanel(hub *Hub, db *sql.DB, pluginManager *manager.PluginManager, configDir, dbPath string, port int) *AdminPanel {
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
	}

	// Initialize table
	columns := []table.Column{
		{Title: "Username", Width: 15},
		{Title: "Status", Width: 10},
		{Title: "IP", Width: 15},
		{Title: "Messages", Width: 10},
		{Title: "Admin", Width: 8},
		{Title: "Last Seen", Width: 12},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(primaryColor).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(secondaryColor).
		Bold(false)
	t.SetStyles(s)

	// Initialize log viewer
	logViewer := textarea.New()
	logViewer.Placeholder = "System logs will appear here..."
	logViewer.SetHeight(15)
	logViewer.SetWidth(80)
	logViewer.ShowLineNumbers = true
	logViewer.FocusedStyle.CursorLine = lipgloss.NewStyle().
		Background(lipgloss.Color("57")).
		Foreground(lipgloss.Color("230"))

	panel := &AdminPanel{
		activeTab:     tabOverview,
		tabs:          []string{"Overview", "Users", "System", "Logs", "Plugins"},
		help:          help.New(),
		userTable:     t,
		logViewer:     logViewer,
		keys:          keys,
		hub:           hub,
		db:            db,
		pluginManager: pluginManager,
		startTime:     time.Now(),
		config: configData{
			Port:        port,
			AdminKey:    "***hidden***",
			DBPath:      dbPath,
			LogLevel:    "info",
			MaxMessages: 1000, // Actual limit from handlers.go
			ConfigDir:   configDir,
		},
		systemInfo: systemStats{
			Uptime:       0,
			MemoryUsage:  0,
			CPUUsage:     0,
			ServerStatus: "Running",
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
			user.ConnectedAt = time.Now() // Simplified
			user.LastSeen = time.Now()
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
	// Create some sample logs based on system activity
	ap.logs = []logEntry{
		{Timestamp: time.Now().Add(-1 * time.Minute), Level: "INFO", Message: "Admin panel started", User: "Admin"},
		{Timestamp: time.Now().Add(-2 * time.Minute), Level: "INFO", Message: fmt.Sprintf("Active users: %d", len(ap.hub.clients)), User: "System"},
		{Timestamp: time.Now().Add(-5 * time.Minute), Level: "INFO", Message: "Server running normally", User: "System"},
	}
}

func (ap *AdminPanel) updateSystemStats() {
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

	ap.systemInfo.MessagesSent = messageCount
	ap.systemInfo.TotalUsers = userCount
	ap.systemInfo.ActiveUsers = len(ap.hub.clients)
	ap.systemInfo.PluginsActive = activePlugins
	ap.systemInfo.Uptime = time.Since(ap.startTime)
	ap.systemInfo.ServerStatus = "Running"
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

		rows = append(rows, table.Row{
			user.Username,
			status,
			user.IP,
			fmt.Sprintf("%d", user.Messages),
			adminStatus,
			lastSeen,
		})
	}
	ap.userTable.SetRows(rows)
}

// RunAdminPanel starts the admin panel TUI
func RunAdminPanel(hub *Hub, db *sql.DB, pluginManager *manager.PluginManager, configDir, dbPath string, port int) error {
	panel := NewAdminPanel(hub, db, pluginManager, configDir, dbPath, port)

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
		ap.logViewer.SetWidth(availableWidth)

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
	case tabLogs:
		var cmd tea.Cmd
		ap.logViewer, cmd = ap.logViewer.Update(msg)
		cmds = append(cmds, cmd)
	}

	return ap, tea.Batch(cmds...)
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
	doc.WriteString(contentStyle.Width(contentWidth).Render("System Status\n"))
	doc.WriteString(strings.Repeat("─", min(20, contentWidth-2)) + "\n")

	statusText := "🟢 " + ap.systemInfo.ServerStatus
	doc.WriteString(fmt.Sprintf("Status: %s\n", statusStyle.Render(statusText)))
	doc.WriteString(fmt.Sprintf("Uptime: %s\n", formatDuration(ap.systemInfo.Uptime)))
	doc.WriteString(fmt.Sprintf("Active Users: %d\n", ap.systemInfo.ActiveUsers))
	doc.WriteString(fmt.Sprintf("Total Users: %d\n", ap.systemInfo.TotalUsers))
	doc.WriteString(fmt.Sprintf("Messages Sent: %d\n", ap.systemInfo.MessagesSent))
	doc.WriteString(fmt.Sprintf("Active Plugins: %d\n", ap.systemInfo.PluginsActive))

	doc.WriteString("\n")

	// Database info
	doc.WriteString(contentStyle.Width(contentWidth).Render("Database Information\n"))
	doc.WriteString(strings.Repeat("─", min(20, contentWidth-2)) + "\n")
	doc.WriteString(fmt.Sprintf("Database Path: %s\n", ap.config.DBPath))
	doc.WriteString(fmt.Sprintf("Config Directory: %s\n", ap.config.ConfigDir))

	return doc.String()
}

func (ap *AdminPanel) renderUsers() string {
	doc := strings.Builder{}

	contentWidth := ap.width - 12
	if contentWidth < 30 {
		contentWidth = 30
	}

	doc.WriteString(contentStyle.Width(contentWidth).Render("User Management\n"))
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

	doc.WriteString(contentStyle.Width(contentWidth).Render("System Management\n"))
	doc.WriteString(strings.Repeat("─", min(20, contentWidth-2)) + "\n")

	doc.WriteString("Use [c] Clear Database, [b] Backup Database, [s] Show Stats\n\n")

	doc.WriteString("Configuration:\n")
	doc.WriteString(fmt.Sprintf("  Server Port: %d\n", ap.config.Port))
	doc.WriteString(fmt.Sprintf("  Database: %s\n", ap.config.DBPath))
	doc.WriteString(fmt.Sprintf("  Config Directory: %s\n", ap.config.ConfigDir))
	doc.WriteString(fmt.Sprintf("  Log Level: %s\n", ap.config.LogLevel))
	doc.WriteString(fmt.Sprintf("  Max Messages: %d (database limit)\n", ap.config.MaxMessages))
	doc.WriteString("  Max Users: No limit (system resources)\n")
	doc.WriteString("  Max File Size: 1MB\n")
	doc.WriteString("  WebSocket Buffer: 1MB+ per connection\n")

	doc.WriteString("\n")
	doc.WriteString("Database Statistics:\n")
	doc.WriteString(fmt.Sprintf("  Total Messages: %d\n", ap.systemInfo.MessagesSent))
	doc.WriteString(fmt.Sprintf("  Total Users: %d\n", ap.systemInfo.TotalUsers))
	doc.WriteString(fmt.Sprintf("  Active Plugins: %d\n", ap.systemInfo.PluginsActive))

	return doc.String()
}

func (ap *AdminPanel) renderLogs() string {
	doc := strings.Builder{}

	contentWidth := ap.width - 12
	if contentWidth < 30 {
		contentWidth = 30
	}

	doc.WriteString(contentStyle.Width(contentWidth).Render("System Logs\n"))
	doc.WriteString(strings.Repeat("─", min(20, contentWidth-2)) + "\n")

	// Update log viewer with recent logs
	var logText strings.Builder
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
		logText.WriteString(fmt.Sprintf("[%s] %s: %s\n",
			levelStyle.Render(logEntry.Level),
			logEntry.Timestamp.Format("15:04:05"),
			logEntry.Message))
	}

	ap.logViewer.SetValue(logText.String())
	doc.WriteString(ap.logViewer.View())

	return doc.String()
}

func (ap *AdminPanel) renderPlugins() string {
	doc := strings.Builder{}

	contentWidth := ap.width - 12
	if contentWidth < 30 {
		contentWidth = 30
	}

	doc.WriteString(contentStyle.Width(contentWidth).Render("Plugin Management\n"))
	doc.WriteString(strings.Repeat("─", min(20, contentWidth-2)) + "\n")

	doc.WriteString("Use ↑/↓ to navigate, [e] Enable, [d] Disable, [i] Install, [n] Uninstall\n\n")

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

	return doc.String()
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
