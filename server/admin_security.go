package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/Cod-e-Codes/marchat/shared"
)

// ConfirmationType represents the type of confirmation required
type ConfirmationType string

const (
	ConfirmationClearDB     ConfirmationType = "clear_db"
	ConfirmationBackupDB    ConfirmationType = "backup_db"
	ConfirmationBanUser     ConfirmationType = "ban_user"
	ConfirmationKickUser    ConfirmationType = "kick_user"
	ConfirmationDeleteUser  ConfirmationType = "delete_user"
	ConfirmationResetConfig ConfirmationType = "reset_config"
)

// ConfirmationPrompt represents a pending confirmation
type ConfirmationPrompt struct {
	Type         ConfirmationType
	UserID       string
	Message      string
	Confirmation string
	Timestamp    time.Time
	Data         map[string]interface{}
}

// AdminSecurityManager handles security-related admin operations
type AdminSecurityManager struct {
	pendingConfirmations map[string]*ConfirmationPrompt
	confirmationTimeout  time.Duration
}

// NewAdminSecurityManager creates a new admin security manager
func NewAdminSecurityManager() *AdminSecurityManager {
	return &AdminSecurityManager{
		pendingConfirmations: make(map[string]*ConfirmationPrompt),
		confirmationTimeout:  5 * time.Minute, // 5 minute timeout for confirmations
	}
}

// RequireConfirmation creates a confirmation prompt for a destructive operation
func (asm *AdminSecurityManager) RequireConfirmation(userID string, confType ConfirmationType, message string, data map[string]interface{}) *ConfirmationPrompt {
	confirmation := asm.getConfirmationText(confType)

	prompt := &ConfirmationPrompt{
		Type:         confType,
		UserID:       userID,
		Message:      message,
		Confirmation: confirmation,
		Timestamp:    time.Now(),
		Data:         data,
	}

	asm.pendingConfirmations[userID] = prompt

	// Log the security event
	SecurityLogger.Info("Confirmation required for destructive operation", map[string]interface{}{
		"user_id": userID,
		"type":    confType,
		"message": message,
	})

	return prompt
}

// CheckConfirmation checks if a user's input matches the required confirmation
func (asm *AdminSecurityManager) CheckConfirmation(userID, input string) (bool, *ConfirmationPrompt) {
	prompt, exists := asm.pendingConfirmations[userID]
	if !exists {
		return false, nil
	}

	// Check if confirmation has expired
	if time.Since(prompt.Timestamp) > asm.confirmationTimeout {
		delete(asm.pendingConfirmations, userID)
		SecurityLogger.Warn("Confirmation expired", map[string]interface{}{
			"user_id": userID,
			"type":    prompt.Type,
		})
		return false, nil
	}

	// Check if input matches required confirmation
	if strings.EqualFold(strings.TrimSpace(input), prompt.Confirmation) {
		delete(asm.pendingConfirmations, userID)
		SecurityLogger.Info("Confirmation successful", map[string]interface{}{
			"user_id": userID,
			"type":    prompt.Type,
		})
		return true, prompt
	}

	return false, prompt
}

// GetPendingConfirmation returns the pending confirmation for a user
func (asm *AdminSecurityManager) GetPendingConfirmation(userID string) *ConfirmationPrompt {
	prompt, exists := asm.pendingConfirmations[userID]
	if !exists {
		return nil
	}

	// Check if confirmation has expired
	if time.Since(prompt.Timestamp) > asm.confirmationTimeout {
		delete(asm.pendingConfirmations, userID)
		return nil
	}

	return prompt
}

// CancelConfirmation cancels a pending confirmation
func (asm *AdminSecurityManager) CancelConfirmation(userID string) bool {
	_, exists := asm.pendingConfirmations[userID]
	if exists {
		delete(asm.pendingConfirmations, userID)
		SecurityLogger.Info("Confirmation cancelled", map[string]interface{}{
			"user_id": userID,
		})
		return true
	}
	return false
}

