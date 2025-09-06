package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Cod-e-Codes/marchat/config"
	"github.com/Cod-e-Codes/marchat/server"
	"github.com/Cod-e-Codes/marchat/shared"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

// Multi-admin support
// Usage: --admin Cody --admin Alice --admin-key changeme
//
// Remove old --admin-username logic

type multiFlag []string

func (m *multiFlag) String() string       { return strings.Join(*m, ",") }
func (m *multiFlag) Set(val string) error { *m = append(*m, val); return nil }

var adminUsers multiFlag
var adminKey = flag.String("admin-key", "", "Admin key for privileged commands (deprecated, use MARCHAT_ADMIN_KEY)")
var port = flag.Int("port", 0, "Port to listen on (deprecated, use MARCHAT_PORT)")
var configPath = flag.String("config", "", "Path to server config file (JSON, deprecated)")
var configDir = flag.String("config-dir", "", "Configuration directory (default: ./config in dev, $XDG_CONFIG_HOME/marchat in prod)")
var enableAdminPanel = flag.Bool("admin-panel", false, "Enable the built-in admin panel TUI")

func printBanner(addr string, admins []string, scheme string) {
	fmt.Println(`
⢀⠀⠀⠀⠀⠀⠀⠀⢀⣠⣤⣶⣶⣶⣶⣶⣶⣶⣶⣶⣦⡀⠀⠀⠀⠀⠀⠀⣀⣀⣀⣀⣀⣀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀  
⣿⣷⠀⠀⣀⣤⣴⣾⣿⡿⣿⣧⣿⣶⣿⣿⣿⣽⣿⣽⣿⣷⣤⣤⣴⣶⣾⣿⣿⡿⠿⠛⠛⠿⣷⡀⢀⣀⣀⣀⣀⡀⠀⠀⠀⠀  
⠈⣿⣶⣿⣿⣛⣿⣶⣿⣿⣿⣿⣛⣿⣭⣿⣽⣿⣹⣿⣻⣿⡿⠿⠛⠛⠋⠉⠀⢀⣀⣀⣀⣀⣈⣿⠿⠿⠟⠻⠿⢿⡇⠀⠀⠀  
⠀⢹⣿⣿⣿⣿⡟⣿⣯⣿⣿⣿⣿⢿⣿⢻⣟⣻⡟⢿⠿⣿⣇⣄⣠⣤⣴⣶⣾⣿⡿⠿⠿⠻⠿⠇⠀⣀⣀⣀⣀⣸⡇⠀⠀⠀  
⠀⠀⢻⣿⣿⣿⣿⡿⣿⡛⣿⣥⣿⣿⣿⣿⢿⡿⣿⣿⣷⣿⣿⢿⣿⠿⠟⠋⠉⠀⢀⣀⣀⣀⣀⡘⣿⣿⠿⠿⠿⢿⡇⠀⠀⠀  
⠀⠀⠈⣿⡏⢿⣿⣷⣾⣿⡟⢿⣋⣿⣴⣿⣾⣷⣿⣷⣾⣾⣿⡆⠀⣀⣤⣤⣶⣾⣿⠿⠟⠛⠻⠿⣇⣀⣠⣤⣤⣼⡇⠀⠀⠀  
⠀⠀⠀⠸⣿⡀⢻⣿⣯⣸⣷⣾⡿⠟⠋⠉⠀⠀⠀⠀⠀⠀⠀⠘⣿⣿⠿⠟⠛⠉⢀⣠⣤⣤⣤⣥⣽⡿⠿⠿⠿⠿⡇⠀⠀⠀  
⠀⠀⠀⠀⢻⣷⠀⢻⣿⠟⠋⠁⠀⢀⣠⣤⣴⣶⣶⣶⣶⣾⣶⣾⡁⣀⣀⣤⣴⣾⣿⠿⠛⠋⠉⠉⢳⣤⣤⣤⣤⣤⣷⠀⠀⠀  
⠀⠀⠀⠀⠈⣿⣇⠀⣿⣀⣠⣴⣾⣿⡿⠟⠋⠉⠉⠀⠀⠀⠈⠉⣿⠿⠟⠛⠋⠁⢀⣤⣶⣶⣶⣶⣾⠟⠛⠋⠉⠉⢿⠀⠀⠀  
⠀⠀⠀⠀⠀⠘⣿⡆⠸⣿⡿⠟⠋⠁⢀⣀⣤⣴⣶⣶⣶⣶⣶⣾⣇⣀⣤⣤⣶⣾⡿⠛⠋⠉⠉⠉⠉⣤⣴⣶⣶⠶⢿⣇⠀⠀  
⠀⠀⠀⠀⠀⠀⢹⣿⡀⣿⡄⣀⣤⣾⡿⠟⠋⠉⠁⠀⠀⠀⠀⡿⠛⠛⠛⠉⠁⣀⣴⣾⣿⣿⣿⣿⡟⠉⠀⠀⣀⣀⣀⣿⡀⠀  
⠀⠀⠀⠀⠀⠀⠀⢿⣧⢸⣿⠟⠋⠁⣀⣠⣤⣴⣶⣾⣿⣿⣿⣿⣧⣤⣤⣶⠾⠟⠛⠉⢁⣀⣀⣀⣀⢰⣶⣿⠿⠿⠿⠿⣧⠀  
⠀⠀⠀⠀⠀⠀⠀⠈⣿⡆⣿⣠⣴⣿⣿⣿⠿⠟⠛⠉⠉⠀⠀⢠⡟⠋⠉⣀⣠⣴⣾⠿⠿⠟⠛⠛⠛⣿⠏⠀⠀⣀⣀⣀⣿⡄  
⠀⠀⠀⠀⠀⠀⠀⠀⠸⣿⣿⡿⠟⠋⠁⠀⠀⠀⠀⠀⠀⠀⠀⠸⣧⣶⠿⠛⠋⠁⠀⠀⠀⠀⠀⠀⠘⣿⣴⣾⠿⠛⠋⠉⠉⠁  
⠀⠀⠀⠀⠀⠀⠀⠀⠀⢻⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠉⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠁⠀⠀⠀⠀⠀⠀⠀  
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⣿⣆⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀  
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠸⣿⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀  
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠙⠃⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀  

░███     ░███                                ░██                      ░██    
░████   ░████                                ░██                      ░██    
░██░██ ░██░██  ░██████   ░██░████  ░███████  ░████████   ░██████   ░████████ 
░██ ░████ ░██       ░██  ░███     ░██    ░██ ░██    ░██       ░██     ░██    
░██  ░██  ░██  ░███████  ░██      ░██        ░██    ░██  ░███████     ░██    
░██       ░██ ░██   ░██  ░██      ░██    ░██ ░██    ░██ ░██   ░██     ░██    
░██       ░██  ░█████░██ ░██       ░███████  ░██    ░██  ░█████░██     ░████ `)
	fmt.Println()
	fmt.Printf("\U0001F310 WebSocket: %s://%s/ws\n", scheme, addr)
	fmt.Printf("\U0001F511 Admins: %s\n", strings.Join(admins, ", "))
	fmt.Printf("\U0001F4E6 Version: %s\n", shared.GetServerVersionInfo())
	fmt.Println("\U0001F4A1 Tip: Use --username <admin> --admin --admin-key <key> to connect as admin")
}

