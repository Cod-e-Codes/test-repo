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
func GetKeystorePath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "keystore.dat"), nil
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
