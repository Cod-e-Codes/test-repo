package host

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Cod-e-Codes/marchat/plugin/sdk"
)

// PluginHost manages the lifecycle and communication with plugins
type PluginHost struct {
	plugins     map[string]*PluginInstance
	pluginDir   string
	dataDir     string
	mu          sync.RWMutex
	messageChan chan sdk.Message
	userList    []string
}

// PluginInstance represents a running plugin
type PluginInstance struct {
	Name     string
	Manifest *sdk.PluginManifest
	Process  *exec.Cmd
	Stdin    io.WriteCloser
	Stdout   io.ReadCloser
	Stderr   io.ReadCloser
	Config   sdk.Config
	Enabled  bool
	mu       sync.Mutex
}

// NewPluginHost creates a new plugin host
func NewPluginHost(pluginDir, dataDir string) *PluginHost {
	return &PluginHost{
		plugins:     make(map[string]*PluginInstance),
		pluginDir:   pluginDir,
		dataDir:     dataDir,
		messageChan: make(chan sdk.Message, 100),
	}
}

// LoadPlugin loads a plugin from the plugin directory
func (h *PluginHost) LoadPlugin(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	pluginPath := filepath.Join(h.pluginDir, name)
	manifestPath := filepath.Join(pluginPath, "plugin.json")

	// Read and validate manifest
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read plugin manifest: %w", err)
	}

	var manifest sdk.PluginManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("failed to parse plugin manifest: %w", err)
	}

	if err := sdk.ValidateManifest(&manifest); err != nil {
		return fmt.Errorf("invalid plugin manifest: %w", err)
	}

	// Check if plugin binary exists (with .exe extension on Windows)
	binaryName := name
	if filepath.Ext(name) == "" {
		binaryName = name + ".exe"
	}
	binaryPath := filepath.Join(pluginPath, binaryName)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Try without .exe extension as fallback
		binaryPath = filepath.Join(pluginPath, name)
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			return fmt.Errorf("plugin binary not found: %s", binaryPath)
		}
	}

	// Create plugin instance
	instance := &PluginInstance{
		Name:     name,
		Manifest: &manifest,
		Config: sdk.Config{
			PluginDir: pluginPath,
			DataDir:   filepath.Join(h.dataDir, name),
			Settings:  make(map[string]string),
		},
		Enabled: true,
	}

	h.plugins[name] = instance
	return nil
}

// StartPlugin starts a plugin subprocess
func (h *PluginHost) StartPlugin(name string) error {
	h.mu.RLock()
	instance, exists := h.plugins[name]
	h.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	if instance.Process != nil {
		return fmt.Errorf("plugin %s is already running", name)
	}

	// Create plugin data directory
	if err := os.MkdirAll(instance.Config.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin data directory: %w", err)
	}

	// Start plugin subprocess
	binaryName := name
	if filepath.Ext(name) == "" {
		binaryName = name + ".exe"
	}
	binaryPath := filepath.Join(instance.Config.PluginDir, binaryName)

	// Check if the .exe version exists, otherwise use the original name
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		binaryPath = filepath.Join(instance.Config.PluginDir, name)
	}

	log.Printf("[DEBUG] Starting plugin %s with binary path: %s", name, binaryPath)
	log.Printf("[DEBUG] Plugin directory: %s", instance.Config.PluginDir)

	// Use absolute path for the executable
	absBinaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	cmd := exec.CommandContext(context.Background(), absBinaryPath)
	cmd.Dir = instance.Config.PluginDir

	// Set up pipes for communication
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	instance.Process = cmd
	instance.Stdin = stdin
	instance.Stdout = stdout
	instance.Stderr = stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		// Provide clearer error on platform mismatch
		errMsg := strings.ToLower(err.Error())
		expected := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
		if strings.Contains(errMsg, "exec format error") ||
			strings.Contains(errMsg, "not a valid win32 application") ||
			strings.Contains(errMsg, "wrong architecture") {
			return fmt.Errorf("plugin %s binary is not compatible with this host (%s). Install the matching build for your platform or use :install %s --os %s --arch %s. underlying error: %v", name, expected, name, runtime.GOOS, runtime.GOARCH, err)
		}
		return fmt.Errorf("failed to start plugin %s: %w", name, err)
	}

	// Initialize plugin
	if err := h.initializePlugin(instance); err != nil {
		if stopErr := h.StopPlugin(name); stopErr != nil {
			log.Printf("Failed to stop plugin %s after initialization error: %v", name, stopErr)
		}
		return fmt.Errorf("failed to initialize plugin %s: %w", name, err)
	}

	// Start communication goroutines
	go h.handlePluginOutput(instance)
	go h.handlePluginErrors(instance)

	log.Printf("Plugin %s started successfully", name)
	return nil
}

// StopPlugin stops a plugin subprocess
func (h *PluginHost) StopPlugin(name string) error {
	h.mu.RLock()
	instance, exists := h.plugins[name]
	h.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	if instance.Process == nil {
		return nil // Already stopped
	}

	// Send shutdown request
	shutdownReq := sdk.PluginRequest{
		Type: "shutdown",
	}
	if err := h.sendRequest(instance, shutdownReq); err != nil {
		log.Printf("Failed to send shutdown request to plugin %s: %v", name, err)
	}

	// Wait for graceful shutdown with timeout
	done := make(chan error, 1)
	go func() {
		done <- instance.Process.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("Plugin %s exited with error: %v", name, err)
		}
	case <-time.After(5 * time.Second):
		// Force kill if graceful shutdown fails
		if err := instance.Process.Process.Kill(); err != nil {
			log.Printf("Failed to force kill plugin %s: %v", name, err)
		}
	}

	instance.Process = nil
	instance.Stdin = nil
	instance.Stdout = nil
	instance.Stderr = nil

	log.Printf("Plugin %s stopped", name)
	return nil
}

