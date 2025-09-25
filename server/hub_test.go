package server

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestNewHub(t *testing.T) {
	// Create a test database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	hub := NewHub("./plugins", "./data", "http://registry.example.com", db)

	if hub == nil {
		t.Fatal("NewHub returned nil")
	}

	if hub.clients == nil {
		t.Error("clients map should not be nil")
	}

	if hub.broadcast == nil {
		t.Error("broadcast channel should not be nil")
	}

	if hub.register == nil {
		t.Error("register channel should not be nil")
	}

	if hub.unregister == nil {
		t.Error("unregister channel should not be nil")
	}

	if hub.bans == nil {
		t.Error("bans map should not be nil")
	}

	if hub.tempKicks == nil {
		t.Error("tempKicks map should not be nil")
	}

	if hub.pluginManager == nil {
		t.Error("pluginManager should not be nil")
	}

	if hub.pluginCommandHandler == nil {
		t.Error("pluginCommandHandler should not be nil")
	}

	if hub.db != db {
		t.Error("database reference should be set correctly")
	}
}

func TestHubBanUser(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create schema for database operations
	CreateSchema(db)

	hub := NewHub("./plugins", "./data", "http://registry.example.com", db)

	username := "testuser"
	adminUsername := "admin"

	// Test banning a user
	hub.BanUser(username, adminUsername)

	// Check if user is banned
	if !hub.IsUserBanned(username) {
		t.Error("User should be banned")
	}

	// Check case insensitive
	if !hub.IsUserBanned(strings.ToUpper(username)) {
		t.Error("Ban should be case insensitive")
	}

	// Check that ban is permanent (should not expire automatically)
	time.Sleep(100 * time.Millisecond) // Small delay
	if !hub.IsUserBanned(username) {
		t.Error("Permanent ban should not expire")
	}
}

func TestHubUnbanUser(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create schema for database operations
	CreateSchema(db)

	hub := NewHub("./plugins", "./data", "http://registry.example.com", db)

	username := "testuser"
	adminUsername := "admin"

	// First ban the user
	hub.BanUser(username, adminUsername)
	if !hub.IsUserBanned(username) {
		t.Error("User should be banned")
	}

	// Now unban the user
	unbanned := hub.UnbanUser(username, adminUsername)
	if !unbanned {
		t.Error("Unban should return true for existing ban")
	}

	// Check if user is unbanned
	if hub.IsUserBanned(username) {
		t.Error("User should not be banned after unban")
	}

	// Test unbanning non-existent user
	unbanned = hub.UnbanUser("nonexistent", adminUsername)
	if unbanned {
		t.Error("Unban should return false for non-existent ban")
	}
}

func TestHubKickUser(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create schema for database operations
	CreateSchema(db)

	hub := NewHub("./plugins", "./data", "http://registry.example.com", db)

	username := "testuser"
	adminUsername := "admin"

	// Test kicking a user
	hub.KickUser(username, adminUsername)

	// Check if user is kicked (temporarily banned)
	if !hub.IsUserBanned(username) {
		t.Error("User should be kicked")
	}

	// Check case insensitive
	if !hub.IsUserBanned(strings.ToUpper(username)) {
		t.Error("Kick should be case insensitive")
	}
}

func TestHubAllowUser(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create schema for database operations
	CreateSchema(db)

	hub := NewHub("./plugins", "./data", "http://registry.example.com", db)

	username := "testuser"
	adminUsername := "admin"

	// First kick the user
	hub.KickUser(username, adminUsername)
	if !hub.IsUserBanned(username) {
		t.Error("User should be kicked")
	}

	// Now allow the user back
	allowed := hub.AllowUser(username, adminUsername)
	if !allowed {
		t.Error("Allow should return true for existing kick")
	}

	// Check if user is allowed back
	if hub.IsUserBanned(username) {
		t.Error("User should not be banned after allow")
	}

	// Test allowing non-kicked user
	allowed = hub.AllowUser("nonexistent", adminUsername)
	if allowed {
		t.Error("Allow should return false for non-kicked user")
	}
}

func TestHubBanOverridesKick(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create schema for database operations
	CreateSchema(db)

	hub := NewHub("./plugins", "./data", "http://registry.example.com", db)

	username := "testuser"
	adminUsername := "admin"

	// First kick the user
	hub.KickUser(username, adminUsername)
	if !hub.IsUserBanned(username) {
		t.Error("User should be kicked")
	}

	// Now ban the user (should override kick)
	hub.BanUser(username, adminUsername)
	if !hub.IsUserBanned(username) {
		t.Error("User should be banned")
	}

	// Try to kick a permanently banned user (should not work)
	hub.KickUser(username, adminUsername)
	if !hub.IsUserBanned(username) {
		t.Error("Permanently banned user should remain banned")
	}
}

