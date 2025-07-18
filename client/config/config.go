package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Username       string `json:"username"`
	ServerURL      string `json:"server_url"`
	Theme          string `json:"theme"`
	TwentyFourHour bool   `json:"twenty_four_hour"`
	AdminURL       string `json:"admin_url"`
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
