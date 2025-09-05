package main

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/quick"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

type codeSnippetState int

const (
	stateSelectLang codeSnippetState = iota
	stateInputCode
	stateConfirmSend
)

// Wrap language strings into a type that implements list.Item
type langItem string

func (l langItem) Title() string       { return string(l) }
func (l langItem) Description() string { return "" }
func (l langItem) FilterValue() string { return string(l) }

type codeSnippetModel struct {
	state     codeSnippetState
	languages []string
	selected  string
	code      string
	lines     []string
	cursorX   int
	cursorY   int
	langList  list.Model
	highlight string
	styles    themeStyles
	width     int
	height    int
	// Integration with main model
	onSend   func(string) // Callback to send the highlighted code
	onCancel func()       // Callback to cancel
	// For sub-model integration
	initialized bool
}

func newCodeSnippetModel(styles themeStyles, width, height int, onSend func(string), onCancel func()) codeSnippetModel {
	languages := []string{
		"go", "python", "javascript", "typescript", "java", "c", "cpp", "csharp",
		"rust", "php", "ruby", "swift", "kotlin", "scala", "haskell", "clojure",
		"lua", "perl", "bash", "powershell", "sql", "html", "css", "json",
		"yaml", "xml", "markdown", "dockerfile", "makefile", "vim", "diff",
	}

	items := make([]list.Item, len(languages))
	for i, lang := range languages {
		items[i] = langItem(lang)
	}

	l := list.New(items, list.NewDefaultDelegate(), width-4, height-8)
	l.Title = "Select Programming Language"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	return codeSnippetModel{
		state:       stateSelectLang,
		languages:   languages,
		langList:    l,
		lines:       []string{""},
		cursorX:     0,
		cursorY:     0,
		styles:      styles,
		width:       width,
		height:      height,
		onSend:      onSend,
		onCancel:    onCancel,
		initialized: true,
	}
}

func (m codeSnippetModel) Init() tea.Cmd {
	return nil
}

func (m codeSnippetModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateSelectLang:
		var cmd tea.Cmd
		m.langList, cmd = m.langList.Update(msg)
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				if item, ok := m.langList.SelectedItem().(langItem); ok {
					m.selected = string(item)
				} else {
					m.selected = m.languages[m.langList.Index()]
				}
				m.state = stateInputCode
			case "esc", "ctrl+c":
				m.onCancel()
				return m, nil
			}
		}
		return m, cmd

	case stateInputCode:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+s":
				// Ctrl+S to finish input and show preview
				m.code = strings.Join(m.lines, "\n")
				var sb strings.Builder
				err := quick.Highlight(&sb, m.code, m.selected, "terminal256", "monokai")
				if err != nil {
					m.highlight = fmt.Sprintf("Error: %v", err)
				} else {
					m.highlight = sb.String()
				}
				m.state = stateConfirmSend
			case "esc", "ctrl+c":
				m.onCancel()
				return m, nil
			case "enter":
				// Regular Enter - add new line
				currentLine := m.lines[m.cursorY]
				beforeCursor := currentLine[:m.cursorX]
				afterCursor := currentLine[m.cursorX:]

				// Split current line
				m.lines[m.cursorY] = beforeCursor

				// Insert new line
				newLines := make([]string, len(m.lines)+1)
				copy(newLines, m.lines[:m.cursorY+1])
				newLines[m.cursorY+1] = afterCursor
				copy(newLines[m.cursorY+2:], m.lines[m.cursorY+1:])
				m.lines = newLines

				// Move cursor to start of new line
				m.cursorY++
				m.cursorX = 0
			case "backspace":
				if m.cursorX > 0 {
					// Delete character before cursor
					currentLine := m.lines[m.cursorY]
					m.lines[m.cursorY] = currentLine[:m.cursorX-1] + currentLine[m.cursorX:]
					m.cursorX--
				} else if m.cursorY > 0 {
					// Merge with previous line
					prevLine := m.lines[m.cursorY-1]
					currentLine := m.lines[m.cursorY]
					m.lines[m.cursorY-1] = prevLine + currentLine

					// Remove current line
					m.lines = append(m.lines[:m.cursorY], m.lines[m.cursorY+1:]...)
					m.cursorY--
					m.cursorX = len(prevLine)
				}
			case "delete":
				if m.cursorX < len(m.lines[m.cursorY]) {
					// Delete character at cursor
					currentLine := m.lines[m.cursorY]
					m.lines[m.cursorY] = currentLine[:m.cursorX] + currentLine[m.cursorX+1:]
				} else if m.cursorY < len(m.lines)-1 {
					// Merge with next line
					currentLine := m.lines[m.cursorY]
					nextLine := m.lines[m.cursorY+1]
					m.lines[m.cursorY] = currentLine + nextLine

					// Remove next line
					m.lines = append(m.lines[:m.cursorY+1], m.lines[m.cursorY+2:]...)
				}
			case "left":
				if m.cursorX > 0 {
					m.cursorX--
				} else if m.cursorY > 0 {
					m.cursorY--
					m.cursorX = len(m.lines[m.cursorY])
				}
			case "right":
				if m.cursorX < len(m.lines[m.cursorY]) {
					m.cursorX++
				} else if m.cursorY < len(m.lines)-1 {
					m.cursorY++
					m.cursorX = 0
				}
			case "up":
				if m.cursorY > 0 {
					m.cursorY--
					if m.cursorX > len(m.lines[m.cursorY]) {
						m.cursorX = len(m.lines[m.cursorY])
					}
				}
			case "down":
				if m.cursorY < len(m.lines)-1 {
					m.cursorY++
					if m.cursorX > len(m.lines[m.cursorY]) {
						m.cursorX = len(m.lines[m.cursorY])
					}
				}
			case "home":
				m.cursorX = 0
			case "end":
				m.cursorX = len(m.lines[m.cursorY])
			default:
				// Handle regular character input
				if len(msg.String()) == 1 {
					char := msg.String()
					currentLine := m.lines[m.cursorY]
					m.lines[m.cursorY] = currentLine[:m.cursorX] + char + currentLine[m.cursorX:]
					m.cursorX++
				}
			}
		}
		return m, nil

	case stateConfirmSend:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				// Send the highlighted code as a message
				markdown := fmt.Sprintf("```%s\n%s\n```", m.selected, m.code)
				m.onSend(markdown)
				m.onCancel() // Close the interface after sending
				return m, nil
			case "esc", "ctrl+c":
				m.onCancel()
				return m, nil
			case "r":
				// restart input
				m.state = stateSelectLang
				m.lines = []string{""}
				m.cursorX = 0
				m.cursorY = 0
			case "c":
				// copy to clipboard - sanitize the code first
				sanitizedCode := strings.ReplaceAll(m.code, "\x00", "")
				clipboard.WriteAll(sanitizedCode)
			}
		}
		return m, nil
	}

	return m, nil
}

