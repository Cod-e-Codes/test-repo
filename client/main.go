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
	cfg      config.Config
	input    textinput.Model
	messages []shared.Message
	err      error
	styles   themeStyles
}

type themeStyles struct {
	User lipgloss.Style
	Time lipgloss.Style
	Msg  lipgloss.Style
}

func getThemeStyles(theme string) themeStyles {
	switch theme {
	case "slack":
		return themeStyles{
			User: lipgloss.NewStyle().Foreground(lipgloss.Color("#36C5F0")).Bold(true),
			Time: lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")),
			Msg:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")),
		}
	case "discord":
		return themeStyles{
			User: lipgloss.NewStyle().Foreground(lipgloss.Color("#7289DA")).Bold(true),
			Time: lipgloss.NewStyle().Foreground(lipgloss.Color("#99AAB5")),
			Msg:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")),
		}
	case "aim":
		return themeStyles{
			User: lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true),
			Time: lipgloss.NewStyle().Foreground(lipgloss.Color("#00AEEF")),
			Msg:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")),
		}
	default:
		return themeStyles{
			User: lipgloss.NewStyle().Bold(true),
			Time: lipgloss.NewStyle().Faint(true),
			Msg:  lipgloss.NewStyle(),
		}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, pollMessages(m.cfg.ServerURL))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			text := m.input.Value()
			if text != "" {
				sendMessage(m.cfg.ServerURL, m.cfg.Username, text)
				m.input.SetValue("")
			}
			return m, pollMessages(m.cfg.ServerURL)
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	case []shared.Message:
		m.messages = msg
		return m, tea.Tick(time.Second*2, func(time.Time) tea.Msg {
			return pollMessages(m.cfg.ServerURL)()
		})
	case error:
		m.err = msg
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m model) View() string {
	var b bytes.Buffer
	for _, msg := range m.messages {
		b.WriteString(fmt.Sprintf("%s %s: %s\n",
			m.styles.Time.Render("["+msg.CreatedAt.Format("15:04")+"]"),
			m.styles.User.Render(msg.Sender),
			m.styles.Msg.Render(renderEmojis(msg.Content)),
		))
	}
	b.WriteString("\n> " + m.input.View() + "\n")
	if m.err != nil {
		b.WriteString("\n[Error] " + m.err.Error())
	}
	return b.String()
}

func sendMessage(serverURL, sender, content string) {
	data := shared.Message{Sender: sender, Content: content}
	body, _ := json.Marshal(data)
	http.Post(serverURL+"/send", "application/json", bytes.NewBuffer(body))
}

func pollMessages(serverURL string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(serverURL + "/messages")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var msgs []shared.Message
		err = json.Unmarshal(body, &msgs)
		if err != nil {
			return err
		}
		return msgs
	}
}

func renderEmojis(s string) string {
	// Simple ASCII emoji replacement
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
		cfg.ServerURL = "http://localhost:8080"
	}

	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()

	m := model{
		cfg:    cfg,
		input:  ti,
		styles: getThemeStyles(cfg.Theme),
	}

	p := tea.NewProgram(m)
	if err := p.Start(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
