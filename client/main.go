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

// Remove debugLog variable and logger setup

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
	configPath = flag.String("config", "config.json", "Path to config file")
	serverURL  = flag.String("server", "", "Server URL (overrides config)")
	username   = flag.String("username", "", "Username (overrides config)")
	theme      = flag.String("theme", "", "Theme (overrides config)")
)

var isAdmin = flag.Bool("admin", false, "Connect as admin (requires --admin-key)")
var adminKey = flag.String("admin-key", "", "Admin key for privileged commands like :cleardb, :kick, :ban, :unban")
var useE2E = flag.Bool("e2e", false, "Enable end-to-end encryption")
var keystorePassphrase = flag.String("keystore-passphrase", "", "Passphrase for keystore (required for E2E)")
var skipTLSVerify = flag.Bool("skip-tls-verify", false, "Skip TLS certificate verification (for development)")

// Add these helper functions after the existing imports and before the model struct

// debugEncryptAndSend provides comprehensive logging around encryption
func debugEncryptAndSend(recipients []string, plaintext string, ws *websocket.Conn, keystore *crypto.KeyStore, username string) error {
	log.Printf("DEBUG: Starting encryption for %d recipients", len(recipients))
	log.Printf("DEBUG: Plaintext length: %d", len(plaintext))

	// Check keystore status AND private key access
	if keystore == nil {
		log.Printf("ERROR: Keystore is nil")
		return fmt.Errorf("keystore not initialized")
	}
	log.Printf("DEBUG: Keystore loaded: %t", keystore != nil)

	// CRITICAL: Verify private key is accessible (not just public keys)
	keypair := keystore.GetKeyPair()
	if keypair == nil {
		log.Printf("ERROR: Cannot access keypair from keystore")
		return fmt.Errorf("keypair not accessible - keystore may not be unlocked")
	}
	if keypair.PrivateKey == nil {
		log.Printf("ERROR: Private key is nil - keystore may not be unlocked")
		return fmt.Errorf("private key is nil - keystore unlock failed")
	}
	log.Printf("DEBUG: Private key accessible: %t", keypair.PrivateKey != nil)

	// Log recipient key lookup
	for _, recipient := range recipients {
		if pubKey := keystore.GetPublicKey(recipient); pubKey != nil {
			log.Printf("DEBUG: Found key for %s (length: %d)", recipient, len(pubKey.PublicKey))
		} else {
			log.Printf("WARNING: No key found for recipient: %s", recipient)
			return fmt.Errorf("missing public key for recipient: %s", recipient)
		}
	}

	// Perform encryption with error catching
	conversationID := "global" // For now, use global conversation
	encryptedMsg, err := keystore.EncryptMessage(username, plaintext, conversationID)
	if err != nil {
		log.Printf("ERROR: Encryption failed: %v", err)
		return fmt.Errorf("encryption failed: %v", err)
	}

	log.Printf("DEBUG: Raw encryption result - encrypted length: %d", len(encryptedMsg.Encrypted))

	// Guard against empty ciphertext
	if len(encryptedMsg.Encrypted) == 0 {
		log.Printf("ERROR: Encryption returned empty ciphertext")
		return fmt.Errorf("encryption returned empty ciphertext; aborting send")
	}

	// CRITICAL: Combine nonce + encrypted data and base64 encode for safe JSON transport
	// Format: nonce + encrypted_data (concatenated, then base64 encoded)
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

	// Create a regular Message struct for the server (not EncryptedMessage)
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

	log.Printf("DEBUG: Message sent successfully")
	return nil
}

// validateEncryptionRoundtrip tests encryption primitives
func validateEncryptionRoundtrip(keystore *crypto.KeyStore, username string) error {
	testPlaintext := "Hello, encryption test!"

	log.Printf("DEBUG: Testing encryption roundtrip")

	// Create a test public key for validation
	testKeypair, err := shared.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate test keypair: %v", err)
	}

	// Store test public key
	testPubKeyInfo := &shared.PublicKeyInfo{
		Username:  "testuser",
		PublicKey: testKeypair.PublicKey,
		CreatedAt: time.Now(),
		KeyID:     shared.GetKeyID(testKeypair.PublicKey),
	}

	if err := keystore.StorePublicKey(testPubKeyInfo); err != nil {
		return fmt.Errorf("failed to store test public key: %v", err)
	}

	// Encrypt
	conversationID := "test"
	encryptedMsg, err := keystore.EncryptMessage(username, testPlaintext, conversationID)
	if err != nil {
		return fmt.Errorf("encryption test failed: %v", err)
	}

	if len(encryptedMsg.Encrypted) == 0 {
		return fmt.Errorf("encryption test produced empty ciphertext")
	}

	log.Printf("DEBUG: Encryption test successful - ciphertext length: %d", len(encryptedMsg.Encrypted))

	// Test decryption roundtrip
	decryptedMsg, err := keystore.DecryptMessage(encryptedMsg, conversationID)
	if err != nil {
		return fmt.Errorf("decryption test failed: %v", err)
	}

	if decryptedMsg.Content != testPlaintext {
		return fmt.Errorf("decryption roundtrip failed: expected '%s', got '%s'", testPlaintext, decryptedMsg.Content)
	}

	log.Printf("DEBUG: Encryption roundtrip test successful")
	return nil
}

