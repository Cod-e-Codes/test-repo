package server

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	serverFocusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B9D"))
	serverBlurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	serverCursorStyle  = serverFocusedStyle
	serverNoStyle      = lipgloss.NewStyle()
	serverHelpStyle    = serverBlurredStyle
	serverTitleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFEAA7")).Bold(true)
	serverErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))

	serverFocusedButton = serverFocusedStyle.Render("[ Start Server ]")
	serverBlurredButton = fmt.Sprintf("[ %s ]", serverBlurredStyle.Render("Start Server"))
)

type serverConfigField int

const (
	adminKeyField serverConfigField = iota
	adminUsersField
	portField
	enableE2EField
	globalE2EKeyField
	submitButton
)

type ServerConfigModel struct {
	focusIndex    int
	inputs        []textinput.Model
	config        *ServerConfig
	errorMessage  string
	finished      bool
	cancelled     bool
	showE2EFields bool
}

type ServerConfig struct {
	AdminKey     string
	AdminUsers   string
	Port         string
	EnableE2E    bool
	GlobalE2EKey string
}

func NewServerConfigUI() ServerConfigModel {
	m := ServerConfigModel{
		inputs: make([]textinput.Model, 5), // 5 input fields
		config: &ServerConfig{
			Port: "8080",
		},
	}

	// Initialize text inputs
	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.Cursor.Style = serverCursorStyle

		switch serverConfigField(i) {
		case adminKeyField:
			t.Placeholder = "Enter a secure admin key"
			t.Prompt = "Admin Key: "
			t.CharLimit = 128
			t.Width = 50
			t.Focus()
			t.PromptStyle = serverFocusedStyle
			t.TextStyle = serverFocusedStyle
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		case adminUsersField:
			t.Placeholder = "admin1,admin2,admin3"
			t.Prompt = "Admin Users: "
			t.CharLimit = 256
			t.Width = 50
		case portField:
			t.Placeholder = "8080"
			t.Prompt = "Port: "
			t.CharLimit = 5
			t.Width = 10
			t.SetValue("8080")
		case enableE2EField:
			t.Placeholder = "y/n"
			t.Prompt = "Enable Global E2E Encryption? "
			t.CharLimit = 1
			t.Width = 5
		case globalE2EKeyField:
			t.Placeholder = "Enter or generate E2E key"
			t.Prompt = "Global E2E Key: "
			t.CharLimit = 128
			t.Width = 50
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		}

		m.inputs[i] = t
	}

	return m
}

func (m ServerConfigModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ServerConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Clear message on any key press if it's been shown
		if m.errorMessage != "" && m.errorMessage != "error" {
			m.errorMessage = ""
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			// Handle submit
			if s == "enter" && m.focusIndex == int(submitButton) {
				if err := m.validateAndBuildConfig(); err != nil {
					m.errorMessage = err.Error()
					return m, nil
				}
				m.finished = true
				return m, tea.Quit
			}

			// Update conditional field visibility on any navigation
			m.updateConditionalFields()

			// Navigate between fields
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			// Skip conditional fields that shouldn't be shown
			m.focusIndex = m.getNextValidFocus(m.focusIndex, s == "up" || s == "shift+tab")

			if m.focusIndex > int(submitButton) {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = int(submitButton)
			}

			m.updateFocus()
			return m, nil
		}
	}

	// Handle character input
	cmd := m.updateInputs(msg)
	return m, cmd
}

func (m *ServerConfigModel) updateConditionalFields() {
	// Update E2E field visibility
	e2eValue := strings.ToLower(strings.TrimSpace(m.inputs[enableE2EField].Value()))
	m.showE2EFields = e2eValue == "y" || e2eValue == "yes"
	if !m.showE2EFields {
		m.inputs[globalE2EKeyField].SetValue("")
	}
}

func (m *ServerConfigModel) getNextValidFocus(index int, reverse bool) int {
	for {
		if index < 0 {
			return int(submitButton)
		}
		if index > int(submitButton) {
			return 0
		}

		field := serverConfigField(index)

		// Skip E2E key field if E2E not enabled
		if field == globalE2EKeyField && !m.showE2EFields {
			if reverse {
				index--
			} else {
				index++
			}
			continue
		}

		break
	}
	return index
}