func main() {
	flag.Var(&adminUsers, "admin", "[DEPRECATED] Admin username (use MARCHAT_USERS env var instead)")
	flag.Parse()

	// Check for MARCHAT_CONFIG_DIR environment variable first
	configDir := *configDir
	if envConfigDir := os.Getenv("MARCHAT_CONFIG_DIR"); envConfigDir != "" {
		configDir = envConfigDir
	}

	// Load configuration from environment variables and .env files
	cfg, err := config.LoadConfig(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nConfiguration options:\n")
		fmt.Fprintf(os.Stderr, "  Environment variables:\n")
		fmt.Fprintf(os.Stderr, "    MARCHAT_PORT=8080 (default: 8080)\n")
		fmt.Fprintf(os.Stderr, "    MARCHAT_ADMIN_KEY=your-secret-key (required)\n")
		fmt.Fprintf(os.Stderr, "    MARCHAT_USERS=user1,user2,user3 (comma-separated, required)\n")
		fmt.Fprintf(os.Stderr, "    MARCHAT_DB_PATH=/path/to/db (default: $CONFIG_DIR/marchat.db)\n")
		fmt.Fprintf(os.Stderr, "    MARCHAT_LOG_LEVEL=info (default: info)\n")
		fmt.Fprintf(os.Stderr, "    MARCHAT_JWT_SECRET=your-jwt-secret (default: auto-generated)\n")
		fmt.Fprintf(os.Stderr, "    MARCHAT_TLS_CERT_FILE=/path/to/cert.pem (optional)\n")
		fmt.Fprintf(os.Stderr, "    MARCHAT_TLS_KEY_FILE=/path/to/key.pem (optional)\n")
		fmt.Fprintf(os.Stderr, "    MARCHAT_CONFIG_DIR=/path/to/config (optional)\n")
		fmt.Fprintf(os.Stderr, "    MARCHAT_BAN_HISTORY_GAPS=true (optional, default: true)\n")
		fmt.Fprintf(os.Stderr, "    MARCHAT_PLUGIN_REGISTRY_URL=url (optional, default: GitHub registry)\n")
		fmt.Fprintf(os.Stderr, "  .env file: Create %s/.env with the above variables\n", cfg.ConfigDir)
		fmt.Fprintf(os.Stderr, "  Config directory: Use --config-dir or MARCHAT_CONFIG_DIR to specify custom location\n")
		os.Exit(1)
	}

	// Warn about deprecated flags
	if len(adminUsers) > 0 {
		fmt.Fprintln(os.Stderr, "[WARNING] --admin flag is deprecated. Use MARCHAT_USERS environment variable.")
	}
	if *adminKey != "" {
		fmt.Fprintln(os.Stderr, "[WARNING] --admin-key flag is deprecated. Use MARCHAT_ADMIN_KEY environment variable.")
	}
	if *port != 0 {
		fmt.Fprintln(os.Stderr, "[WARNING] --port flag is deprecated. Use MARCHAT_PORT environment variable.")
	}
	if *configPath != "" {
		fmt.Fprintln(os.Stderr, "[WARNING] --config flag is deprecated. Use environment variables or .env file.")
	}

	// Override with deprecated flags if provided (for backward compatibility)
	admins := cfg.Admins
	key := cfg.AdminKey
	listenPort := cfg.Port

	if len(adminUsers) > 0 {
		admins = make([]string, len(adminUsers))
		copy(admins, adminUsers)
	}
	if *adminKey != "" {
		key = *adminKey
	}
	if *port != 0 {
		listenPort = *port
	}

	// Final validation
	if len(admins) == 0 {
		log.Fatal("At least one admin username is required (set MARCHAT_USERS or use --admin flag).")
	}
	if key == "" {
		log.Fatal("admin_key is required (set MARCHAT_ADMIN_KEY or use --admin-key flag).")
	}
	if listenPort < 1 || listenPort > 65535 {
		log.Fatal("Port must be between 1 and 65535.")
	}

	// Normalize admin usernames to lowercase and check for duplicates
	adminSet := make(map[string]struct{})
	for _, u := range admins {
		lu := strings.ToLower(u)
		if _, exists := adminSet[lu]; exists {
			log.Fatalf("Duplicate admin username (case-insensitive): %s", u)
		}
		adminSet[lu] = struct{}{}
	}
	admins = make([]string, 0, len(adminSet))
	for u := range adminSet {
		admins = append(admins, u)
	}

	// Initialize database with the configured path
	db := server.InitDB(cfg.DBPath)
	server.CreateSchema(db)

	// Set up plugin directories
	pluginDir := cfg.ConfigDir + "/plugins"
	dataDir := cfg.ConfigDir + "/data"

	// Get registry URL from configuration
	registryURL := cfg.PluginRegistryURL

	// Create plugin directories if they don't exist
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		log.Printf("Warning: Failed to create plugin directory %s: %v", pluginDir, err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("Warning: Failed to create data directory %s: %v", dataDir, err)
	}

	hub := server.NewHub(pluginDir, dataDir, registryURL, db)
	go hub.Run()

	// Initialize admin panel if enabled (but don't start it yet)
	var adminPanelReady bool
	if *enableAdminPanel {
		adminPanelReady = true
	}

	http.HandleFunc("/ws", server.ServeWs(hub, db, admins, key, cfg.BanGapsHistory))

	// Initialize health checker
	healthChecker := server.NewHealthChecker(hub, db, shared.GetServerVersionInfo())
	http.HandleFunc("/health", healthChecker.HealthCheckHandler)
	http.HandleFunc("/health/simple", healthChecker.SimpleHealthHandler)

	addr := fmt.Sprintf(":%d", listenPort)
	serverAddr := fmt.Sprintf("localhost:%d", listenPort)
	scheme := cfg.GetWebSocketScheme()

	log.Printf("marchat WebSocket server running on %s", addr)
	log.Printf("Configuration directory: %s", cfg.ConfigDir)
	log.Printf("Database path: %s", cfg.DBPath)
	if cfg.IsTLSEnabled() {
		log.Printf("TLS enabled with cert: %s, key: %s", cfg.TLSCertFile, cfg.TLSKeyFile)
	} else {
		log.Printf("TLS disabled - running in HTTP mode")
	}
	printBanner(serverAddr, admins, scheme)
	if adminPanelReady {
		fmt.Println("\U0001F4BB Admin Panel: Press Ctrl+A to open admin panel, Ctrl+C to shutdown")
	}

	// Create a custom server instance
	srv := &http.Server{Addr: addr}

	// Channel to listen for OS signals (Ctrl+C, etc.)
	stop := make(chan os.Signal, 1)
	adminShutdown := make(chan bool, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Run server in a goroutine
	go func() {
		var err error
		if cfg.IsTLSEnabled() {
			log.Printf("Starting server with TLS on %s", addr)
			err = srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			log.Printf("Starting server without TLS on %s", addr)
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Start admin panel hotkey listener
	if adminPanelReady {
		go func() {
			// Check if stdin is a terminal
			fd := os.Stdin.Fd()
			if !term.IsTerminal(fd) {
				log.Printf("Warning: stdin is not a terminal, admin panel hotkeys disabled")
				return
			}

			// Set terminal to raw mode for character-by-character input
			oldState, err := term.MakeRaw(fd)
			if err != nil {
				log.Printf("Warning: Could not set terminal to raw mode: %v", err)
				return
			}

			// Ensure we restore terminal state when the goroutine exits
			defer func() {
				if err := term.Restore(fd, oldState); err != nil {
					log.Printf("Warning: Could not restore terminal state: %v", err)
				}
			}()

			log.Printf("Admin panel ready - press Ctrl+A to open")

			// Read input character by character
			buf := make([]byte, 1)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil {
					log.Printf("Error reading from stdin: %v", err)
					break
				}
				if n == 0 {
					continue
				}

				// Check for Ctrl+A (ASCII 1) or Ctrl+C (ASCII 3)
				if buf[0] == 1 {
					log.Printf("Admin panel hotkey detected (Ctrl+A)")

					// Temporarily restore terminal state
					if err := term.Restore(fd, oldState); err != nil {
						log.Printf("Warning: Could not restore terminal state: %v", err)
					}

					// Launch admin panel
					pluginManager := hub.GetPluginManager()
					panel := server.NewAdminPanel(hub, db, pluginManager, cfg.ConfigDir, cfg.DBPath, listenPort)
					p := tea.NewProgram(panel, tea.WithAltScreen())
					if _, err := p.Run(); err != nil {
						log.Printf("Admin panel error: %v", err)
					}

					// Set terminal back to raw mode
					oldState, err = term.MakeRaw(fd)
					if err != nil {
						log.Printf("Warning: Could not reset terminal to raw mode: %v", err)
						break
					}
					log.Printf("Admin panel ready - press Ctrl+A to open")
				} else if buf[0] == 3 { // Ctrl+C (ASCII 3)
					log.Printf("Ctrl+C detected, shutting down server...")

					// Restore terminal state before shutdown
					if err := term.Restore(fd, oldState); err != nil {
						log.Printf("Warning: Could not restore terminal state: %v", err)
					}

					// Signal shutdown via our channel
					adminShutdown <- true
					return
				}
			}
		}()
	}

	// Block until we receive SIGINT (Ctrl+C) or admin shutdown
	select {
	case <-stop:
		log.Println("Shutting down server gracefully...")
	case <-adminShutdown:
		log.Println("Shutting down server gracefully...")
	}

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Graceful shutdown failed: %v", err)
	}

	log.Println("Server shut down cleanly")
}
