package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestServerConfigJSONLoaders(t *testing.T) {
	tdir := t.TempDir()
	cfgPath := filepath.Join(tdir, "server_config.json")
	data := map[string]interface{}{"port": 9090, "admins": []string{"x"}, "admin_key": "k"}
	b, _ := json.Marshal(data)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Port != 9090 || cfg.AdminKey != "k" || len(cfg.Admins) != 1 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}

	// LoadConfigFromDir finds server_config.json
	cfg2, err := LoadConfigFromDir(tdir)
	if err != nil {
		t.Fatalf("LoadConfigFromDir: %v", err)
	}
	if cfg2.Port != 9090 {
		t.Fatalf("unexpected port: %d", cfg2.Port)
	}
}
