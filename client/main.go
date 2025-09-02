package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Cod-e-Codes/marchat/client/config"
	"github.com/Cod-e-Codes/marchat/client/crypto"
	"github.com/Cod-e-Codes/marchat/shared"

	"os/signal"
	"syscall"

	"encoding/base64"
	"encoding/json"

	"context"
	"sync"

	"log"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

const maxMessages = 100
const maxUsersDisplay = 20
const userListWidth = 18
const pingPeriod = 50 * time.Second        // moved from magic number
const reconnectMaxDelay = 30 * time.Second // for exponential backoff

var mentionRegex *regexp.Regexp

// keyMap defines all keybindings for the help system
type keyMap struct {
	Send       key.Binding
	ScrollUp   key.Binding
	ScrollDown key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
	Copy       key.Binding
	Paste      key.Binding
	Cut        key.Binding
	SelectAll  key.Binding
	Help       key.Binding
	Quit       key.Binding
	TimeFormat key.Binding
	Clear      key.Binding
	// Commands
	SendFile key.Binding
	SaveFile key.Binding
	Theme    key.Binding
	// Admin commands (populated dynamically)
	ClearDB key.Binding
	Kick    key.Binding
	Ban     key.Binding
	Unban   key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns keybindings for the expanded help view
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Send, k.ScrollUp, k.ScrollDown, k.PageUp, k.PageDown},
		{k.Copy, k.Paste, k.Cut, k.SelectAll},
		{k.TimeFormat, k.Clear, k.Help, k.Quit},
	}
}

// GetCommandHelp returns command-specific help based on user permissions
func (k keyMap) GetCommandHelp(isAdmin, useE2E bool) [][]key.Binding {
	commands := [][]key.Binding{
		{k.SendFile, k.SaveFile, k.Theme},
	}

	// Individual E2E commands removed - only global E2E encryption is supported

	if isAdmin {
		commands = append(commands, []key.Binding{k.ClearDB, k.Kick, k.Ban, k.Unban})
	}

	return commands
}

// newKeyMap creates a new keymap for the application
// This function is kept for potential future use in help documentation
func newKeyMap() keyMap {
	return keyMap{
		Send: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send message"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "scroll up"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdown", "page down"),
		),
		Copy: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "copy"),
		),
		Paste: key.NewBinding(
			key.WithKeys("ctrl+v"),
			key.WithHelp("ctrl+v", "paste"),
		),
		Cut: key.NewBinding(
			key.WithKeys("ctrl+x"),
			key.WithHelp("ctrl+x", "cut"),
		),
		SelectAll: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("ctrl+a", "select all"),
		),
		Help: key.NewBinding(
			key.WithKeys("ctrl+h"),
			key.WithHelp("ctrl+h", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "quit"),
		),
		TimeFormat: key.NewBinding(
			key.WithKeys(":time"),
			key.WithHelp(":time", "toggle 12/24h format"),
		),
		Clear: key.NewBinding(
			key.WithKeys(":clear"),
			key.WithHelp(":clear", "clear chat history"),
		),
		SendFile: key.NewBinding(
			key.WithKeys(":sendfile"),
			key.WithHelp(":sendfile <path>", "send a file"),
		),
		SaveFile: key.NewBinding(
			key.WithKeys(":savefile"),
			key.WithHelp(":savefile <name>", "save received file"),
		),
		Theme: key.NewBinding(
			key.WithKeys(":theme"),
			key.WithHelp(":theme <name>", "change theme"),
		),
		// Individual E2E key bindings removed - only global E2E encryption supported
		ClearDB: key.NewBinding(
			key.WithKeys(":cleardb"),
			key.WithHelp(":cleardb", "clear server database (admin)"),
		),
		Kick: key.NewBinding(
			key.WithKeys(":kick"),
			key.WithHelp(":kick <user>", "kick user (admin)"),
		),
		Ban: key.NewBinding(
			key.WithKeys(":ban"),
			key.WithHelp(":ban <user>", "ban user (admin)"),
		),
		Unban: key.NewBinding(
			key.WithKeys(":unban"),
			key.WithHelp(":unban <user>", "unban user (admin)"),
		),
	}
}

// sortMessagesByTimestamp ensures messages are displayed in chronological order
// This provides client-side protection against server ordering issues
func sortMessagesByTimestamp(messages []shared.Message) {
	sort.Slice(messages, func(i, j int) bool {
		// Primary sort: by timestamp
		if !messages[i].CreatedAt.Equal(messages[j].CreatedAt) {
			return messages[i].CreatedAt.Before(messages[j].CreatedAt)
		}
		// Secondary sort: by sender for deterministic ordering when timestamps are identical
		if messages[i].Sender != messages[j].Sender {
			return messages[i].Sender < messages[j].Sender
		}
		// Tertiary sort: by content for full deterministic ordering
		return messages[i].Content < messages[j].Content
	})
}

func init() {
	mentionRegex = regexp.MustCompile(`\B@([a-zA-Z0-9_]+)\b`)
	// Set up debug logger
	f, err := os.OpenFile("marchat-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(f)
	} else {
		log.SetOutput(os.Stderr)
	}
}

var (
	configPath         = flag.String("config", "", "Path to config file (optional)")
	serverURL          = flag.String("server", "", "Server URL")
	username           = flag.String("username", "", "Username")
	theme              = flag.String("theme", "", "Theme")
	isAdmin            = flag.Bool("admin", false, "Connect as admin (requires --admin-key)")
	adminKey           = flag.String("admin-key", "", "Admin key for privileged commands")
	useE2E             = flag.Bool("e2e", false, "Enable end-to-end encryption")
	keystorePassphrase = flag.String("keystore-passphrase", "", "Passphrase for keystore (required for E2E)")
	skipTLSVerify      = flag.Bool("skip-tls-verify", false, "Skip TLS certificate verification")
	quickStart         = flag.Bool("quick-start", false, "Use last connection or select from saved profiles")
	autoConnect        = flag.Bool("auto", false, "Automatically connect using most recent profile")
	nonInteractive     = flag.Bool("non-interactive", false, "Skip interactive prompts (require all flags)")
)

// Add these helper functions after the existing imports and before the model struct

