package config

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B9D"))
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	cursorStyle  = focusedStyle
	noStyle      = lipgloss.NewStyle()
	helpStyle    = blurredStyle
	titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFEAA7")).Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))

	focusedButton = focusedStyle.Render("[ Connect ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Connect"))
)

type configField int

const (
	serverURLField configField = iota
	usernameField
	adminField
	adminKeyField
	e2eField
	keystorePassField
	themeField
	submitButton
)

type ConfigUIModel struct {
	focusIndex    int
	inputs        []textinput.Model
	config        *Config
	errorMessage  string
	showAdminKey  bool
	showE2EFields bool
	finished      bool
	cancelled     bool
}

func NewConfigUI() ConfigUIModel {
	m := ConfigUIModel{
		inputs: make([]textinput.Model, 7), // 7 input fields
		config: &Config{
			Theme:          "system",
			TwentyFourHour: true,
		},
	}

	// Initialize text inputs
	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.Cursor.Style = cursorStyle

		switch configField(i) {
		case serverURLField:
			t.Placeholder = "wss://marchat.mckerley.net/ws"
			t.Prompt = "Server URL: "
			t.CharLimit = 256
			t.Width = 50
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
		case usernameField:
			t.Placeholder = "Enter your username"
			t.Prompt = "Username: "
			t.CharLimit = 32
			t.Width = 30
		case adminField:
			t.Placeholder = "y/n"
			t.Prompt = "Admin user? "
			t.CharLimit = 1
			t.Width = 5
		case adminKeyField:
			t.Placeholder = "Enter admin key"
			t.Prompt = "Admin Key: "
			t.CharLimit = 64
			t.Width = 40
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		case e2eField:
			t.Placeholder = "y/n"
			t.Prompt = "Enable E2E encryption? "
			t.CharLimit = 1
			t.Width = 5
		case keystorePassField:
			t.Placeholder = "Enter keystore passphrase"
			t.Prompt = "Keystore passphrase: "
			t.CharLimit = 128
			t.Width = 40
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		case themeField:
			t.Placeholder = "system"
			t.Prompt = "Theme: "
			t.CharLimit = 20
			t.Width = 20
		}

		m.inputs[i] = t
	}

	return m
}

