package store

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cod-e-Codes/marchat/plugin/sdk"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StorePlugin represents a plugin in the store
type StorePlugin struct {
	Name        string              `json:"name"`
	Version     string              `json:"version"`
	Description string              `json:"description"`
	Author      string              `json:"author"`
	License     string              `json:"license"`
	Repository  string              `json:"repository,omitempty"`
	Homepage    string              `json:"homepage,omitempty"`
	DownloadURL string              `json:"download_url"`
	Checksum    string              `json:"checksum,omitempty"`
	Category    string              `json:"category"`
	Tags        []string            `json:"tags"`
	Installed   bool                `json:"-"`
	Enabled     bool                `json:"-"`
	Commands    []sdk.PluginCommand `json:"commands"`
}

// Store represents the plugin store
type Store struct {
	plugins     []StorePlugin
	registryURL string
	cacheFile   string
	lastUpdate  time.Time
}

// NewStore creates a new plugin store
func NewStore(registryURL, cacheDir string) *Store {
	return &Store{
		registryURL: registryURL,
		cacheFile:   filepath.Join(cacheDir, "store_cache.json"),
	}
}

// Refresh fetches the latest plugin registry
func (s *Store) Refresh() error {
	resp, err := http.Get(s.registryURL)
	if err != nil {
		return fmt.Errorf("failed to fetch registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read registry: %w", err)
	}

	var plugins []StorePlugin
	if err := json.Unmarshal(data, &plugins); err != nil {
		return fmt.Errorf("failed to parse registry: %w", err)
	}

	s.plugins = plugins
	s.lastUpdate = time.Now()

	// Cache the registry
	if err := s.saveCache(); err != nil {
		return fmt.Errorf("failed to save cache: %w", err)
	}

	return nil
}

// LoadFromCache loads plugins from cache
func (s *Store) LoadFromCache() error {
	data, err := os.ReadFile(s.cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file, not an error
		}
		return fmt.Errorf("failed to read cache: %w", err)
	}

	var plugins []StorePlugin
	if err := json.Unmarshal(data, &plugins); err != nil {
		return fmt.Errorf("failed to parse cache: %w", err)
	}

	s.plugins = plugins
	return nil
}

// GetPlugins returns all plugins
func (s *Store) GetPlugins() []StorePlugin {
	return s.plugins
}

// GetPlugin returns a specific plugin
func (s *Store) GetPlugin(name string) *StorePlugin {
	for _, plugin := range s.plugins {
		if plugin.Name == name {
			return &plugin
		}
	}
	return nil
}

// FilterPlugins filters plugins by category, tags, or search term
func (s *Store) FilterPlugins(category, search string, tags []string) []StorePlugin {
	var filtered []StorePlugin

	for _, plugin := range s.plugins {
		// Category filter
		if category != "" && plugin.Category != category {
			continue
		}

		// Search filter
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(plugin.Name), searchLower) &&
				!strings.Contains(strings.ToLower(plugin.Description), searchLower) &&
				!strings.Contains(strings.ToLower(plugin.Author), searchLower) {
				continue
			}
		}

		// Tags filter
		if len(tags) > 0 {
			found := false
			for _, tag := range tags {
				for _, pluginTag := range plugin.Tags {
					if pluginTag == tag {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				continue
			}
		}

		filtered = append(filtered, plugin)
	}

	return filtered
}

// GetCategories returns all available categories
func (s *Store) GetCategories() []string {
	categories := make(map[string]bool)
	for _, plugin := range s.plugins {
		if plugin.Category != "" {
			categories[plugin.Category] = true
		}
	}

	var result []string
	for category := range categories {
		result = append(result, category)
	}
	return result
}

// GetTags returns all available tags
func (s *Store) GetTags() []string {
	tags := make(map[string]bool)
	for _, plugin := range s.plugins {
		for _, tag := range plugin.Tags {
			tags[tag] = true
		}
	}

	var result []string
	for tag := range tags {
		result = append(result, tag)
	}
	return result
}