// debugEncryptAndSend provides comprehensive logging around encryption
func debugEncryptAndSend(recipients []string, plaintext string, ws *websocket.Conn, keystore *crypto.KeyStore, username string) error {
	log.Printf("DEBUG: Starting global encryption for %d recipients", len(recipients))
	log.Printf("DEBUG: Plaintext length: %d", len(plaintext))

	// Check keystore status
	if keystore == nil {
		log.Printf("ERROR: Keystore is nil")
		return fmt.Errorf("keystore not initialized")
	}
	log.Printf("DEBUG: Keystore loaded: %t", keystore != nil)

	// Verify global key is available
	globalKey := keystore.GetSessionKey("global")
	if globalKey == nil {
		log.Printf("ERROR: Global key not found")
		return fmt.Errorf("global key not available - global E2E encryption not initialized")
	}
	log.Printf("DEBUG: Global key available (ID: %s)", globalKey.KeyID)

	// Perform encryption using global key
	conversationID := "global"
	encryptedMsg, err := keystore.EncryptMessage(username, plaintext, conversationID)
	if err != nil {
		log.Printf("ERROR: Global encryption failed: %v", err)
		return fmt.Errorf("global encryption failed: %v", err)
	}

	log.Printf("DEBUG: Global encryption successful - encrypted length: %d", len(encryptedMsg.Encrypted))

	// Guard against empty ciphertext
	if len(encryptedMsg.Encrypted) == 0 {
		log.Printf("ERROR: Encryption returned empty ciphertext")
		return fmt.Errorf("encryption returned empty ciphertext; aborting send")
	}

	// Combine nonce + encrypted data and base64 encode for safe JSON transport
	combinedData := make([]byte, 0, len(encryptedMsg.Nonce)+len(encryptedMsg.Encrypted))
	combinedData = append(combinedData, encryptedMsg.Nonce...)
	combinedData = append(combinedData, encryptedMsg.Encrypted...)

	finalContent := base64.StdEncoding.EncodeToString(combinedData)
	log.Printf("DEBUG: Base64 encoded nonce+ciphertext - length: %d", len(finalContent))

	// Verify final content is not empty
	if len(finalContent) == 0 {
		log.Printf("ERROR: Final content is empty after encoding")
		return fmt.Errorf("final content is empty after encoding")
	}

	// Create a regular Message struct for the server
	msg := shared.Message{
		Content:   finalContent,
		Sender:    username,
		CreatedAt: time.Now(),
		Type:      shared.TextMessage,
		Encrypted: true, // Mark as encrypted
	}

	log.Printf("DEBUG: Final message - Content length: %d, Type: %s",
		len(msg.Content), msg.Type)

	// Send message
	if err := ws.WriteJSON(msg); err != nil {
		log.Printf("ERROR: WebSocket write failed: %v", err)
		return err
	}

	log.Printf("DEBUG: Global encrypted message sent successfully")
	return nil
}

// validateEncryptionRoundtrip tests encryption primitives using global key
func validateEncryptionRoundtrip(keystore *crypto.KeyStore, username string) error {
	testPlaintext := "Hello, global encryption test!"

	log.Printf("DEBUG: Testing global encryption roundtrip")

	// Test global conversation encryption
	conversationID := "global"

	// Verify global key exists
	globalKey := keystore.GetSessionKey(conversationID)
	if globalKey == nil {
		return fmt.Errorf("global key not found - global E2E encryption not available")
	}

	log.Printf("DEBUG: Global key found (ID: %s)", globalKey.KeyID)

	// Test encryption using global key
	encryptedMsg, err := keystore.EncryptMessage(username, testPlaintext, conversationID)
	if err != nil {
		return fmt.Errorf("global encryption test failed: %v", err)
	}

	if len(encryptedMsg.Encrypted) == 0 {
		return fmt.Errorf("global encryption test produced empty ciphertext")
	}

	log.Printf("DEBUG: Global encryption test successful - ciphertext length: %d", len(encryptedMsg.Encrypted))

	// Test decryption roundtrip
	decryptedMsg, err := keystore.DecryptMessage(encryptedMsg, conversationID)
	if err != nil {
		return fmt.Errorf("global decryption test failed: %v", err)
	}

	if decryptedMsg.Content != testPlaintext {
		return fmt.Errorf("global decryption roundtrip failed: expected '%s', got '%s'", testPlaintext, decryptedMsg.Content)
	}

	log.Printf("DEBUG: Global encryption roundtrip test successful")
	return nil
}

// verifyKeystoreUnlocked verifies keystore is properly unlocked (global encryption only)
func verifyKeystoreUnlocked(keystore *crypto.KeyStore) error {
	if keystore == nil {
		return fmt.Errorf("keystore is nil")
	}

	// Check if global key is available
	globalKey := keystore.GetGlobalKey()
	if globalKey == nil {
		return fmt.Errorf("global key not available")
	}

	log.Printf("DEBUG: Keystore properly unlocked for global encryption")
	return nil
}

// debugWebSocketWrite logs what's being sent over the wire
func debugWebSocketWrite(ws *websocket.Conn, msg interface{}) error {
	// Marshal to JSON to see what's being sent
	jsonData, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ERROR: JSON marshal failed: %v", err)
		return err
	}

	// Log without exposing sensitive content
	log.Printf("DEBUG: Sending WebSocket message - length: %d bytes", len(jsonData))

	// Check if content field is present and non-empty
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err == nil {
		if content, exists := parsed["content"]; exists {
			if contentStr, ok := content.(string); ok {
				log.Printf("DEBUG: Message content length: %d", len(contentStr))
				if len(contentStr) == 0 {
					log.Printf("WARNING: Sending message with empty content!")
				}
			}
		}
	}

	return ws.WriteJSON(msg)
}

type model struct {
	cfg            config.Config
	configFilePath string // Store the config file path for saving
	textarea       textarea.Model
	viewport       viewport.Model
	messages       []shared.Message
	styles         themeStyles
	banner         string
	connected      bool

	users []string // NEW: user list

	width  int // NEW: track window width
	height int // NEW: track window height

	userListViewport viewport.Model // NEW: scrollable user list

	twentyFourHour bool // NEW: timestamp format toggle

	sending bool // NEW: sending message feedback

	conn    *websocket.Conn // persistent WebSocket connection
	msgChan chan tea.Msg    // channel for incoming messages from WS goroutine
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	reconnectDelay time.Duration               // for exponential backoff
	receivedFiles  map[string]*shared.FileMeta // filename -> filemeta for saving

	// E2E Encryption
	keystore *crypto.KeyStore
	useE2E   bool // Flag to enable/disable E2E encryption

	// Help system
	keys     keyMap
	help     help.Model
	showHelp bool
}

type themeStyles struct {
	User    lipgloss.Style
	Time    lipgloss.Style
	Msg     lipgloss.Style
	Banner  lipgloss.Style
	Box     lipgloss.Style // frame color
	Mention lipgloss.Style // mention highlighting

	UserList lipgloss.Style // NEW: user list panel
	Me       lipgloss.Style // NEW: current user style
	Other    lipgloss.Style // NEW: other user style

	Background lipgloss.Style // NEW: main background
	Header     lipgloss.Style // NEW: header background
	Footer     lipgloss.Style // NEW: footer background
	Input      lipgloss.Style // NEW: input background

	// Help styles
	HelpOverlay lipgloss.Style
	HelpTitle   lipgloss.Style
}