func TestHubCleanupExpiredBans(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create schema for database operations
	CreateSchema(db)

	hub := NewHub("./plugins", "./data", "http://registry.example.com", db)

	username := "testuser"
	adminUsername := "admin"

	// Kick a user (24 hour temporary ban)
	hub.KickUser(username, adminUsername)
	if !hub.IsUserBanned(username) {
		t.Error("User should be kicked")
	}

	// Manually set the kick time to the past (simulate expired kick)
	hub.banMutex.Lock()
	hub.tempKicks[strings.ToLower(username)] = time.Now().Add(-1 * time.Hour)
	hub.banMutex.Unlock()

	// Run cleanup
	hub.CleanupExpiredBans()

	// User should no longer be banned
	if hub.IsUserBanned(username) {
		t.Error("User should not be banned after cleanup")
	}
}

func TestHubForceDisconnectUser(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	hub := NewHub("./plugins", "./data", "http://registry.example.com", db)

	username := "testuser"
	adminUsername := "admin"

	// Test force disconnecting non-existent user
	disconnected := hub.ForceDisconnectUser(username, adminUsername)
	if disconnected {
		t.Error("ForceDisconnectUser should return false for non-existent user")
	}

	// Note: Testing with actual clients would require more complex setup
	// with WebSocket connections, which is beyond the scope of unit tests
}

func TestHubGetPluginManager(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	hub := NewHub("./plugins", "./data", "http://registry.example.com", db)

	pluginManager := hub.GetPluginManager()
	if pluginManager == nil {
		t.Error("GetPluginManager should return non-nil plugin manager")
	}

	if pluginManager != hub.pluginManager {
		t.Error("GetPluginManager should return the same plugin manager instance")
	}
}

func TestHubBanCaseInsensitive(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create schema for database operations
	CreateSchema(db)

	hub := NewHub("./plugins", "./data", "http://registry.example.com", db)

	username := "TestUser"
	adminUsername := "admin"

	// Ban user with mixed case
	hub.BanUser(username, adminUsername)

	// Test various case combinations
	testCases := []string{
		"testuser",
		"TESTUSER",
		"TestUser",
		"tEsTuSeR",
	}

	for _, testCase := range testCases {
		if !hub.IsUserBanned(testCase) {
			t.Errorf("Ban should be case insensitive for %s", testCase)
		}
	}
}

func TestHubMultipleBansAndKicks(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create schema for database operations
	CreateSchema(db)

	hub := NewHub("./plugins", "./data", "http://registry.example.com", db)

	adminUsername := "admin"
	users := []string{"user1", "user2", "user3"}

	// Ban multiple users
	for _, user := range users {
		hub.BanUser(user, adminUsername)
	}

	// Check all users are banned
	for _, user := range users {
		if !hub.IsUserBanned(user) {
			t.Errorf("User %s should be banned", user)
		}
	}

	// Unban one user
	if !hub.UnbanUser("user2", adminUsername) {
		t.Error("Should be able to unban user2")
	}

	if hub.IsUserBanned("user2") {
		t.Error("user2 should not be banned after unban")
	}

	// Other users should still be banned
	if !hub.IsUserBanned("user1") {
		t.Error("user1 should still be banned")
	}

	if !hub.IsUserBanned("user3") {
		t.Error("user3 should still be banned")
	}
}

func TestHubConcurrentBanOperations(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create schema for database operations
	CreateSchema(db)

	hub := NewHub("./plugins", "./data", "http://registry.example.com", db)

	username := "testuser"
	adminUsername := "admin"

	// Test concurrent ban/unban operations
	done := make(chan bool, 2)

	// Goroutine 1: Ban and unban user
	go func() {
		for i := 0; i < 100; i++ {
			hub.BanUser(username, adminUsername)
			hub.UnbanUser(username, adminUsername)
		}
		done <- true
	}()

	// Goroutine 2: Check if user is banned
	go func() {
		for i := 0; i < 100; i++ {
			hub.IsUserBanned(username)
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Final state should be consistent
	// The user should not be banned after the unban in the first goroutine
	if hub.IsUserBanned(username) {
		t.Error("User should not be banned after concurrent operations")
	}
}