func (m *ServerConfigModel) updateFocus() {
	for i := 0; i < len(m.inputs); i++ {
		if i == m.focusIndex {
			m.inputs[i].Focus()
			m.inputs[i].PromptStyle = serverFocusedStyle
			m.inputs[i].TextStyle = serverFocusedStyle
		} else {
			m.inputs[i].Blur()
			m.inputs[i].PromptStyle = serverNoStyle
			m.inputs[i].TextStyle = serverNoStyle
		}
	}
}

func (m *ServerConfigModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m *ServerConfigModel) validateAndBuildConfig() error {
	// Clear previous error
	m.errorMessage = ""

	// Get values
	adminKey := strings.TrimSpace(m.inputs[adminKeyField].Value())
	adminUsers := strings.TrimSpace(m.inputs[adminUsersField].Value())
	port := strings.TrimSpace(m.inputs[portField].Value())
	e2eStr := strings.ToLower(strings.TrimSpace(m.inputs[enableE2EField].Value()))
	globalE2EKey := strings.TrimSpace(m.inputs[globalE2EKeyField].Value())

	// Validation
	if adminKey == "" {
		return fmt.Errorf("admin key is required")
	}
	if adminUsers == "" {
		return fmt.Errorf("at least one admin user is required")
	}
	if port == "" {
		return fmt.Errorf("port is required")
	}

	// Validate port is numeric
	if port != "8080" && (port < "1" || port > "65535") {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	enableE2E := e2eStr == "y" || e2eStr == "yes"
	if enableE2E && globalE2EKey == "" {
		return fmt.Errorf("global E2E key is required when E2E encryption is enabled")
	}

	// Build config
	m.config = &ServerConfig{
		AdminKey:     adminKey,
		AdminUsers:   adminUsers,
		Port:         port,
		EnableE2E:    enableE2E,
		GlobalE2EKey: globalE2EKey,
	}

	return nil
}

func (m ServerConfigModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(serverTitleStyle.Render("🚀 marchat Server Configuration"))
	b.WriteString("\n\n")

	// Show message if any
	if m.errorMessage != "" {
		b.WriteString(serverErrorStyle.Render("✗ " + m.errorMessage))
		b.WriteString("\n\n")
	}

	// Admin Key
	b.WriteString(m.inputs[adminKeyField].View())
	b.WriteString("\n")

	// Admin Users
	b.WriteString(m.inputs[adminUsersField].View())
	if m.focusIndex == int(adminUsersField) {
		b.WriteString(serverHelpStyle.Render(" (comma-separated usernames)"))
	}
	b.WriteString("\n")

	// Port
	b.WriteString(m.inputs[portField].View())
	if m.focusIndex == int(portField) {
		b.WriteString(serverHelpStyle.Render(" (default: 8080)"))
	}
	b.WriteString("\n")

	// E2E Encryption
	b.WriteString(m.inputs[enableE2EField].View())
	if m.focusIndex == int(enableE2EField) {
		b.WriteString(serverHelpStyle.Render(" (y/n)"))
	}
	b.WriteString("\n")

	// Global E2E Key (conditional)
	if m.showE2EFields {
		b.WriteString(m.inputs[globalE2EKeyField].View())
		if m.focusIndex == int(globalE2EKeyField) {
			b.WriteString(serverHelpStyle.Render(" (leave empty to auto-generate)"))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Submit button
	button := &serverBlurredButton
	if m.focusIndex == int(submitButton) {
		button = &serverFocusedButton
	}
	b.WriteString(*button)
	b.WriteString("\n\n")

	// Help
	b.WriteString(serverHelpStyle.Render("Tab/Shift+Tab: Navigate • Enter: Select/Submit • Esc: Cancel"))

	return b.String()
}

// GetConfig returns the built configuration
func (m ServerConfigModel) GetConfig() *ServerConfig {
	return m.config
}

// IsFinished returns true if the user completed the configuration
func (m ServerConfigModel) IsFinished() bool {
	return m.finished
}

// IsCancelled returns true if the user cancelled the configuration
func (m ServerConfigModel) IsCancelled() bool {
	return m.cancelled
}

// RunServerConfig runs the interactive server configuration UI
func RunServerConfig() (*ServerConfig, error) {
	model := NewServerConfigUI()

	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run server configuration UI: %w", err)
	}

	configModel := finalModel.(ServerConfigModel)

	if configModel.IsCancelled() {
		return nil, fmt.Errorf("server configuration cancelled by user")
	}

	if !configModel.IsFinished() {
		return nil, fmt.Errorf("server configuration not completed")
	}

	return configModel.GetConfig(), nil
}
