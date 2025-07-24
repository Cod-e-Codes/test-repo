package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"marchat/server"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

// Multi-admin support
// Usage: --admin Cody --admin Alice --admin-key changeme
//
// Remove old --admin-username logic

type multiFlag []string

func (m *multiFlag) String() string       { return strings.Join(*m, ",") }
func (m *multiFlag) Set(val string) error { *m = append(*m, val); return nil }

var adminUsers multiFlag
var adminKey = flag.String("admin-key", "", "Admin key for privileged commands (required)")
var port = flag.Int("port", 9090, "Port to listen on (default 9090)")
var configPath = flag.String("config", "server_config.json", "Path to server config file (JSON)")

func printBanner(addr string, admins []string) {
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
⠀⠀⠀⠀⠀⠀⢹⣿⡀⣿⡄⣀⣤⣾⡿⠟⠋⠉⠉⠁⠀⠀⠀⠀⡿⠛⠛⠛⠉⠁⣀⣴⣾⣿⣿⣿⣿⡟⠉⠀⠀⣀⣀⣿⡀⠀  
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
	fmt.Printf("\U0001F310 WebSocket: wss://%s/ws\n", addr)
	fmt.Printf("\U0001F511 Admins: %s\n", strings.Join(admins, ", "))
	fmt.Println("\U0001F4A1 Tip: Use --username <admin> --admin --admin-key <key> to connect as admin")
}

func main() {
	flag.Var(&adminUsers, "admin", "[DEPRECATED] Admin username (use config file instead)")
	flag.Parse()

	cfg, err := server.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please create a server_config.json (see example in repo).\n")
		os.Exit(1)
	}

	// Warn if flags are used
	if len(adminUsers) > 0 {
		fmt.Fprintln(os.Stderr, "[WARNING] --admin flag is deprecated. Use admins in config file.")
	}
	if *adminKey != "" {
		fmt.Fprintln(os.Stderr, "[WARNING] --admin-key flag is deprecated. Use admin_key in config file.")
	}
	if *port != 9090 {
		fmt.Fprintln(os.Stderr, "[WARNING] --port flag is deprecated. Use port in config file.")
	}

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
	if *port != 9090 {
		listenPort = *port
	}

	if len(admins) == 0 {
		log.Fatal("At least one admin username is required (set in config file or --admin flag).")
	}
	if key == "" {
		log.Fatal("admin_key is required (set in config file or --admin-key flag).")
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

	db := server.InitDB("chat.db")
	server.CreateSchema(db)

	hub := server.NewHub()
	go hub.Run()

	http.HandleFunc("/ws", server.ServeWs(hub, db, admins, key))

	addr := fmt.Sprintf(":%d", listenPort)
	serverAddr := fmt.Sprintf("localhost:%d", listenPort)
	log.Printf("marchat WebSocket server running on %s", addr)
	printBanner(serverAddr, admins)

	// Create a custom server instance
	srv := &http.Server{Addr: addr}

	// Channel to listen for OS signals (Ctrl+C, etc.)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Run server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Block until we receive SIGINT (Ctrl+C)
	<-stop
	log.Println("Shutting down server gracefully...")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Graceful shutdown failed: %v", err)
	}

	log.Println("Server shut down cleanly")
}
