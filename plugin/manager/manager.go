package manager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cod-e-Codes/marchat/plugin/host"
	"github.com/Cod-e-Codes/marchat/plugin/sdk"
	"github.com/Cod-e-Codes/marchat/plugin/store"
)

// PluginManager manages plugin installation and commands
type PluginManager struct {
	host        *host.PluginHost
	store       *store.Store
	pluginDir   string
	dataDir     string
	registryURL string
}

// NewPluginManager creates a new plugin manager
func NewPluginManager(pluginDir, dataDir, registryURL string) *PluginManager {
	host := host.NewPluginHost(pluginDir, dataDir)
	store := store.NewStore(registryURL, dataDir)

	return &PluginManager{
		host:        host,
		store:       store,
		pluginDir:   pluginDir,
		dataDir:     dataDir,
		registryURL: registryURL,
	}
}

// InstallPlugin installs a plugin from the store using the current platform
func (pm *PluginManager) InstallPlugin(name string) error {
	return pm.InstallPluginWithPlatform(name, "", "")
}

// InstallPluginWithPlatform installs a plugin selecting a specific os/arch if provided.
// When osName or arch are empty, the current runtime platform is used for selection.
func (pm *PluginManager) InstallPluginWithPlatform(name, osName, arch string) error {
	// Get plugin from store
	plugin := pm.store.ResolvePlugin(name, osName, arch)
	if plugin == nil {
		return fmt.Errorf("plugin %s not found in store", name)
	}

	// Create plugin directory
	pluginPath := filepath.Join(pm.pluginDir, name)
	if err := os.MkdirAll(pluginPath, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Download plugin
	if err := pm.downloadPlugin(plugin, pluginPath); err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	// Validate checksum if provided
	if plugin.Checksum != "" {
		if err := pm.validateChecksum(pluginPath, plugin.Checksum); err != nil {
			return fmt.Errorf("checksum validation failed: %w", err)
		}
	}

	// Load plugin into host
	if err := pm.host.LoadPlugin(name); err != nil {
		return fmt.Errorf("failed to load plugin: %w", err)
	}

	// Start plugin
	if err := pm.host.StartPlugin(name); err != nil {
		return fmt.Errorf("failed to start plugin: %w", err)
	}

	return nil
}

// UninstallPlugin removes a plugin
func (pm *PluginManager) UninstallPlugin(name string) error {
	// Stop plugin if running
	if err := pm.host.StopPlugin(name); err != nil {
		return fmt.Errorf("failed to stop plugin: %w", err)
	}

	// Remove plugin directory
	pluginPath := filepath.Join(pm.pluginDir, name)
	if err := os.RemoveAll(pluginPath); err != nil {
		return fmt.Errorf("failed to remove plugin directory: %w", err)
	}

	// Remove data directory
	dataPath := filepath.Join(pm.dataDir, name)
	if err := os.RemoveAll(dataPath); err != nil {
		return fmt.Errorf("failed to remove plugin data: %w", err)
	}

	return nil
}

// EnablePlugin enables a plugin
func (pm *PluginManager) EnablePlugin(name string) error {
	return pm.host.EnablePlugin(name)
}

// DisablePlugin disables a plugin
func (pm *PluginManager) DisablePlugin(name string) error {
	return pm.host.DisablePlugin(name)
}

// ListPlugins returns all installed plugins
func (pm *PluginManager) ListPlugins() map[string]*host.PluginInstance {
	return pm.host.ListPlugins()
}

// GetPlugin returns a specific plugin
func (pm *PluginManager) GetPlugin(name string) *host.PluginInstance {
	return pm.host.GetPlugin(name)
}

// ExecuteCommand executes a plugin command
func (pm *PluginManager) ExecuteCommand(pluginName, command string, args []string) error {
	return pm.host.ExecuteCommand(pluginName, command, args)
}

// SendMessage sends a message to all enabled plugins
func (pm *PluginManager) SendMessage(msg sdk.Message) {
	pm.host.SendMessage(msg)
}

// GetMessageChannel returns the channel for receiving messages from plugins
func (pm *PluginManager) GetMessageChannel() <-chan sdk.Message {
	return pm.host.GetMessageChannel()
}

// UpdateUserList updates the user list for plugins
func (pm *PluginManager) UpdateUserList(users []string) {
	pm.host.UpdateUserList(users)
}

// RefreshStore refreshes the plugin store
func (pm *PluginManager) RefreshStore() error {
	return pm.store.Refresh()
}

// LoadStoreFromCache loads the store from cache
func (pm *PluginManager) LoadStoreFromCache() error {
	return pm.store.LoadFromCache()
}

// GetStore returns the plugin store
func (pm *PluginManager) GetStore() *store.Store {
	return pm.store
}

// downloadPlugin downloads a plugin from the given URL
func (pm *PluginManager) downloadPlugin(plugin *store.StorePlugin, pluginPath string) error {
	var reader io.Reader

	if strings.HasPrefix(plugin.DownloadURL, "file://") {
		// Handle local file URLs
		filePath := strings.TrimPrefix(plugin.DownloadURL, "file://")
		filePath = strings.TrimPrefix(filePath, "/")
		filePath = strings.ReplaceAll(filePath, "/", "\\")

		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open local plugin file: %w", err)
		}
		defer file.Close()
		reader = file
	} else {
		// Handle HTTP URLs
		resp, err := http.Get(plugin.DownloadURL)
		if err != nil {
			return fmt.Errorf("failed to download plugin: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("download failed with status %d", resp.StatusCode)
		}
		reader = resp.Body
	}

	// Determine file type and extract
	if strings.HasSuffix(plugin.DownloadURL, ".zip") {
		return pm.extractZip(reader, pluginPath)
	} else if strings.HasSuffix(plugin.DownloadURL, ".tar.gz") || strings.HasSuffix(plugin.DownloadURL, ".tgz") {
		return pm.extractTarGz(reader, pluginPath)
	} else {
		// Assume it's a single binary
		return pm.downloadBinary(reader, pluginPath, plugin.Name)
	}
}

// isPathSafe validates that a path doesn't contain directory traversal elements
func isPathSafe(path string) bool {
	// Check for directory traversal attempts
	if strings.Contains(path, "..") {
		return false
	}

	// Check for absolute paths
	if filepath.IsAbs(path) {
		return false
	}

	// Check for paths that start with common problematic patterns
	cleanPath := filepath.Clean(path)
	if strings.HasPrefix(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") {
		return false
	}

	return true
}

// extractZip extracts a zip file
func (pm *PluginManager) extractZip(reader io.Reader, pluginPath string) error {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "plugin-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write to temp file
	if _, err := io.Copy(tmpFile, reader); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Open zip file
	zipReader, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer zipReader.Close()

	// Extract files
	for _, file := range zipReader.File {
		// Validate path to prevent zip slip attacks
		if !isPathSafe(file.Name) {
			return fmt.Errorf("unsafe file path in archive: %s", file.Name)
		}

		filePath := filepath.Join(pluginPath, file.Name)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Extract file
		fileReader, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open file in zip: %w", err)
		}

		fileWriter, err := os.Create(filePath)
		if err != nil {
			fileReader.Close()
			return fmt.Errorf("failed to create file: %w", err)
		}

		if _, err := io.Copy(fileWriter, fileReader); err != nil {
			fileReader.Close()
			fileWriter.Close()
			return fmt.Errorf("failed to copy file: %w", err)
		}

		fileReader.Close()
		fileWriter.Close()

		// Make executable if it's the main binary
		if strings.HasSuffix(file.Name, filepath.Base(pluginPath)) {
			if err := os.Chmod(filePath, 0755); err != nil {
				return fmt.Errorf("failed to make executable: %w", err)
			}
		}
	}

	return nil
}