// Base theme style helper
func baseThemeStyles() themeStyles {
	return themeStyles{
		User:       lipgloss.NewStyle().Bold(true),
		Time:       lipgloss.NewStyle().Faint(true),
		Msg:        lipgloss.NewStyle(),
		Banner:     lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F")).Bold(true),
		Box:        lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#AAAAAA")),
		Mention:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")),
		UserList:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#AAAAAA")).Padding(0, 1),
		Me:         lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")).Bold(true),
		Other:      lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")),
		Background: lipgloss.NewStyle(),
		Header:     lipgloss.NewStyle(),
		Footer:     lipgloss.NewStyle(),
		Input:      lipgloss.NewStyle(),
		HelpOverlay: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#1a1a1a")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(1, 2),
		HelpTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Bold(true).
			MarginBottom(1),
	}
}

func getThemeStyles(theme string) themeStyles {
	s := baseThemeStyles()
	switch strings.ToLower(theme) {
	case "system":
		// System theme uses minimal styling to respect terminal defaults
		s.User = lipgloss.NewStyle().Bold(true)
		s.Time = lipgloss.NewStyle().Faint(true)
		s.Msg = lipgloss.NewStyle()
		s.Banner = lipgloss.NewStyle().Bold(true)
		s.Box = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
		s.Mention = lipgloss.NewStyle().Bold(true)
		s.UserList = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
		s.Me = lipgloss.NewStyle().Bold(true)
		s.Other = lipgloss.NewStyle()
		// Background and UI elements use no colors to respect terminal theme
		s.Background = lipgloss.NewStyle()
		s.Header = lipgloss.NewStyle().Bold(true)
		s.Footer = lipgloss.NewStyle()
		s.Input = lipgloss.NewStyle()
		s.HelpOverlay = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	case "patriot":
		s.User = s.User.Foreground(lipgloss.Color("#002868"))              // Navy blue
		s.Time = s.Time.Foreground(lipgloss.Color("#BF0A30")).Faint(false) // Red
		s.Msg = s.Msg.Foreground(lipgloss.Color("#FFFFFF"))
		s.Box = s.Box.BorderForeground(lipgloss.Color("#BF0A30"))
		s.Mention = s.Mention.Foreground(lipgloss.Color("#FFD700")) // Gold
		s.UserList = s.UserList.BorderForeground(lipgloss.Color("#002868"))
		s.Me = s.Me.Foreground(lipgloss.Color("#BF0A30"))
		// Background and UI
		s.Background = lipgloss.NewStyle().Background(lipgloss.Color("#00203F")) // Deep navy
		s.Header = lipgloss.NewStyle().Background(lipgloss.Color("#BF0A30")).Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
		s.Footer = lipgloss.NewStyle().Background(lipgloss.Color("#00203F")).Foreground(lipgloss.Color("#FFD700"))
		s.Input = lipgloss.NewStyle().Background(lipgloss.Color("#002868")).Foreground(lipgloss.Color("#FFFFFF"))
		s.HelpOverlay = s.HelpOverlay.BorderForeground(lipgloss.Color("#BF0A30")).Background(lipgloss.Color("#00203F"))
	case "retro":
		s.User = s.User.Foreground(lipgloss.Color("#FF8800"))              // Orange
		s.Time = s.Time.Foreground(lipgloss.Color("#00FF00")).Faint(false) // Green
		s.Msg = s.Msg.Foreground(lipgloss.Color("#FFFFAA"))
		s.Box = s.Box.BorderForeground(lipgloss.Color("#FF8800"))
		s.Mention = s.Mention.Foreground(lipgloss.Color("#00FFFF")) // Cyan
		s.UserList = s.UserList.BorderForeground(lipgloss.Color("#FF8800"))
		s.Me = s.Me.Foreground(lipgloss.Color("#FF8800"))
		// Background and UI
		s.Background = lipgloss.NewStyle().Background(lipgloss.Color("#181818")) // Retro dark
		s.Header = lipgloss.NewStyle().Background(lipgloss.Color("#FF8800")).Foreground(lipgloss.Color("#181818")).Bold(true)
		s.Footer = lipgloss.NewStyle().Background(lipgloss.Color("#181818")).Foreground(lipgloss.Color("#00FF00"))
		s.Input = lipgloss.NewStyle().Background(lipgloss.Color("#222200")).Foreground(lipgloss.Color("#FFFFAA"))
		s.HelpOverlay = s.HelpOverlay.BorderForeground(lipgloss.Color("#FF8800")).Background(lipgloss.Color("#181818"))
	case "modern":
		s.User = s.User.Foreground(lipgloss.Color("#4F8EF7"))              // Blue
		s.Time = s.Time.Foreground(lipgloss.Color("#A0A0A0")).Faint(false) // Gray
		s.Msg = s.Msg.Foreground(lipgloss.Color("#E0E0E0"))
		s.Box = s.Box.BorderForeground(lipgloss.Color("#4F8EF7"))
		s.Mention = s.Mention.Foreground(lipgloss.Color("#FF5F5F")) // Red
		s.UserList = s.UserList.BorderForeground(lipgloss.Color("#4F8EF7"))
		s.Me = s.Me.Foreground(lipgloss.Color("#4F8EF7"))
		// Background and UI
		s.Background = lipgloss.NewStyle().Background(lipgloss.Color("#181C24")) // Modern dark blue-gray
		s.Header = lipgloss.NewStyle().Background(lipgloss.Color("#4F8EF7")).Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
		s.Footer = lipgloss.NewStyle().Background(lipgloss.Color("#181C24")).Foreground(lipgloss.Color("#4F8EF7"))
		s.Input = lipgloss.NewStyle().Background(lipgloss.Color("#23272E")).Foreground(lipgloss.Color("#E0E0E0"))
		s.HelpOverlay = s.HelpOverlay.BorderForeground(lipgloss.Color("#4F8EF7")).Background(lipgloss.Color("#181C24"))
	}
	return s
}

