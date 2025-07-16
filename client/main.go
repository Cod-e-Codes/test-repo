package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"marchat/client/config"
	"marchat/shared"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	configPath = flag.String("config", "config.json", "Path to config file")
	serverURL  = flag.String("server", "", "Server URL (overrides config)")
	username   = flag.String("username", "", "Username (overrides config)")
	theme      = flag.String("theme", "", "Theme (overrides config)")
)

type model struct {
	cfg          config.Config
	input        textinput.Model
	viewport     viewport.Model
	messages     []shared.Message
	styles       themeStyles
	banner       string
	connected    bool
	lastMsgCount int // for repeat prevention
}

type themeStyles struct {
	User    lipgloss.Style
	Time    lipgloss.Style
	Msg     lipgloss.Style
	Banner  lipgloss.Style
	Box     lipgloss.Style // frame color
	Mention lipgloss.Style // mention highlighting
}

func getThemeStyles(theme string) themeStyles {
	switch theme {
	case "slack":
		return themeStyles{
			User:    lipgloss.NewStyle().Foreground(lipgloss.Color("#36C5F0")).Bold(true),
			Time:    lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")),
			Msg:     lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")),
			Banner:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F")).Bold(true),
			Box:     lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#36C5F0")),
			Mention: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF00FF")),
		}
	case "discord":
		return themeStyles{
			User:    lipgloss.NewStyle().Foreground(lipgloss.Color("#7289DA")).Bold(true),
			Time:    lipgloss.NewStyle().Foreground(lipgloss.Color("#99AAB5")),
			Msg:     lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")),
			Banner:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F")).Bold(true),
			Box:     lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#7289DA")),
			Mention: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")),
		}
	case "aim":
		return themeStyles{
			User:    lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true),
			Time:    lipgloss.NewStyle().Foreground(lipgloss.Color("#00AEEF")),
			Msg:     lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")),
			Banner:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F")).Bold(true),
			Box:     lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#FFCC00")),
			Mention: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")),
		}
	default:
		return themeStyles{
			User:    lipgloss.NewStyle().Bold(true),
			Time:    lipgloss.NewStyle().Faint(true),
			Msg:     lipgloss.NewStyle(),
			Banner:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F")).Bold(true),
			Box:     lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#AAAAAA")),
			Mention: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")),
		}
	}
}

func renderMessages(msgs []shared.Message, styles themeStyles, username string) string {
	const max = 100
	if len(msgs) > max {
		msgs = msgs[len(msgs)-max:]
	}
	var b strings.Builder
	for _, msg := range msgs {
		content := renderEmojis(msg.Content)
		if strings.Contains(msg.Content, "@"+username) {
			content = styles.Mention.Render(content)
		} else {
			content = styles.Msg.Render(content)
		}
		fmt.Fprintf(&b, "%s %s: %s\n",
			styles.Time.Render("["+msg.CreatedAt.Format("15:04")+"]"),
			styles.User.Render(msg.Sender),
			content,
		)
	}
	return b.String()
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, pollMessages(m.cfg.ServerURL))
}

type errMsg error

type messagesMsg []shared.Message

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "up":
			m.viewport.ScrollUp(1)
			return m, nil
		case "down":
			m.viewport.ScrollDown(1)
			return m, nil
		case "enter":
			text := m.input.Value()
			if strings.HasPrefix(text, ":theme ") {
				parts := strings.SplitN(text, " ", 2)
				if len(parts) == 2 {
					m.cfg.Theme = parts[1]
					m.styles = getThemeStyles(m.cfg.Theme)
					m.banner = "Theme changed to " + m.cfg.Theme
				}
				m.input.SetValue("")
				return m, nil
			}
			if text != "" {
				err := sendMessage(m.cfg.ServerURL, m.cfg.Username, text)
				if err != nil {
					m.banner = "Error sending message!"
					return m, nil
				}
				m.banner = "" // Clear any previous error
				m.input.SetValue("")
				return m, nil
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	case errMsg:
		fmt.Println("Received error:", msg.Error()) // debug
		m.connected = false
		m.viewport.SetContent(renderMessages(m.messages, m.styles, m.cfg.Username))
		if strings.Contains(msg.Error(), "connectex") || strings.Contains(msg.Error(), "connection refused") {
			m.banner = "🚫 Server unreachable. Trying to reconnect..."
		} else {
			m.banner = "⚠️ " + msg.Error()
		}
		return m, nil
	case messagesMsg:
		was := m.connected
		m.connected = true
		if !was {
			m.banner = "✅ Reconnected to server!"
		} else {
			m.banner = ""
		}
		if len(msg) == m.lastMsgCount {
			return m, tea.Tick(time.Second*2, func(time.Time) tea.Msg {
				return pollMessages(m.cfg.ServerURL)()
			})
		}
		m.messages = msg
		m.lastMsgCount = len(msg)
		m.viewport.SetContent(renderMessages(m.messages, m.styles, m.cfg.Username))
		m.viewport.GotoBottom()
		return m, tea.Tick(time.Second*2, func(time.Time) tea.Msg {
			return pollMessages(m.cfg.ServerURL)()
		})
	case tea.WindowSizeMsg:
		availableHeight := msg.Height - 3
		if availableHeight < 3 {
			availableHeight = 3
		}
		m.viewport.Width = msg.Width
		m.viewport.Height = availableHeight
		m.viewport.SetContent(renderMessages(m.messages, m.styles, m.cfg.Username))
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m model) View() string {
	var b strings.Builder

	// Banner
	if m.banner != "" {
		bannerBox := lipgloss.NewStyle().
			Width(m.viewport.Width).
			PaddingLeft(1).
			Background(lipgloss.Color("#FF5F5F")).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Render(m.banner)
		b.WriteString(bannerBox + "\n")
	}

	// Chat Viewport (no box)
	b.WriteString(m.viewport.View() + "\n")

	// Input
	inputBox := m.styles.Box.Render("> " + m.input.View())
	b.WriteString(inputBox + "\n")

	return b.String()
}

func sendMessage(serverURL, sender, content string) error {
	data := shared.Message{Sender: sender, Content: content}
	body, _ := json.Marshal(data)
	resp, err := http.Post(serverURL+"/send", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

func pollMessages(serverURL string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(serverURL + "/messages")
		if err != nil {
			return errMsg(err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var msgs []shared.Message
		err = json.Unmarshal(body, &msgs)
		if err != nil {
			return errMsg(err)
		}
		return messagesMsg(msgs)
	}
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
		fmt.Println("Username required. Use --username or set in config file.")
		return
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = "http://localhost:9090"
	}

	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()

	vp := viewport.New(80, 20)

	m := model{
		cfg:      cfg,
		input:    ti,
		viewport: vp,
		styles:   getThemeStyles(cfg.Theme),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