// CleanupExpiredConfirmations removes expired confirmations
func (asm *AdminSecurityManager) CleanupExpiredConfirmations() {
	now := time.Now()
	for userID, prompt := range asm.pendingConfirmations {
		if now.Sub(prompt.Timestamp) > asm.confirmationTimeout {
			delete(asm.pendingConfirmations, userID)
			SecurityLogger.Debug("Cleaned up expired confirmation", map[string]interface{}{
				"user_id": userID,
				"type":    prompt.Type,
			})
		}
	}
}

// getConfirmationText returns the required confirmation text for each operation type
func (asm *AdminSecurityManager) getConfirmationText(confType ConfirmationType) string {
	switch confType {
	case ConfirmationClearDB:
		return "CONFIRM"
	case ConfirmationBackupDB:
		return "BACKUP"
	case ConfirmationBanUser:
		return "BAN"
	case ConfirmationKickUser:
		return "KICK"
	case ConfirmationDeleteUser:
		return "DELETE"
	case ConfirmationResetConfig:
		return "RESET"
	default:
		return "CONFIRM"
	}
}

// FormatConfirmationMessage formats a confirmation prompt message
func (asm *AdminSecurityManager) FormatConfirmationMessage(prompt *ConfirmationPrompt) string {
	var sb strings.Builder

	sb.WriteString("🚨 DESTRUCTIVE OPERATION\n")
	sb.WriteString(prompt.Message)
	sb.WriteString(fmt.Sprintf("\nType \"%s\" to proceed: _", prompt.Confirmation))

	return sb.String()
}

// HandleAdminCommandWithConfirmation handles admin commands that require confirmation
func (asm *AdminSecurityManager) HandleAdminCommandWithConfirmation(client *Client, command string, args []string) (bool, string) {
	// Check if user has a pending confirmation
	if prompt := asm.GetPendingConfirmation(client.username); prompt != nil {
		// User is trying to confirm an operation
		if confirmed, confirmedPrompt := asm.CheckConfirmation(client.username, command); confirmed {
			return asm.executeConfirmedOperation(client, confirmedPrompt)
		} else {
			return false, "❌ Invalid confirmation. Please type the exact confirmation text."
		}
	}

	// Handle commands that require confirmation
	switch args[0] {
	case ":cleardb":
		return asm.handleClearDBConfirmation(client)
	case ":backup":
		return asm.handleBackupConfirmation(client)
	case ":ban":
		if len(args) < 2 {
			return false, "Usage: :ban <username>"
		}
		return asm.handleBanConfirmation(client, args[1])
	case ":kick":
		if len(args) < 2 {
			return false, "Usage: :kick <username>"
		}
		return asm.handleKickConfirmation(client, args[1])
	default:
		return false, ""
	}
}

// handleClearDBConfirmation creates a confirmation prompt for clearing the database
func (asm *AdminSecurityManager) handleClearDBConfirmation(client *Client) (bool, string) {
	prompt := asm.RequireConfirmation(
		client.username,
		ConfirmationClearDB,
		"Clear Database will permanently delete all messages.\nThis action cannot be undone.",
		map[string]interface{}{
			"operation": "clear_database",
		},
	)

	message := asm.FormatConfirmationMessage(prompt)

	// Send confirmation prompt to client
	client.send <- shared.Message{
		Sender:    "System",
		Content:   message,
		CreatedAt: time.Now(),
		Type:      shared.TextMessage,
	}

	return true, ""
}

// handleBackupConfirmation creates a confirmation prompt for database backup
func (asm *AdminSecurityManager) handleBackupConfirmation(client *Client) (bool, string) {
	prompt := asm.RequireConfirmation(
		client.username,
		ConfirmationBackupDB,
		"Database backup will create a copy of all data.\nThis may take a few moments.",
		map[string]interface{}{
			"operation": "backup_database",
		},
	)

	message := asm.FormatConfirmationMessage(prompt)

	// Send confirmation prompt to client
	client.send <- shared.Message{
		Sender:    "System",
		Content:   message,
		CreatedAt: time.Now(),
		Type:      shared.TextMessage,
	}

	return true, ""
}