func renderMessages(msgs []shared.Message, styles themeStyles, username string, users []string, width int, twentyFourHour bool) string {
	const max = maxMessages
	if len(msgs) > max {
		msgs = msgs[len(msgs)-max:]
	}

	// CRITICAL FIX: Sort messages client-side to ensure consistent ordering
	// This handles cases where server-side ordering may be inconsistent
	sortMessagesByTimestamp(msgs)

	var b strings.Builder
	var prevDate string
	for _, msg := range msgs {
		sender := msg.Sender
		align := lipgloss.Left
		msgBoxStyle := lipgloss.NewStyle().Width(width - 4)
		if sender == username {
			align = lipgloss.Right
			msgBoxStyle = msgBoxStyle.Background(lipgloss.Color("#222244")).Foreground(lipgloss.Color("#FFFFFF"))
		} else {
			msgBoxStyle = msgBoxStyle.Background(lipgloss.Color("#222222")).Foreground(lipgloss.Color("#AAAAAA"))
		}
		// Date header if date changes
		dateStr := msg.CreatedAt.Format("2006-01-02")
		if dateStr != prevDate {
			b.WriteString(styles.Time.Render(dateStr) + "\n")
			prevDate = dateStr
		}
		// Time format
		timeFmt := "15:04:05"
		if !twentyFourHour {
			timeFmt = "03:04:05 PM"
		}
		timestamp := styles.Time.Render(msg.CreatedAt.Format(timeFmt))
		var content string
		if msg.Type == shared.FileMessageType && msg.File != nil {
			fileInfo := styles.Mention.Render("[File] ") + styles.User.Render(msg.File.Filename) + styles.Time.Render(fmt.Sprintf(" (%d bytes)", msg.File.Size))
			content = fileInfo + "\n" + styles.Msg.Render("Type :savefile "+msg.File.Filename+" to save.")
		} else {
			content = renderEmojis(msg.Content)
			// Improved mention highlighting: highlight if any @username in user list (case-insensitive)
			matches := mentionRegex.FindAllStringSubmatch(msg.Content, -1)
			highlight := false
			for _, m := range matches {
				if len(m) > 1 {
					for _, u := range users {
						if strings.EqualFold(m[1], u) {
							highlight = true
							break
						}
					}
					if highlight {
						break
					}
				}
			}
			if highlight {
				content = styles.Mention.Render(content)
			} else {
				content = styles.Msg.Render(content)
			}
		}
		meta := styles.User.Render(sender) + " " + timestamp
		wrapped := msgBoxStyle.Render(content)
		msgBlock := lipgloss.JoinVertical(lipgloss.Left, meta, wrapped)
		b.WriteString(msgBoxStyle.Align(align).Render(msgBlock) + "\n\n")
	}
	return b.String()
}

type wsMsg struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type wsErr error

type wsConnected bool

type UserList struct {
	Users []string `json:"users"`
}

func (m *model) connectWebSocket(serverURL string) error {
	escapedUsername := url.QueryEscape(m.cfg.Username)
	fullURL := serverURL + "?username=" + escapedUsername

	log.Printf("Attempting to connect to: %s", fullURL)
	log.Printf("Username: %s, Admin: %v", m.cfg.Username, *isAdmin)
	if *isAdmin {
		log.Printf("Admin key: %s", *adminKey)
	}

	// Create custom dialer with TLS configuration
	dialer := websocket.DefaultDialer
	if *skipTLSVerify {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	conn, resp, err := dialer.Dial(fullURL, nil)
	if err != nil {
		if resp != nil {
			log.Printf("WebSocket connection failed with status %d: %v", resp.StatusCode, err)
		} else {
			log.Printf("WebSocket connection failed: %v", err)
		}
		return err
	}

	log.Printf("WebSocket connection established successfully")
	m.conn = conn
	m.connected = true
	m.banner = "✅ Connected to server!"
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.wg.Add(1)

	// Send handshake as first message
	handshake := shared.Handshake{
		Username: m.cfg.Username,
		Admin:    *isAdmin,
		AdminKey: "",
	}
	if *isAdmin {
		handshake.AdminKey = *adminKey
	}

	log.Printf("Sending handshake: %+v", handshake)
	if err := m.conn.WriteJSON(handshake); err != nil {
		log.Printf("Failed to send handshake: %v", err)
		return err
	}
	log.Printf("Handshake sent successfully")

	// Set pong handler
	m.conn.SetPongHandler(func(appData string) error {
		return nil
	})

	// Start ping goroutine
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				_ = m.conn.WriteMessage(websocket.PingMessage, nil)
			}
		}
	}()

	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-m.ctx.Done():
				return
			default:
				_, raw, err := conn.ReadMessage()
				if err != nil {
					log.Printf("WebSocket read error: %v", err)
					m.msgChan <- wsErr(err)
					return
				}

				log.Printf("Received message: %s", string(raw))

				// Try to unmarshal as shared.Message first
				var msg shared.Message
				if err := json.Unmarshal(raw, &msg); err == nil {
					if msg.Sender != "" {
						// Check if this is an encrypted message
						if m.useE2E && msg.Encrypted && msg.Content != "" {
							// Try to decode base64 encrypted content (nonce + encrypted_data)
							if decoded, err := base64.StdEncoding.DecodeString(msg.Content); err == nil && len(decoded) > 12 {
								// This might be encrypted content, try to decrypt it
								log.Printf("DEBUG: Detected potential encrypted content, attempting decryption")

								// Extract nonce (first 12 bytes) and encrypted data (rest)
								nonce := decoded[:12]
								encryptedData := decoded[12:]

								// Create an EncryptedMessage struct for decryption
								encryptedMsg := shared.EncryptedMessage{
									Sender:      msg.Sender,
									CreatedAt:   msg.CreatedAt,
									Encrypted:   encryptedData,
									Nonce:       nonce,
									IsEncrypted: true,
									Type:        msg.Type,
								}

								conversationID := "global" // Same as sending
								decryptedMsg, err := m.keystore.DecryptMessage(&encryptedMsg, conversationID)
								if err != nil {
									log.Printf("DEBUG: Failed to decrypt message: %v", err)
									// Keep original message but mark as failed decryption
									msg.Content = "[ENCRYPTED - DECRYPTION FAILED]"
									m.msgChan <- msg
									continue
								}

								log.Printf("DEBUG: Successfully decrypted message")
								m.msgChan <- *decryptedMsg
								continue
							}
						}

						// Regular message (not encrypted or decryption not needed)
						m.msgChan <- msg
						continue
					}
				}

				// Then try as wsMsg
				var ws wsMsg
				if err := json.Unmarshal(raw, &ws); err == nil && ws.Type != "" {
					log.Printf("Received wsMsg type: %s", ws.Type)
					m.msgChan <- ws
					continue
				}

				log.Printf("Could not parse message: %s", string(raw))
			}
		}
	}()
	return nil
}

func (m *model) closeWebSocket() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.conn != nil {
		m.conn.Close()
	}
	m.wg.Wait()
}

