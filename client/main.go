package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"marchat/client/config"
	"marchat/shared"

	"os/signal"
	"syscall"

	"encoding/json"

	"context"
	"sync"

	"log"

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
var adminKey = flag.String("admin-key", "", "Admin key for privileged commands like :cleardb")

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
	conn, _, err := websocket.DefaultDialer.Dial(fullURL, nil)
	if err != nil {
		return err
	}
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
	if err := m.conn.WriteJSON(handshake); err != nil {
		return err
	}
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
					m.msgChan <- wsErr(err)
					return
				}
				// Try to unmarshal as shared.Message first
				var msg shared.Message
				if err := json.Unmarshal(raw, &msg); err == nil {
					if msg.Sender != "" {
						m.msgChan <- msg
						continue
					}
				}
				// Then try as wsMsg
				var ws wsMsg
				if err := json.Unmarshal(raw, &ws); err == nil && ws.Type != "" {
					m.msgChan <- ws
					continue
				}
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
		return m, m.listenWebSocket()
	case shared.Message:
		// Remove debugLog.Printf
		// Cap messages to maxMessages
		if len(m.messages) >= maxMessages {
			m.messages = m.messages[len(m.messages)-maxMessages+1:]
		}
		m.messages = append(m.messages, v)
		// Store file in memory for session if file message
		if v.Type == shared.FileMessageType && v.File != nil {
			if m.receivedFiles == nil {
				m.receivedFiles = make(map[string]*shared.FileMeta)
			}
			m.receivedFiles[v.File.Filename] = v.File
		}
		m.viewport.SetContent(renderMessages(m.messages, m.styles, m.cfg.Username, m.users, m.viewport.Width, m.twentyFourHour))
		m.viewport.GotoBottom()
		m.sending = false // Only set sending=false after receiving echo
		return m, m.listenWebSocket()
	case wsErr:
		m.connected = false
		m.banner = "🚫 Connection lost. Reconnecting..."
		m.closeWebSocket()
		// Exponential backoff for reconnect
		delay := m.reconnectDelay
		if delay < reconnectMaxDelay {
			m.reconnectDelay *= 2
			if m.reconnectDelay > reconnectMaxDelay {
				m.reconnectDelay = reconnectMaxDelay
			}
		}
		log.Printf("Reconnect in %v", delay)
		return m, tea.Tick(delay, func(time.Time) tea.Msg {
			return m.Init()()
		})
	case tea.KeyMsg:
		switch v.String() {
		case "ctrl+c", "esc":
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
				parts := strings.SplitN(text, " ", 2)
				if len(parts) == 2 {
					filename := strings.TrimSpace(parts[1])
					if m.receivedFiles != nil {
						file, ok := m.receivedFiles[filename]
						if ok {
							err := os.WriteFile(filename, file.Data, 0644)
							if err != nil {
								m.banner = "❌ Failed to save file: " + err.Error()
							} else {
								m.banner = "File saved: " + filename
							}
						} else {
							m.banner = "❌ No such file received: " + filename
						}
					} else {
						m.banner = "❌ No files received yet."
					}
					m.textarea.SetValue("")
					return m, nil
				}
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
			if text == ":time" {
				m.twentyFourHour = !m.twentyFourHour
				m.cfg.TwentyFourHour = m.twentyFourHour
				_ = config.SaveConfig(*configPath, m.cfg) // ignore error for now
				m.banner = "Timestamp format: " + map[bool]string{true: "24h", false: "12h"}[m.twentyFourHour]
				m.viewport.SetContent(renderMessages(m.messages, m.styles, m.cfg.Username, m.users, m.viewport.Width, m.twentyFourHour))
				m.viewport.GotoBottom()
				m.textarea.SetValue("")
				return m, nil
			}
			if text != "" {
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
				// m.sending = false // Now set in shared.Message handler
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
		m.viewport.SetContent(renderMessages(m.messages, m.styles, m.cfg.Username, m.users, chatWidth, m.twentyFourHour))
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
		"[Enter] Send  [Up/Down] Scroll  [Esc/Ctrl+C] Quit  Commands: :sendfile <path> :savefile <filename> :clear :theme NAME :time" +
			func() string {
				cmds := ""
				if *isAdmin {
					cmds += " :cleardb"
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
		log.Printf("Username required. Use --username or set in config file.")
		return
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = "ws://localhost:9090/ws"
	}
	if !strings.HasPrefix(cfg.ServerURL, "ws://") && !strings.HasPrefix(cfg.ServerURL, "wss://") {
		log.Printf("Warning: --server should be a WebSocket URL (ws:// or wss://), not http://")
	}
	if cfg.Theme == "" {
		cfg.Theme = "modern"
	}

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

	m := &model{
		cfg:              cfg,
		textarea:         ta,
		viewport:         vp,
		styles:           getThemeStyles(cfg.Theme),
		users:            []string{cfg.Username},
		userListViewport: userListVp,
		twentyFourHour:   cfg.TwentyFourHour,
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