// saveCache saves the plugin list to cache
func (s *Store) saveCache() error {
	data, err := json.MarshalIndent(s.plugins, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	// Ensure cache directory exists
	cacheDir := filepath.Dir(s.cacheFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	if err := os.WriteFile(s.cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache: %w", err)
	}

	return nil
}

// UpdateInstalledStatus updates the installed/enabled status of plugins
func (s *Store) UpdateInstalledStatus(installedPlugins map[string]bool, enabledPlugins map[string]bool) {
	for i := range s.plugins {
		s.plugins[i].Installed = installedPlugins[s.plugins[i].Name]
		s.plugins[i].Enabled = enabledPlugins[s.plugins[i].Name]
	}
}

// StoreUI represents the terminal UI for the plugin store
type StoreUI struct {
	list     list.Model
	search   textinput.Model
	spinner  spinner.Model
	store    *Store
	state    string // "loading", "browsing", "installing"
	selected *StorePlugin
	err      error
}

// NewStoreUI creates a new store UI
func NewStoreUI(store *Store) *StoreUI {
	// Create list items from plugins
	var items []list.Item
	for _, plugin := range store.GetPlugins() {
		items = append(items, pluginItem{plugin})
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Plugin Store"
	l.SetShowHelp(true)

	search := textinput.New()
	search.Placeholder = "Search plugins..."
	search.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &StoreUI{
		list:    l,
		search:  search,
		spinner: s,
		store:   store,
		state:   "browsing",
	}
}

// pluginItem represents a plugin in the list
type pluginItem struct {
	plugin StorePlugin
}

func (i pluginItem) Title() string {
	status := ""
	if i.plugin.Installed {
		if i.plugin.Enabled {
			status = " [✓]"
		} else {
			status = " [⊘]"
		}
	}
	return i.plugin.Name + status
}

func (i pluginItem) Description() string {
	return i.plugin.Description
}

func (i pluginItem) FilterValue() string {
	return i.plugin.Name + " " + i.plugin.Description + " " + i.plugin.Author
}

// Init initializes the store UI
func (s *StoreUI) Init() tea.Cmd {
	return tea.Batch(
		s.spinner.Tick,
		s.list.StartSpinner(),
	)
}

// Update handles UI updates
func (s *StoreUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch s.state {
		case "browsing":
			switch msg.String() {
			case "q", "ctrl+c":
				return s, tea.Quit
			case "/":
				s.search.Focus()
				return s, textinput.Blink
			case "r":
				s.state = "loading"
				return s, s.refreshStore()
			case "enter":
				if len(s.list.Items()) > 0 {
					if item, ok := s.list.SelectedItem().(pluginItem); ok {
						s.selected = &item.plugin
						s.state = "installing"
						return s, s.installPlugin(item.plugin)
					}
				}
			}
		case "installing":
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				s.state = "browsing"
				s.selected = nil
				return s, nil
			}
		}
	case tea.WindowSizeMsg:
		s.list.SetSize(msg.Width, msg.Height-2)
		s.search.Width = msg.Width - 4
	case refreshMsg:
		s.state = "browsing"
		s.updateList()
	case installMsg:
		s.state = "browsing"
		s.selected = nil
		if msg.err != nil {
			s.err = msg.err
		}
	}

	var cmd tea.Cmd
	switch s.state {
	case "loading":
		s.spinner, cmd = s.spinner.Update(msg)
	case "browsing":
		s.list, cmd = s.list.Update(msg)
		s.search, _ = s.search.Update(msg)
		if s.search.Focused() {
			s.filterList()
		}
	case "installing":
		s.spinner, cmd = s.spinner.Update(msg)
	}

	return s, cmd
}

// View renders the store UI
func (s *StoreUI) View() string {
	switch s.state {
	case "loading":
		return lipgloss.JoinVertical(
			lipgloss.Left,
			"Refreshing plugin store...",
			s.spinner.View(),
		)
	case "installing":
		if s.selected == nil {
			return "Installing plugin..."
		}
		return lipgloss.JoinVertical(
			lipgloss.Left,
			fmt.Sprintf("Installing %s...", s.selected.Name),
			s.spinner.View(),
			"Press q to cancel",
		)
	default:
		var view strings.Builder
		view.WriteString(s.search.View())
		view.WriteString("\n")
		view.WriteString(s.list.View())

		if s.err != nil {
			view.WriteString("\n")
			view.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("red")).Render("Error: " + s.err.Error()))
		}

		return view.String()
	}
}

// refreshMsg is sent when store refresh completes
type refreshMsg struct{}

// installMsg is sent when plugin installation completes
type installMsg struct {
	err error
}

// refreshStore refreshes the plugin store
func (s *StoreUI) refreshStore() tea.Cmd {
	return func() tea.Msg {
		_ = s.store.Refresh()
		return refreshMsg{}
	}
}

// installPlugin installs a plugin
func (s *StoreUI) installPlugin(plugin StorePlugin) tea.Cmd {
	return func() tea.Msg {
		// This would integrate with the plugin host to install the plugin
		// For now, just simulate installation with the actual plugin data
		time.Sleep(2 * time.Second)

		// Log the plugin being installed for debugging
		fmt.Printf("Installing plugin: %s v%s\n", plugin.Name, plugin.Version)

		// TODO: Integrate with plugin manager to actually install the plugin
		// This would call something like:
		// err := pluginManager.InstallPlugin(plugin.DownloadURL, plugin.Checksum)

		return installMsg{err: nil}
	}
}

// updateList updates the list with current plugins
func (s *StoreUI) updateList() {
	var items []list.Item
	for _, plugin := range s.store.GetPlugins() {
		items = append(items, pluginItem{plugin})
	}
	s.list.SetItems(items)
}

// filterList filters the list based on search input
func (s *StoreUI) filterList() {
	searchTerm := s.search.Value()
	filtered := s.store.FilterPlugins("", searchTerm, nil)

	var items []list.Item
	for _, plugin := range filtered {
		items = append(items, pluginItem{plugin})
	}
	s.list.SetItems(items)
}