func (m codeSnippetModel) View() string {
	switch m.state {
	case stateSelectLang:
		return m.langList.View() + "\n" + m.styles.Time.Render("Press Enter to select language, Esc to cancel.")
	case stateInputCode:
		var sb strings.Builder
		sb.WriteString(m.styles.User.Render(fmt.Sprintf("Language: %s", m.selected)) + "\n\n")

		// Display all lines with cursor
		for i, line := range m.lines {
			if i == m.cursorY {
				// Current line with cursor
				beforeCursor := line[:m.cursorX]
				afterCursor := line[m.cursorX:]
				sb.WriteString(m.styles.Msg.Render(fmt.Sprintf("> %s|%s", beforeCursor, afterCursor)) + "\n")
			} else {
				// Other lines
				sb.WriteString(m.styles.Msg.Render(fmt.Sprintf("  %s", line)) + "\n")
			}
		}

		sb.WriteString("\n" + m.styles.Time.Render("Press Ctrl+S to preview, Enter for new line, Esc to cancel."))
		return sb.String()
	case stateConfirmSend:
		return m.formatCodeBlock(m.selected, m.code, false)
	default:
		return "Unknown state"
	}
}

func (m codeSnippetModel) formatCodeBlock(language, plainCode string, copied bool) string {
	// Create a markdown code block with the language using the plain code
	markdown := fmt.Sprintf("```%s\n%s\n```", language, plainCode)

	// Use Glamour to render it with proper styling
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(m.width-4),
	)

	rendered, err := r.Render(markdown)
	if err != nil {
		// Fallback to simple formatting if Glamour fails
		return fmt.Sprintf("```%s\n%s\n```\n\n%s", language, plainCode, m.styles.Time.Render("Press Enter to send, 'r' to restart, Esc to cancel."))
	}

	// Add status message
	if copied {
		rendered += "\n" + m.styles.Banner.Render("✓ Code copied to clipboard!")
	} else {
		rendered += "\n" + m.styles.Time.Render("Press Enter to send message, 'r' to restart, 'c' to copy, Esc to cancel.")
	}

	return rendered
}