// verifyKeystoreUnlocked verifies keystore is properly unlocked
func verifyKeystoreUnlocked(keystore *crypto.KeyStore) error {
	if keystore == nil {
		return fmt.Errorf("keystore is nil")
	}

	// Try to access private key material
	keypair := keystore.GetKeyPair()
	if keypair == nil {
		return fmt.Errorf("cannot access keypair")
	}

	if keypair.PrivateKey == nil {
		return fmt.Errorf("private key is nil")
	}

	log.Printf("DEBUG: Keystore properly unlocked")
	return nil
}

// validateRecipientsHaveKeys ensures all recipients have public keys
func validateRecipientsHaveKeys(keystore *crypto.KeyStore, recipients []string) error {
	missingKeys := []string{}

	for _, recipient := range recipients {
		if keystore.GetPublicKey(recipient) == nil {
			missingKeys = append(missingKeys, recipient)
		}
	}

	if len(missingKeys) > 0 {
		return fmt.Errorf("missing keys for recipients: %s", strings.Join(missingKeys, ", "))
	}

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
	cfg       config.Config
	textarea  textarea.Model
	viewport  viewport.Model
	messages  []shared.Message
	styles    themeStyles
	banner    string
	connected bool

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
	}
}

func getThemeStyles(theme string) themeStyles {
	s := baseThemeStyles()
	switch strings.ToLower(theme) {
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
		// Remove debugLog.Printf
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
		switch v.String() {
		case "esc":
			m.closeWebSocket()
			return m, tea.Quit
		case "up":
			if m.textarea.Focused() {
				m.viewport.ScrollUp(1)
			} else {
				m.userListViewport.ScrollUp(1)
			}
			return m, nil
		case "down":
			if m.textarea.Focused() {
				m.viewport.ScrollDown(1)
			} else {
				m.userListViewport.ScrollDown(1)
			}
			return m, nil
		case "pgup":
			m.viewport.ScrollUp(m.viewport.Height)
			return m, nil
		case "pgdown":
			m.viewport.ScrollDown(m.viewport.Height)
			return m, nil
		case "ctrl+c": // Custom Copy
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
			return m, nil // Return when textarea is not focused
		case "ctrl+v": // Custom Paste
			if m.textarea.Focused() {
				text, err := clipboard.ReadAll()
				if err != nil {
					m.banner = "❌ Failed to paste from clipboard: " + err.Error()
				} else {
					m.textarea.SetValue(m.textarea.Value() + text)
				}
				return m, nil
			}
			return m, nil // Return when textarea is not focused
		case "ctrl+x": // Custom Cut
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
			return m, nil // Return when textarea is not focused
		case "ctrl+a": // Custom Select All
			if m.textarea.Focused() {
				m.textarea.SetCursor(0)
				// Since textarea doesn't support selecting all, we can copy all text to clipboard
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
			return m, nil // Return when textarea is not focused
		case "enter":
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
					_ = config.SaveConfig(*configPath, m.cfg)
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

			// E2E Encryption commands
			if text == ":showkey" {
				if !m.useE2E {
					m.banner = "E2E encryption not enabled. Use --e2e flag."
					m.textarea.SetValue("")
					return m, nil
				}
				pubKeyInfo := m.keystore.GetPublicKeyInfo(m.cfg.Username)
				if pubKeyInfo != nil {
					m.banner = fmt.Sprintf("🔑 Your public key ID: %s", pubKeyInfo.KeyID)
				} else {
					m.banner = "❌ No public key available"
				}
				m.textarea.SetValue("")
				return m, nil
			}

			if strings.HasPrefix(text, ":addkey ") {
				if !m.useE2E {
					m.banner = "E2E encryption not enabled. Use --e2e flag."
					m.textarea.SetValue("")
					return m, nil
				}
				parts := strings.Fields(text)
				if len(parts) < 3 {
					m.banner = "Usage: :addkey <username> <base64-public-key>"
					m.textarea.SetValue("")
					return m, nil
				}
				username := parts[1]
				pubKeyB64 := parts[2]

				// Decode base64 public key
				pubKey, err := base64.StdEncoding.DecodeString(pubKeyB64)
				if err != nil {
					m.banner = "❌ Invalid public key format"
					m.textarea.SetValue("")
					return m, nil
				}

				// Store the public key
				pubKeyInfo := &shared.PublicKeyInfo{
					Username:  username,
					PublicKey: pubKey,
					CreatedAt: time.Now(),
					KeyID:     shared.GetKeyID(pubKey),
				}

				if err := m.keystore.StorePublicKey(pubKeyInfo); err != nil {
					m.banner = fmt.Sprintf("❌ Failed to store key: %v", err)
				} else {
					m.banner = fmt.Sprintf("✅ Added public key for %s (ID: %s)", username, pubKeyInfo.KeyID)
				}
				m.textarea.SetValue("")
				return m, nil
			}
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
						// Use E2E encryption for messages with comprehensive debugging
						log.Printf("DEBUG: Attempting to send encrypted message: '%s'", text)

						// Validate keystore is unlocked
						if err := verifyKeystoreUnlocked(m.keystore); err != nil {
							m.banner = fmt.Sprintf("❌ Keystore not unlocked: %v", err)
							m.sending = false
							m.textarea.SetValue("")
							return m, nil
						}

						// For now, use global conversation and all users as recipients
						recipients := m.users
						if len(recipients) == 0 {
							recipients = []string{m.cfg.Username} // Fallback to self
						}

						// Validate all recipients have keys
						if err := validateRecipientsHaveKeys(m.keystore, recipients); err != nil {
							m.banner = fmt.Sprintf("❌ Missing keys: %v", err)
							m.sending = false
							m.textarea.SetValue("")
							return m, nil
						}

						// Use the debug encryption function
						if err := debugEncryptAndSend(recipients, text, m.conn, m.keystore, m.cfg.Username); err != nil {
							m.banner = fmt.Sprintf("❌ Encryption failed: %v", err)
							m.sending = false
							m.textarea.SetValue("")
							return m, nil
						}

						log.Printf("DEBUG: Encrypted message sent successfully")
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

func (m *model) View() string {
	// Header
	header := m.styles.Header.Width(m.viewport.Width + userListWidth + 4).Render(" marchat ")
	footer := m.styles.Footer.Width(m.viewport.Width + userListWidth + 4).Render(
		"[Enter] Send  [Up/Down] Scroll  [Esc] Quit  Commands: :sendfile <path> :savefile <filename> :clear :theme NAME :time" +
			func() string {
				cmds := ""
				if *isAdmin {
					cmds += " :cleardb :kick USER :ban USER :unban USER"
				}
				if m.useE2E {
					cmds += " :showkey :addkey USER KEY"
				}
				return cmds
			}(),
	)

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

	// Validate admin flags
	if *isAdmin && *adminKey == "" {
		fmt.Println("❌ Error: --admin flag requires --admin-key")
		fmt.Println("Usage: go run client/main.go --admin --admin-key your-key")
		os.Exit(1)
	}

	// Validate E2E flags
	if *useE2E && *keystorePassphrase == "" {
		fmt.Println("❌ Error: --e2e flag requires --keystore-passphrase")
		fmt.Println("Usage: go run client/main.go --e2e --keystore-passphrase your-passphrase")
		os.Exit(1)
	}

	cfg, _ := config.LoadConfig(*configPath)
	if *serverURL != "" {
		cfg.ServerURL = *serverURL
	}
	if *username != "" {
		cfg.Username = *username
	}
	if *theme != "" {
		cfg.Theme = *theme
	}
	if cfg.Username == "" {
		fmt.Println("Error: --username not provided and no config found.")
		flag.Usage()
		os.Exit(1)
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = "ws://localhost:8080/ws"
	}
	if !strings.HasPrefix(cfg.ServerURL, "ws://") && !strings.HasPrefix(cfg.ServerURL, "wss://") {
		log.Printf("Warning: --server should be a WebSocket URL (ws:// or wss://), not http://")
	}
	if cfg.Theme == "" {
		cfg.Theme = "modern"
	}

	// Setup textarea
	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 280
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(80, 20)

	userListVp := viewport.New(18, 10) // height will be set on resize
	userListVp.SetContent(renderUserList([]string{cfg.Username}, cfg.Username, getThemeStyles(cfg.Theme), 18))

	// Initialize keystore if E2E is enabled
	var keystore *crypto.KeyStore
	if *useE2E {
		keystorePath := filepath.Join(filepath.Dir(*configPath), "keystore.dat")
		keystore = crypto.NewKeyStore(keystorePath)

		// Initialize or load keystore
		if err := keystore.Initialize(*keystorePassphrase); err != nil {
			fmt.Printf("❌ Error initializing keystore: %v\n", err)
			os.Exit(1)
		}

		// Verify keystore is properly unlocked
		if err := verifyKeystoreUnlocked(keystore); err != nil {
			fmt.Printf("❌ Keystore unlock verification failed: %v\n", err)
			os.Exit(1)
		}

		// Test encryption roundtrip
		if err := validateEncryptionRoundtrip(keystore, cfg.Username); err != nil {
			fmt.Printf("❌ Encryption validation failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("🔐 E2E encryption enabled with keystore: %s\n", keystorePath)
		fmt.Printf("✅ Encryption validation passed\n")
	}

	m := &model{
		cfg:              cfg,
		textarea:         ta,
		viewport:         vp,
		styles:           getThemeStyles(cfg.Theme),
		users:            []string{cfg.Username},
		userListViewport: userListVp,
		twentyFourHour:   cfg.TwentyFourHour,
		keystore:         keystore,
		useE2E:           *useE2E,
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
