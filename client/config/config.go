package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	Username       string `json:"username"`
	ServerURL      string `json:"server_url"`
	Theme          string `json:"theme"`
	TwentyFourHour bool   `json:"twenty_four_hour"`
}

// GetConfigDir returns the platform-appropriate configuration directory
func GetConfigDir() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		// Windows: %APPDATA%\marchat
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", os.ErrNotExist
		}
		configDir = filepath.Join(appData, "marchat")
	case "darwin":
		// macOS: ~/Library/Application Support/marchat
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(homeDir, "Library", "Application Support", "marchat")
	case "linux", "android":
		// Linux and Android/Termux: ~/.config/marchat
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(homeDir, ".config", "marchat")
	default:
		// Fallback for other platforms
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(homeDir, ".config", "marchat")
	}

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}

	return configDir, nil
}

// GetConfigPath returns the full path to the config file
func GetConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

// GetKeystorePath returns the full path to the keystore file
// Checks old location (current directory) first for backward compatibility
func GetKeystorePath() (string, error) {
	// Check if keystore exists in current directory (legacy location)
	legacyPath := "keystore.dat"
	if _, err := os.Stat(legacyPath); err == nil {
		// Legacy keystore exists, use it
		abs, err := filepath.Abs(legacyPath)
		if err != nil {
			return legacyPath, nil // fallback to relative path
		}
		return abs, nil
	}

	// Use new platform-appropriate location
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "keystore.dat"), nil
}

// MigrateKeystoreToNewLocation migrates keystore from legacy location to new platform-appropriate location
func MigrateKeystoreToNewLocation() error {
	legacyPath := "keystore.dat"

	// Check if legacy keystore exists
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		return nil // No legacy keystore to migrate
	}

	// Get new keystore path
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}
	newPath := filepath.Join(configDir, "keystore.dat")

	// Check if new keystore already exists
	if _, err := os.Stat(newPath); err == nil {
		return nil // New keystore already exists, don't overwrite
	}

	// Copy legacy keystore to new location
	input, err := os.ReadFile(legacyPath)
	if err != nil {
		return err
	}

	if err := os.WriteFile(newPath, input, 0600); err != nil {
		return err
	}

	return nil
}

func LoadConfig(path string) (Config, error) {
	var cfg Config
	f, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func SaveConfig(path string, cfg Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(cfg)
}