// extractTarGz extracts a tar.gz file
func (pm *PluginManager) extractTarGz(reader io.Reader, pluginPath string) error {
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Validate path to prevent zip slip attacks
		if !isPathSafe(header.Name) {
			return fmt.Errorf("unsafe file path in archive: %s", header.Name)
		}

		filePath := filepath.Join(pluginPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Create file
			file, err := os.Create(filePath)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return fmt.Errorf("failed to copy file: %w", err)
			}
			file.Close()

			// Make executable if it's the main binary
			if strings.HasSuffix(header.Name, filepath.Base(pluginPath)) {
				if err := os.Chmod(filePath, 0755); err != nil {
					return fmt.Errorf("failed to make executable: %w", err)
				}
			}
		}
	}

	return nil
}

// downloadBinary downloads a single binary file
func (pm *PluginManager) downloadBinary(reader io.Reader, pluginPath, pluginName string) error {
	binaryPath := filepath.Join(pluginPath, pluginName)

	// Create binary file
	file, err := os.Create(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to create binary file: %w", err)
	}
	defer file.Close()

	// Copy data
	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return fmt.Errorf("failed to make executable: %w", err)
	}

	return nil
}

// validateChecksum validates the checksum of a downloaded plugin
func (pm *PluginManager) validateChecksum(pluginPath, expectedChecksum string) error {
	// Calculate SHA256 of the plugin binary
	pluginName := filepath.Base(pluginPath)

	// Try different possible binary names
	binaryNames := []string{
		pluginName,
		pluginName + ".exe",
		pluginName + ".bat",
	}

	var binaryPath string
	var file *os.File
	var err error

	for _, name := range binaryNames {
		binaryPath = filepath.Join(pluginPath, name)
		file, err = os.Open(binaryPath)
		if err == nil {
			break
		}
	}

	if file == nil {
		return fmt.Errorf("failed to open binary for checksum: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	calculatedChecksum := hex.EncodeToString(hash.Sum(nil))

	// Handle both formats: just hash or "sha256:hash"
	expectedHash := expectedChecksum
	if strings.HasPrefix(expectedChecksum, "sha256:") {
		expectedHash = strings.TrimPrefix(expectedChecksum, "sha256:")
	}

	if calculatedChecksum != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, calculatedChecksum)
	}

	return nil
}

// GetPluginCommands returns all available plugin commands
func (pm *PluginManager) GetPluginCommands() map[string][]sdk.PluginCommand {
	commands := make(map[string][]sdk.PluginCommand)

	for name, instance := range pm.host.ListPlugins() {
		if instance.Manifest != nil {
			commands[name] = instance.Manifest.Commands
		}
	}

	return commands
}

// GetPluginManifest returns the manifest for a plugin
func (pm *PluginManager) GetPluginManifest(name string) *sdk.PluginManifest {
	instance := pm.host.GetPlugin(name)
	if instance == nil {
		return nil
	}
	return instance.Manifest
}
