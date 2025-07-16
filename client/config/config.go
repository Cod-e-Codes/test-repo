package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Username  string `json:"username"`
	ServerURL string `json:"server_url"`
	Theme     string `json:"theme"`
}

func LoadConfig(path string) (Config, error) {
	var cfg Config
	f, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	json.NewDecoder(f).Decode(&cfg)
	return cfg, nil
}