func (m *model) Init() tea.Cmd {
	m.msgChan = make(chan tea.Msg, 10) // buffered to avoid blocking
	m.reconnectDelay = time.Second     // reset on each Init
	return func() tea.Msg {
		err := m.connectWebSocket(m.cfg.ServerURL)
		if err != nil {
			return wsErr(err)
		}
		return wsConnected(true)
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case wsConnected:
		m.connected = true
		m.banner = "✅ Connected to server!"
		m.reconnectDelay = time.Second // reset on success
		return m, m.listenWebSocket()
	case wsMsg:
		if v.Type == "userlist" {
			var ul UserList
			if err := json.Unmarshal(v.Data, &ul); err == nil {
				m.users = ul.Users
				userListWidth := 18
				m.userListViewport.SetContent(renderUserList(m.users, m.cfg.Username, m.styles, userListWidth))
			}
			return m, m.listenWebSocket()
		}
		if v.Type == "auth_failed" {
			log.Printf("Authentication failed - admin key rejected")
			var authFail map[string]string
			if err := json.Unmarshal(v.Data, &authFail); err == nil {
				log.Printf("Auth failure reason: %s", authFail["reason"])
			}
			fmt.Printf("❌ Authentication failed: %s\n", authFail["reason"])
			fmt.Printf("Check your --admin-key matches the server's MARCHAT_ADMIN_KEY\n")
			os.Exit(1)
		}
		return m, m.listenWebSocket()
	case shared.Message:
		if len(m.messages) >= maxMessages {
			m.messages = m.messages[len(m.messages)-maxMessages+1:]
		}
		m.messages = append(m.messages, v)

		// CRITICAL FIX: Sort messages after adding new ones to maintain order
		sortMessagesByTimestamp(m.messages)

		if v.Type == shared.FileMessageType && v.File != nil {
			if m.receivedFiles == nil {
				m.receivedFiles = make(map[string]*shared.FileMeta)
			}
			m.receivedFiles[v.File.Filename] = v.File
		}
		m.viewport.SetContent(renderMessages(m.messages, m.styles, m.cfg.Username, m.users, m.viewport.Width, m.twentyFourHour))
		m.viewport.GotoBottom()
		m.sending = false
		return m, m.listenWebSocket()
	case wsErr:
		m.connected = false
		m.banner = "🚫 Connection lost. Reconnecting..."
		m.closeWebSocket()
		delay := m.reconnectDelay
		if delay < reconnectMaxDelay {
			m.reconnectDelay *= 2
			if m.reconnectDelay > reconnectMaxDelay {
				m.reconnectDelay = reconnectMaxDelay
			}
		}
		return m, tea.Tick(delay, func(time.Time) tea.Msg {
			return m.Init()()
		})
	case tea.KeyMsg:
		switch {
		case key.Matches(v, m.keys.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(v, m.keys.Quit):
			m.closeWebSocket()
			return m, tea.Quit
		case key.Matches(v, m.keys.ScrollUp):
			if m.textarea.Focused() {
				m.viewport.ScrollUp(1)
			} else {
				m.userListViewport.ScrollUp(1)
			}
			return m, nil
		case key.Matches(v, m.keys.ScrollDown):
			if m.textarea.Focused() {
				m.viewport.ScrollDown(1)
			} else {
				m.userListViewport.ScrollDown(1)
			}
			return m, nil
		case key.Matches(v, m.keys.PageUp):
			m.viewport.ScrollUp(m.viewport.Height)
			return m, nil
		case key.Matches(v, m.keys.PageDown):
			m.viewport.ScrollDown(m.viewport.Height)
			return m, nil
		case key.Matches(v, m.keys.Copy): // Custom Copy
			if m.textarea.Focused() {
				text := m.textarea.Value()
				if text != "" {
					if err := clipboard.WriteAll(text); err != nil {
						m.banner = "❌ Failed to copy to clipboard: " + err.Error()
					} else {
						m.banner = "✅ Copied to clipboard"
					}
				}
				return m, nil
			}
			return m, nil
		case key.Matches(v, m.keys.Paste): // Custom Paste
			if m.textarea.Focused() {
				text, err := clipboard.ReadAll()
				if err != nil {
					m.banner = "❌ Failed to paste from clipboard: " + err.Error()
				} else {
					m.textarea.SetValue(m.textarea.Value() + text)
				}
				return m, nil
			}
			return m, nil
		case key.Matches(v, m.keys.Cut): // Custom Cut
			if m.textarea.Focused() {
				text := m.textarea.Value()
				if text != "" {
					if err := clipboard.WriteAll(text); err != nil {
						m.banner = "❌ Failed to cut to clipboard: " + err.Error()
					} else {
						m.textarea.SetValue("")
						m.banner = "✅ Cut to clipboard"
					}
				}
				return m, nil
			}
			return m, nil
		case key.Matches(v, m.keys.SelectAll): // Custom Select All
			if m.textarea.Focused() {
				text := m.textarea.Value()
				if text != "" {
					if err := clipboard.WriteAll(text); err != nil {
						m.banner = "❌ Failed to select all: " + err.Error()
					} else {
						m.banner = "✅ Selected all and copied to clipboard"
					}
				}
				return m, nil
			}
			return m, nil
		case key.Matches(v, m.keys.Send):
			text := m.textarea.Value()
			if strings.HasPrefix(text, ":sendfile ") {
				parts := strings.SplitN(text, " ", 2)
				if len(parts) == 2 {
					path := strings.TrimSpace(parts[1])
					if path != "" {
						data, err := os.ReadFile(path)
						if err != nil {
							m.banner = "❌ Failed to read file: " + err.Error()
							m.textarea.SetValue("")
							return m, nil
						}
						if len(data) > 1024*1024 {
							m.banner = "❌ File too large (max 1MB)"
							m.textarea.SetValue("")
							return m, nil
						}
						filename := filepath.Base(path)
						msg := shared.Message{
							Sender:    m.cfg.Username,
							Type:      shared.FileMessageType,
							CreatedAt: time.Now(),
							File: &shared.FileMeta{
								Filename: filename,
								Size:     int64(len(data)),
								Data:     data,
							},
						}
						if m.conn != nil {
							err := m.conn.WriteJSON(msg)
							if err != nil {
								m.banner = "❌ Failed to send file (connection lost)"
								m.textarea.SetValue("")
								return m, m.listenWebSocket()
							}
							m.banner = "File sent: " + filename
						}
						m.textarea.SetValue("")
						return m, m.listenWebSocket()
					}
				}
			}
			if strings.HasPrefix(text, ":savefile ") {
				filename := strings.TrimSpace(strings.TrimPrefix(text, ":savefile "))
				if m.receivedFiles == nil || m.receivedFiles[filename] == nil {
					m.banner = "❌ No files received yet."
					m.textarea.SetValue("")
					return m, nil
				}
				file := m.receivedFiles[filename]
				// Check for duplicate filenames and append suffix if needed
				saveName := file.Filename
				base := saveName
				ext := ""
				if dot := strings.LastIndex(saveName, "."); dot != -1 {
					base = saveName[:dot]
					ext = saveName[dot:]
				}
				tryName := saveName
				for i := 1; ; i++ {
					if _, err := os.Stat(tryName); os.IsNotExist(err) {
						saveName = tryName
						break
					}
					tryName = fmt.Sprintf("%s[%d]%s", base, i, ext)
				}
				err := os.WriteFile(saveName, file.Data, 0644)
				if err != nil {
					m.banner = "❌ Failed to save file: " + err.Error()
				} else {
					m.banner = "✅ File saved as: " + saveName
				}
				m.textarea.SetValue("")
				return m, nil
			}
			if strings.HasPrefix(text, ":theme ") {
				parts := strings.SplitN(text, " ", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
					m.cfg.Theme = strings.TrimSpace(parts[1])
					m.styles = getThemeStyles(m.cfg.Theme)
					_ = config.SaveConfig(m.configFilePath, m.cfg)
					m.banner = "Theme changed to " + m.cfg.Theme
				} else {
					m.banner = "Please provide a theme name."
				}
				m.textarea.SetValue("")
				return m, nil
			}
			if text == ":clear" {
				m.messages = nil
				m.viewport.SetContent("")
				m.banner = "Chat cleared."
				m.textarea.SetValue("")
				return m, nil
			}
			if text == ":cleardb" {
				if !*isAdmin {
					m.banner = "You are not authenticated as admin."
					m.textarea.SetValue("")
					return m, nil
				}
				m.sending = true
				if m.conn != nil {
					msg := shared.Message{Sender: m.cfg.Username, Content: text}
					err := m.conn.WriteJSON(msg)
					if err != nil {
						m.banner = "❌ Failed to send (connection lost)"
						m.sending = false
						return m, m.listenWebSocket()
					}
					m.banner = ""
				}
				m.textarea.SetValue("")
				return m, m.listenWebSocket()
			}

			// Individual E2E encryption commands removed - only global E2E encryption supported
			if text == ":time" {
				m.twentyFourHour = !m.twentyFourHour
				m.cfg.TwentyFourHour = m.twentyFourHour
				_ = config.SaveConfig(*configPath, m.cfg)
				m.banner = "Timestamp format: " + map[bool]string{true: "24h", false: "12h"}[m.twentyFourHour]
				m.viewport.SetContent(renderMessages(m.messages, m.styles, m.cfg.Username, m.users, m.viewport.Width, m.twentyFourHour))
				m.viewport.GotoBottom()
				m.textarea.SetValue("")
				return m, nil
			}
			if text != "" {
				m.sending = true
				if m.conn != nil {
					if m.useE2E {
						// Use E2E encryption for global chat
						log.Printf("DEBUG: Attempting to send global encrypted message: '%s'", text)

						// Validate keystore is unlocked
						if err := verifyKeystoreUnlocked(m.keystore); err != nil {
							m.banner = fmt.Sprintf("❌ Keystore not unlocked: %v", err)
							m.sending = false
							m.textarea.SetValue("")
							return m, nil
						}

						// For global chat, we don't need individual recipient keys
						// All users in the chat will receive the message encrypted with the global key
						recipients := m.users
						if len(recipients) == 0 {
							recipients = []string{m.cfg.Username} // Fallback to self
						}

						// Use the debug encryption function for global chat
						if err := debugEncryptAndSend(recipients, text, m.conn, m.keystore, m.cfg.Username); err != nil {
							m.banner = fmt.Sprintf("❌ Global encryption failed: %v", err)
							m.sending = false
							m.textarea.SetValue("")
							return m, nil
						}

						log.Printf("DEBUG: Global encrypted message sent successfully")
					} else {
						// Send plain text message
						msg := shared.Message{Sender: m.cfg.Username, Content: text}
						if err := debugWebSocketWrite(m.conn, msg); err != nil {
							m.banner = "❌ Failed to send (connection lost)"
							m.sending = false
							return m, m.listenWebSocket()
						}
					}
					m.banner = ""
				}
				m.textarea.SetValue("")
				return m, m.listenWebSocket()
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(v)
			return m, cmd
		}
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
		m.help.Width = v.Width
		chatWidth := m.width - userListWidth - 4
		if chatWidth < 20 {
			chatWidth = 20
		}
		m.viewport.Width = chatWidth
		m.viewport.Height = m.height - m.textarea.Height() - 6
		m.textarea.SetWidth(chatWidth)
		m.userListViewport.Width = userListWidth
		m.userListViewport.Height = m.height - m.textarea.Height() - 6
		m.viewport.SetContent(renderMessages(m.messages, m.styles, m.cfg.Username, m.users, m.viewport.Width, m.twentyFourHour))
		m.viewport.GotoBottom()
		m.userListViewport.SetContent(renderUserList(m.users, m.cfg.Username, m.styles, userListWidth))
		return m, nil
	case quitMsg:
		return m, tea.Quit
	case tea.MouseMsg:
		if (v.Button == tea.MouseButtonWheelUp || v.Button == tea.MouseButtonWheelDown) && v.Action == tea.MouseActionPress {
			return m, nil // Ignore mouse scroll
		}
		return m, nil // Return for other mouse events
	default:
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(v)
		return m, cmd
	}
}

func (m *model) listenWebSocket() tea.Cmd {
	return func() tea.Msg {
		return <-m.msgChan
	}
}

func (m *model) renderHelpOverlay() string {
	if !m.showHelp {
		return ""
	}

	title := m.styles.HelpTitle.Render("marchat help")

	// Basic keybindings
	basicHelp := "Keyboard Shortcuts:\n"
	basicHelp += "  Ctrl+H               Toggle help\n"
	basicHelp += "  Esc                  Quit application\n"
	basicHelp += "  Enter                Send message\n"
	basicHelp += "  ↑/↓                  Scroll chat history\n"
	basicHelp += "  PgUp/PgDn            Page through chat\n"
	basicHelp += "  Ctrl+C/V/X/A         Copy/Paste/Cut/Select all"

	// Command help
	var commandHelp string
	commandHelp = "\nCommands:\n"

	// Basic commands
	commandHelp += "  :sendfile <path>      Send a file\n"
	commandHelp += "  :savefile <filename>  Save received file\n"
	commandHelp += "  :theme <name>         Change theme (system, patriot, retro, modern)\n"
	commandHelp += "  :time                 Toggle 12/24h time format\n"
	commandHelp += "  :clear                Clear chat history\n"

	// Admin commands (only show if admin)
	if *isAdmin {
		commandHelp += "\nAdmin Commands:\n"
		commandHelp += "  :cleardb              Clear server database\n"
		commandHelp += "  :kick <user>          Kick user\n"
		commandHelp += "  :ban <user>           Ban user\n"
		commandHelp += "  :unban <user>         Unban user\n"
	}

	content := title + "\n\n" + basicHelp + commandHelp

	// Center the help overlay
	helpWidth := 70
	helpHeight := strings.Count(content, "\n") + 4

	overlay := m.styles.HelpOverlay.
		Width(helpWidth).
		Height(helpHeight).
		Render(content)

	// Position the overlay (simplified - just return the styled content)
	return overlay
}

func (m *model) View() string {
	// Header with version
	headerText := fmt.Sprintf(" marchat %s ", shared.ClientVersion)
	header := m.styles.Header.Width(m.viewport.Width + userListWidth + 4).Render(headerText)

	// Simplified footer - just basic info
	footerText := "Press Ctrl+H for help"
	if m.showHelp {
		footerText = "Press Ctrl+H to close help"
	}
	footer := m.styles.Footer.Width(m.viewport.Width + userListWidth + 4).Render(footerText)

	// Banner
	var bannerBox string
	if m.banner != "" || m.sending {
		bannerText := m.banner
		if m.sending {
			if bannerText != "" {
				bannerText += " ⏳ Sending..."
			} else {
				bannerText = "⏳ Sending..."
			}
		}
		bannerBox = m.styles.Banner.
			Width(m.viewport.Width).
			PaddingLeft(1).
			Background(lipgloss.Color("#FF5F5F")).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Render(bannerText)
	}

	// Chat and user list layout
	chatBoxStyle := m.styles.Box
	chatPanel := chatBoxStyle.Width(m.viewport.Width).Render(m.viewport.View())
	userPanel := m.userListViewport.View()
	row := lipgloss.JoinHorizontal(lipgloss.Top, userPanel, chatPanel)

	// Input
	inputPanel := m.styles.Input.Width(m.viewport.Width).Render(m.textarea.View())

	// Compose layout
	ui := lipgloss.JoinVertical(lipgloss.Left,
		header,
		bannerBox,
		row,
		inputPanel,
		footer,
	)

	// Overlay help if shown
	if m.showHelp {
		// Create a background for the help overlay
		helpOverlay := m.renderHelpOverlay()

		// Calculate overlay position (centered)
		overlayLines := strings.Split(helpOverlay, "\n")
		overlayHeight := len(overlayLines)
		overlayWidth := 0
		for _, line := range overlayLines {
			if len(line) > overlayWidth {
				overlayWidth = len(line)
			}
		}

		// Position overlay in center of screen
		mainLines := strings.Split(ui, "\n")
		if len(mainLines) > overlayHeight+4 {
			startLine := (len(mainLines) - overlayHeight) / 2

			// Insert overlay into main UI
			for i, overlayLine := range overlayLines {
				if startLine+i < len(mainLines) {
					// Center the overlay line
					padding := (m.width - overlayWidth) / 2
					if padding < 0 {
						padding = 0
					}
					mainLines[startLine+i] = strings.Repeat(" ", padding) + overlayLine
				}
			}
			ui = strings.Join(mainLines, "\n")
		} else {
			// If screen is too small, just show help overlay
			ui = helpOverlay
		}
	}

	return m.styles.Background.Render(ui)
}

func renderEmojis(s string) string {
	emojis := map[string]string{
		":)": "😊",
		":(": "🙁",
		":D": "😃",
		"<3": "❤️",
		":P": "😛",
	}
	for k, v := range emojis {
		s = strings.ReplaceAll(s, k, v)
	}
	return s
}

func renderUserList(users []string, me string, styles themeStyles, width int) string {
	var b strings.Builder
	b.WriteString(styles.UserList.Width(width).Render(" Users ") + "\n")
	max := maxUsersDisplay
	for i, u := range users {
		if i >= max {
			b.WriteString(lipgloss.NewStyle().Italic(true).Faint(true).Width(width).Render(fmt.Sprintf("+%d more", len(users)-max)) + "\n")
			break
		}
		if u == me {
			b.WriteString(styles.Me.Render("• "+u) + "\n")
		} else {
			b.WriteString(styles.Other.Render("• "+u) + "\n")
		}
	}
	return b.String()
}

// Add a custom quitMsg type
type quitMsg struct{}

func main() {
	flag.Parse()

	// Auto-connect to most recent profile
	if *autoConnect {
		loader, err := config.NewInteractiveConfigLoader()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		cfg, err := loader.AutoConnect()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// Get sensitive data and connect
		adminKey, keystorePass, err := loader.PromptSensitiveData(cfg.IsAdmin, cfg.UseE2E)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		initializeClient(cfg, adminKey, keystorePass)
		return
	}

	// Quick start menu - actually connects using saved profiles
	if *quickStart {
		loader, err := config.NewInteractiveConfigLoader()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		cfg, err := loader.QuickStartConnect()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// Get sensitive data and connect
		adminKey, keystorePass, err := loader.PromptSensitiveData(cfg.IsAdmin, cfg.UseE2E)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		initializeClient(cfg, adminKey, keystorePass)
		return
	}

	var cfg *config.Config
	var err error

	// Check if all required flags are provided for non-interactive mode
	if *nonInteractive || (allFlagsProvided(*serverURL, *username, *isAdmin, *adminKey, *useE2E, *keystorePassphrase)) {
		// Use traditional flag-based configuration
		cfg, err = loadConfigFromFlags(*configPath, *serverURL, *username, *theme, *isAdmin, *useE2E, *skipTLSVerify)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Validate required flags for non-interactive mode
		if err := validateFlags(*isAdmin, *adminKey, *useE2E, *keystorePassphrase); err != nil {
			fmt.Printf("Error: %v\n", err)
			flag.Usage()
			os.Exit(1)
		}

		// Continue with existing client initialization using flag values
		initializeClient(cfg, *adminKey, *keystorePassphrase)

	} else {
		// Use interactive configuration
		loader, err := config.NewInteractiveConfigLoader()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// Create overrides map from provided flags
		overrides := make(map[string]interface{})
		if *serverURL != "" {
			overrides["server"] = *serverURL
		}
		if *username != "" {
			overrides["username"] = *username
		}
		if *theme != "" {
			overrides["theme"] = *theme
		}
		// Only override boolean flags if they were explicitly set to true
		if *isAdmin {
			overrides["admin"] = *isAdmin
		}
		if *useE2E {
			overrides["e2e"] = *useE2E
		}
		if *skipTLSVerify {
			overrides["skip-tls-verify"] = *skipTLSVerify
		}

		var adminKeyFromConfig, keystorePassFromConfig string
		cfg, _, adminKeyFromConfig, keystorePassFromConfig, err = loader.LoadOrPromptConfig(overrides)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// Override sensitive flags from command line if provided
		if *adminKey != "" {
			adminKeyFromConfig = *adminKey
		}
		if *keystorePassphrase != "" {
			keystorePassFromConfig = *keystorePassphrase
		}

		fmt.Println("\nTo connect with these settings in the future, you can use:")
		fmt.Printf("   %s\n", loader.FormatSanitizedLaunchCommand(cfg))
		fmt.Println()

		// Continue with existing client initialization...
		initializeClient(cfg, adminKeyFromConfig, keystorePassFromConfig)
	}
}

func allFlagsProvided(serverURL, username string, isAdmin bool, adminKey string, useE2E bool, keystorePassphrase string) bool {
	if serverURL == "" || username == "" {
		return false
	}

	if isAdmin && adminKey == "" {
		return false
	}

	if useE2E && keystorePassphrase == "" {
		return false
	}

	return true
}

func loadConfigFromFlags(configPath, serverURL, username, theme string, isAdmin, useE2E, skipTLSVerify bool) (*config.Config, error) {
	var cfg config.Config

	// Try to load existing config file if specified
	if configPath != "" {
		if existingCfg, err := config.LoadConfig(configPath); err == nil {
			cfg = existingCfg
		}
	} else {
		// Use platform-appropriate config path
		defaultConfigPath, err := config.GetConfigPath()
		if err == nil {
			if existingCfg, err := config.LoadConfig(defaultConfigPath); err == nil {
				cfg = existingCfg
			}
		}
	}

	// Override with flags
	if serverURL != "" {
		cfg.ServerURL = serverURL
	}
	if username != "" {
		cfg.Username = username
	}
	if theme != "" {
		cfg.Theme = theme
	}

	cfg.IsAdmin = isAdmin
	cfg.UseE2E = useE2E
	cfg.SkipTLSVerify = skipTLSVerify

	// Set defaults
	if cfg.ServerURL == "" {
		cfg.ServerURL = "ws://localhost:8080/ws"
	}
	if cfg.Theme == "" {
		cfg.Theme = "system"
	}

	return &cfg, nil
}

func validateFlags(isAdmin bool, adminKey string, useE2E bool, keystorePassphrase string) error {
	if isAdmin && adminKey == "" {
		return fmt.Errorf("--admin flag requires --admin-key")
	}

	if useE2E && keystorePassphrase == "" {
		return fmt.Errorf("--e2e flag requires --keystore-passphrase")
	}

	return nil
}

func initializeClient(cfg *config.Config, adminKeyParam, keystorePassphraseParam string) {
	// Your existing client initialization code here...
	fmt.Printf("Connecting to %s as %s...\n", cfg.ServerURL, cfg.Username)

	// Use platform-appropriate config path for saving
	var configFilePath string
	defaultConfigPath, err := config.GetConfigPath()
	if err == nil {
		configFilePath = defaultConfigPath
	} else {
		configFilePath = "config.json" // fallback
	}

	// Initialize keystore if E2E is enabled
	var keystore *crypto.KeyStore
	if cfg.UseE2E {
		keystorePath, err := config.GetKeystorePath()
		if err != nil {
			fmt.Printf("Error getting keystore path: %v\n", err)
			os.Exit(1)
		}
		keystore = crypto.NewKeyStore(keystorePath)

		if err := keystore.Initialize(keystorePassphraseParam); err != nil {
			fmt.Printf("Error initializing keystore: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("E2E encryption enabled\n")
	}

	// Setup textarea
	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 2000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(80, 20)

	userListVp := viewport.New(18, 10) // height will be set on resize
	userListVp.SetContent(renderUserList([]string{cfg.Username}, cfg.Username, getThemeStyles(cfg.Theme), 18))

	// Additional keystore initialization if E2E is enabled
	if cfg.UseE2E && keystore != nil {
		// Check environment variable status
		if envKey := os.Getenv("MARCHAT_GLOBAL_E2E_KEY"); envKey != "" {
			fmt.Printf("Using global E2E key from environment variable\n")
		} else {
			fmt.Printf("No MARCHAT_GLOBAL_E2E_KEY environment variable found\n")
		}

		// Verify keystore is properly unlocked
		if err := verifyKeystoreUnlocked(keystore); err != nil {
			fmt.Printf("Keystore unlock verification failed: %v\n", err)
			os.Exit(1)
		}

		// Display global key info
		if globalKey := keystore.GetGlobalKey(); globalKey != nil {
			fmt.Printf("Global chat encryption: ENABLED (Key ID: %s)\n", globalKey.KeyID)
		} else {
			fmt.Printf("Global key not available\n")
			os.Exit(1)
		}

		// Test encryption roundtrip (non-blocking for production use)
		if err := validateEncryptionRoundtrip(keystore, cfg.Username); err != nil {
			fmt.Printf("Encryption validation failed: %v\n", err)
			fmt.Printf("E2E encryption will continue but may have issues\n")
			log.Printf("WARNING: Encryption validation failed: %v", err)
		} else {
			fmt.Printf("Encryption validation passed\n")
		}

		keystorePath, _ := config.GetKeystorePath()
		fmt.Printf("E2E encryption enabled with keystore: %s\n", keystorePath)
	}

	// Update global flags for compatibility with existing code
	*isAdmin = cfg.IsAdmin
	*useE2E = cfg.UseE2E
	*skipTLSVerify = cfg.SkipTLSVerify
	if len(adminKeyParam) > 0 {
		*adminKey = adminKeyParam
	}
	if len(keystorePassphraseParam) > 0 {
		*keystorePassphrase = keystorePassphraseParam
	}

	m := &model{
		cfg:              *cfg,
		configFilePath:   configFilePath,
		textarea:         ta,
		viewport:         vp,
		styles:           getThemeStyles(cfg.Theme),
		users:            []string{cfg.Username},
		userListViewport: userListVp,
		twentyFourHour:   cfg.TwentyFourHour,
		keystore:         keystore,
		useE2E:           cfg.UseE2E,
		keys:             newKeyMap(),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		m.closeWebSocket()
		p.Send(quitMsg{})
	}()

	if _, err := p.Run(); err != nil {
		log.Printf("Error running program: %v", err)
		os.Exit(1)
	}
	m.wg.Wait() // Wait for all goroutines to finish
}
