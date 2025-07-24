package server

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Port     int      `json:"port"`
	Admins   []string `json:"admins"`
	AdminKey string   `json:"admin_key"`
}

func LoadConfig(path string) (Config, error) {
	var cfg Config
	f, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("could not open config file: %w", err)
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("could not decode config: %w", err)
	}
	return cfg, nil
}