func (m ConfigUIModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ConfigUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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

			// Handle special logic for conditional fields
			if s == "enter" {
				switch configField(m.focusIndex) {
				case adminField:
					value := strings.ToLower(strings.TrimSpace(m.inputs[adminField].Value()))
					m.showAdminKey = value == "y" || value == "yes"
					if !m.showAdminKey {
						m.inputs[adminKeyField].SetValue("")
					}
				case e2eField:
					value := strings.ToLower(strings.TrimSpace(m.inputs[e2eField].Value()))
					m.showE2EFields = value == "y" || value == "yes"
					if !m.showE2EFields {
						m.inputs[keystorePassField].SetValue("")
					}
				}
			}

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

func (m *ConfigUIModel) getNextValidFocus(index int, reverse bool) int {
	for {
		if index < 0 {
			return int(submitButton)
		}
		if index > int(submitButton) {
			return 0
		}

		field := configField(index)

		// Skip admin key field if not admin
		if field == adminKeyField && !m.showAdminKey {
			if reverse {
				index--
			} else {
				index++
			}
			continue
		}

		// Skip keystore passphrase field if E2E not enabled
		if field == keystorePassField && !m.showE2EFields {
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

func (m *ConfigUIModel) updateFocus() {
	for i := 0; i < len(m.inputs); i++ {
		if i == m.focusIndex {
			m.inputs[i].Focus()
			m.inputs[i].PromptStyle = focusedStyle
			m.inputs[i].TextStyle = focusedStyle
		} else {
			m.inputs[i].Blur()
			m.inputs[i].PromptStyle = noStyle
			m.inputs[i].TextStyle = noStyle
		}
	}
}

func (m *ConfigUIModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m *ConfigUIModel) validateAndBuildConfig() error {
	// Clear previous error
	m.errorMessage = ""

	// Get values
	serverURL := strings.TrimSpace(m.inputs[serverURLField].Value())
	username := strings.TrimSpace(m.inputs[usernameField].Value())
	adminStr := strings.ToLower(strings.TrimSpace(m.inputs[adminField].Value()))
	adminKey := strings.TrimSpace(m.inputs[adminKeyField].Value())
	e2eStr := strings.ToLower(strings.TrimSpace(m.inputs[e2eField].Value()))
	keystorePass := m.inputs[keystorePassField].Value()
	theme := strings.TrimSpace(m.inputs[themeField].Value())

	// Validation
	if serverURL == "" {
		serverURL = "wss://marchat.mckerley.net/ws"
	}
	if username == "" {
		return fmt.Errorf("username is required")
	}

	isAdmin := adminStr == "y" || adminStr == "yes"
	useE2E := e2eStr == "y" || e2eStr == "yes"

	if isAdmin && adminKey == "" {
		return fmt.Errorf("admin key is required for admin users")
	}

	if useE2E && keystorePass == "" {
		return fmt.Errorf("keystore passphrase is required for E2E encryption")
	}

	if theme == "" {
		theme = "system"
	}

	// Build config
	m.config = &Config{
		Username:       username,
		ServerURL:      serverURL,
		IsAdmin:        isAdmin,
		AdminKey:       adminKey,
		UseE2E:         useE2E,
		Theme:          theme,
		TwentyFourHour: true,
	}

	return nil
}

func (m ConfigUIModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("🚀 marchat Configuration"))
	b.WriteString("\n\n")

	// Server URL
	b.WriteString(m.inputs[serverURLField].View())
	b.WriteString("\n")

	// Username
	b.WriteString(m.inputs[usernameField].View())
	b.WriteString("\n")

	// Admin
	b.WriteString(m.inputs[adminField].View())
	if m.focusIndex == int(adminField) {
		b.WriteString(helpStyle.Render(" (y/n)"))
	}
	b.WriteString("\n")

	// Admin key (conditional)
	if m.showAdminKey {
		b.WriteString(m.inputs[adminKeyField].View())
		b.WriteString("\n")
	}

	// E2E Encryption
	b.WriteString(m.inputs[e2eField].View())
	if m.focusIndex == int(e2eField) {
		b.WriteString(helpStyle.Render(" (y/n)"))
	}
	b.WriteString("\n")

	// Keystore passphrase (conditional)
	if m.showE2EFields {
		b.WriteString(m.inputs[keystorePassField].View())
		b.WriteString("\n")
	}

	// Theme
	b.WriteString(m.inputs[themeField].View())
	if m.focusIndex == int(themeField) {
		b.WriteString(helpStyle.Render(" (system, patriot, retro, modern)"))
	}
	b.WriteString("\n\n")

	// Submit button
	button := &blurredButton
	if m.focusIndex == int(submitButton) {
		button = &focusedButton
	}
	b.WriteString(*button)
	b.WriteString("\n\n")

	// Error message
	if m.errorMessage != "" {
		b.WriteString(errorStyle.Render("❌ " + m.errorMessage))
		b.WriteString("\n\n")
	}

	// Help
	b.WriteString(helpStyle.Render("Tab/Shift+Tab: Navigate • Enter: Select/Submit • Esc: Cancel"))

	return b.String()
}

// GetConfig returns the built configuration
func (m ConfigUIModel) GetConfig() *Config {
	return m.config
}

// IsFinished returns true if the user completed the configuration
func (m ConfigUIModel) IsFinished() bool {
	return m.finished
}

// IsCancelled returns true if the user cancelled the configuration
func (m ConfigUIModel) IsCancelled() bool {
	return m.cancelled
}

// GetKeystorePassphrase returns the keystore passphrase
func (m ConfigUIModel) GetKeystorePassphrase() string {
	if !m.showE2EFields {
		return ""
	}
	return m.inputs[keystorePassField].Value()
}

// ProfileSelectionModel handles profile selection UI
type ProfileSelectionModel struct {
	profiles  []ConnectionProfile
	cursor    int
	selected  bool
	cancelled bool
	choice    int
}

func NewProfileSelectionModel(profiles []ConnectionProfile) ProfileSelectionModel {
	return ProfileSelectionModel{
		profiles: profiles,
		cursor:   0,
	}
}

func (m ProfileSelectionModel) Init() tea.Cmd {
	return nil
}

func (m ProfileSelectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.profiles)-1 {
				m.cursor++
			}
		case "enter":
			m.choice = m.cursor
			m.selected = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ProfileSelectionModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Quick Start - Select a connection:"))
	b.WriteString("\n\n")

	for i, profile := range m.profiles {
		status := ""
		if profile.IsAdmin {
			status += " [Admin]"
		}
		if profile.UseE2E {
			status += " [E2E]"
		}
		if i == 0 && profile.LastUsed > 0 {
			status += " [Recent]"
		}

		line := fmt.Sprintf("%s (%s@%s)%s", profile.Name, profile.Username, profile.ServerURL, status)

		if i == m.cursor {
			b.WriteString(focusedStyle.Render("▶ " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓: Navigate • Enter: Select • Esc: Cancel"))

	return b.String()
}

func (m ProfileSelectionModel) IsSelected() bool {
	return m.selected
}

func (m ProfileSelectionModel) IsCancelled() bool {
	return m.cancelled
}

func (m ProfileSelectionModel) GetChoice() int {
	return m.choice
}

// RunProfileSelection runs the profile selection UI
func RunProfileSelection(profiles []ConnectionProfile) (int, error) {
	model := NewProfileSelectionModel(profiles)

	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return -1, fmt.Errorf("failed to run profile selection UI: %w", err)
	}

	selectionModel := finalModel.(ProfileSelectionModel)

	if selectionModel.IsCancelled() {
		return -1, fmt.Errorf("profile selection cancelled by user")
	}

	if !selectionModel.IsSelected() {
		return -1, fmt.Errorf("no profile selected")
	}

	return selectionModel.GetChoice(), nil
}

// RunInteractiveConfig runs the interactive configuration UI
func RunInteractiveConfig() (*Config, string, error) {
	model := NewConfigUI()

	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return nil, "", fmt.Errorf("failed to run configuration UI: %w", err)
	}

	configModel := finalModel.(ConfigUIModel)

	if configModel.IsCancelled() {
		return nil, "", fmt.Errorf("configuration cancelled by user")
	}

	if !configModel.IsFinished() {
		return nil, "", fmt.Errorf("configuration not completed")
	}

	return configModel.GetConfig(), configModel.GetKeystorePassphrase(), nil
}