// EnablePlugin enables a plugin
func (h *PluginHost) EnablePlugin(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	instance, exists := h.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	instance.Enabled = true
	return h.StartPlugin(name)
}

// DisablePlugin disables a plugin
func (h *PluginHost) DisablePlugin(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	instance, exists := h.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	instance.Enabled = false
	return h.StopPlugin(name)
}

// GetPlugin returns a plugin instance
func (h *PluginHost) GetPlugin(name string) *PluginInstance {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.plugins[name]
}

// ListPlugins returns all loaded plugins
func (h *PluginHost) ListPlugins() map[string]*PluginInstance {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make(map[string]*PluginInstance)
	for name, instance := range h.plugins {
		result[name] = instance
	}
	return result
}

// SendMessage sends a message to all enabled plugins
func (h *PluginHost) SendMessage(msg sdk.Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for name, instance := range h.plugins {
		if !instance.Enabled || instance.Process == nil {
			continue
		}

		req := sdk.PluginRequest{
			Type: "message",
			Data: mustMarshal(msg),
		}

		if err := h.sendRequest(instance, req); err != nil {
			log.Printf("Failed to send message to plugin %s: %v", name, err)
		}
	}
}

// ExecuteCommand executes a plugin command
func (h *PluginHost) ExecuteCommand(pluginName, command string, args []string) error {
	h.mu.RLock()
	instance, exists := h.plugins[pluginName]
	h.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin %s not found", pluginName)
	}

	if !instance.Enabled || instance.Process == nil {
		return fmt.Errorf("plugin %s is not running", pluginName)
	}

	req := sdk.PluginRequest{
		Type:    "command",
		Command: command,
		Data:    mustMarshal(args),
	}

	return h.sendRequest(instance, req)
}

// UpdateUserList updates the list of online users
func (h *PluginHost) UpdateUserList(users []string) {
	h.mu.Lock()
	h.userList = users
	h.mu.Unlock()
}

// initializePlugin sends an initialization request to the plugin
func (h *PluginHost) initializePlugin(instance *PluginInstance) error {
	log.Printf("[DEBUG] Initializing plugin %s", instance.Name)

	initData := map[string]interface{}{
		"config": instance.Config,
	}

	initRequest := sdk.PluginRequest{
		Type: "init",
		Data: mustMarshal(initData),
	}

	log.Printf("[DEBUG] Sending init request to plugin %s", instance.Name)
	if err := h.sendRequest(instance, initRequest); err != nil {
		return fmt.Errorf("failed to send init request: %w", err)
	}

	log.Printf("[DEBUG] Plugin %s initialized successfully", instance.Name)
	return nil
}

// sendRequest sends a request to a plugin
func (h *PluginHost) sendRequest(instance *PluginInstance, req sdk.PluginRequest) error {
	if instance.Stdin == nil {
		return fmt.Errorf("plugin stdin is not available")
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("[DEBUG] Sending request to plugin %s: %s", instance.Name, string(data))

	// Send request with newline delimiter
	data = append(data, '\n')
	_, err = instance.Stdin.Write(data)
	if err != nil {
		log.Printf("[DEBUG] Failed to write to plugin %s stdin: %v", instance.Name, err)
		return err
	}

	log.Printf("[DEBUG] Successfully sent request to plugin %s", instance.Name)
	return nil
}

// handlePluginOutput handles stdout from a plugin
func (h *PluginHost) handlePluginOutput(instance *PluginInstance) {
	log.Printf("[DEBUG] Starting output handler for plugin %s", instance.Name)
	decoder := json.NewDecoder(instance.Stdout)
	for {
		var response sdk.PluginResponse
		if err := decoder.Decode(&response); err != nil {
			if err == io.EOF {
				log.Printf("[DEBUG] Plugin %s stdout closed", instance.Name)
				break
			}
			log.Printf("Failed to decode plugin %s response: %v", instance.Name, err)
			continue
		}

		log.Printf("[DEBUG] Received response from plugin %s: %+v", instance.Name, response)
		h.handlePluginResponse(instance, response)
	}
}

// handlePluginErrors handles stderr from a plugin
func (h *PluginHost) handlePluginErrors(instance *PluginInstance) {
	scanner := json.NewDecoder(instance.Stderr)
	for {
		var logEntry struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		}
		if err := scanner.Decode(&logEntry); err != nil {
			if err == io.EOF {
				break
			}
			break
		}
		log.Printf("[Plugin %s] %s: %s", instance.Name, logEntry.Level, logEntry.Message)
	}
}

// handlePluginResponse handles responses from plugins
func (h *PluginHost) handlePluginResponse(instance *PluginInstance, response sdk.PluginResponse) {
	switch response.Type {
	case "message":
		if response.Success {
			var msg sdk.Message
			if err := json.Unmarshal(response.Data, &msg); err != nil {
				log.Printf("Failed to unmarshal plugin message: %v", err)
				return
			}
			// Send message to chat
			select {
			case h.messageChan <- msg:
			default:
				log.Printf("Message channel full, dropping message from plugin %s", instance.Name)
			}
		}
	case "log":
		if !response.Success {
			log.Printf("Plugin %s error: %s", instance.Name, response.Error)
		}
	}
}

// GetMessageChannel returns the channel for receiving messages from plugins
func (h *PluginHost) GetMessageChannel() <-chan sdk.Message {
	return h.messageChan
}

// mustMarshal is a helper function that panics on marshal error
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return data
}