// handleBanConfirmation creates a confirmation prompt for banning a user
func (asm *AdminSecurityManager) handleBanConfirmation(client *Client, username string) (bool, string) {
	prompt := asm.RequireConfirmation(
		client.username,
		ConfirmationBanUser,
		fmt.Sprintf("Ban user '%s' will permanently prevent them from connecting.\nThis action can be reversed with :unban.", username),
		map[string]interface{}{
			"operation": "ban_user",
			"target":    username,
		},
	)

	message := asm.FormatConfirmationMessage(prompt)

	// Send confirmation prompt to client
	client.send <- shared.Message{
		Sender:    "System",
		Content:   message,
		CreatedAt: time.Now(),
		Type:      shared.TextMessage,
	}

	return true, ""
}

// handleKickConfirmation creates a confirmation prompt for kicking a user
func (asm *AdminSecurityManager) handleKickConfirmation(client *Client, username string) (bool, string) {
	prompt := asm.RequireConfirmation(
		client.username,
		ConfirmationKickUser,
		fmt.Sprintf("Kick user '%s' will temporarily disconnect them for 24 hours.\nThis action can be reversed with :allow.", username),
		map[string]interface{}{
			"operation": "kick_user",
			"target":    username,
		},
	)

	message := asm.FormatConfirmationMessage(prompt)

	// Send confirmation prompt to client
	client.send <- shared.Message{
		Sender:    "System",
		Content:   message,
		CreatedAt: time.Now(),
		Type:      shared.TextMessage,
	}

	return true, ""
}

// executeConfirmedOperation executes the operation after confirmation
func (asm *AdminSecurityManager) executeConfirmedOperation(client *Client, prompt *ConfirmationPrompt) (bool, string) {
	switch prompt.Type {
	case ConfirmationClearDB:
		return asm.executeClearDB(client)
	case ConfirmationBackupDB:
		return asm.executeBackupDB(client)
	case ConfirmationBanUser:
		return asm.executeBanUser(client, prompt.Data["target"].(string))
	case ConfirmationKickUser:
		return asm.executeKickUser(client, prompt.Data["target"].(string))
	default:
		return false, "❌ Unknown operation type"
	}
}

// executeClearDB executes the database clear operation
func (asm *AdminSecurityManager) executeClearDB(client *Client) (bool, string) {
	err := ClearMessages(client.db)
	if err != nil {
		SecurityLogger.Error("Failed to clear database", err, map[string]interface{}{
			"user_id": client.username,
		})
		return false, "❌ Failed to clear database: " + err.Error()
	}

	// Broadcast to all users
	client.hub.broadcast <- shared.Message{
		Sender:    "System",
		Content:   "Chat history cleared by admin.",
		CreatedAt: time.Now(),
		Type:      shared.TextMessage,
	}

	SecurityLogger.Info("Database cleared successfully", map[string]interface{}{
		"user_id": client.username,
	})

	return true, "✅ Database cleared successfully"
}

// executeBackupDB executes the database backup operation
func (asm *AdminSecurityManager) executeBackupDB(client *Client) (bool, string) {
	// This would need the actual database path from server config
	// For now, return a placeholder response
	SecurityLogger.Info("Database backup requested", map[string]interface{}{
		"user_id": client.username,
	})

	return true, "✅ Database backup initiated (feature not fully implemented)"
}

// executeBanUser executes the user ban operation
func (asm *AdminSecurityManager) executeBanUser(client *Client, username string) (bool, string) {
	client.hub.BanUser(username, client.username)

	SecurityLogger.Info("User banned", map[string]interface{}{
		"admin_user":  client.username,
		"target_user": username,
	})

	return true, fmt.Sprintf("✅ User '%s' has been permanently banned", username)
}

// executeKickUser executes the user kick operation
func (asm *AdminSecurityManager) executeKickUser(client *Client, username string) (bool, string) {
	client.hub.KickUser(username, client.username)

	SecurityLogger.Info("User kicked", map[string]interface{}{
		"admin_user":  client.username,
		"target_user": username,
	})

	return true, fmt.Sprintf("✅ User '%s' has been kicked (24 hour temporary ban)", username)
}
