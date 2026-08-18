package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"arham-gateway/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg.Server.ListenAddr != "127.0.0.1:8080" {
		t.Errorf("expected default listen addr 127.0.0.1:8080, got %s", cfg.Server.ListenAddr)
	}
	if cfg.Database.Path != "/var/lib/arham-gateway/gateway.db" {
		t.Errorf("expected default db path, got %s", cfg.Database.Path)
	}
	if cfg.Security.MasterKeyPath != "/etc/arham-gateway/master.key" {
		t.Errorf("expected default master key path, got %s", cfg.Security.MasterKeyPath)
	}
	if cfg.Timeouts.FirstResponseTimeout.Duration() != 30*time.Second {
		t.Errorf("expected 30s first response timeout, got %v", cfg.Timeouts.FirstResponseTimeout)
	}
	if cfg.Retention.DetailedLogDays != 30 {
		t.Errorf("expected 30 days retention, got %d", cfg.Retention.DetailedLogDays)
	}
}

func TestLoadConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "config.toml")
	tomlData := `
[server]
listen_addr = "127.0.0.1:9090"
trusted_proxies = ["127.0.0.1"]

[database]
path = "test.db"

[security]
master_key_path = "test.key"

[timeouts]
first_response_timeout = "15s"
stream_drain_timeout = "20s"

[retention]
detailed_log_days = 14
`
	if err := os.WriteFile(confPath, []byte(tomlData), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := config.Load(confPath)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Server.ListenAddr != "127.0.0.1:9090" {
		t.Errorf("expected listen_addr 127.0.0.1:9090, got %s", cfg.Server.ListenAddr)
	}
	if cfg.Database.Path != "test.db" {
		t.Errorf("expected test.db, got %s", cfg.Database.Path)
	}
	if cfg.Security.MasterKeyPath != "test.key" {
		t.Errorf("expected test.key, got %s", cfg.Security.MasterKeyPath)
	}
	if cfg.Timeouts.FirstResponseTimeout.Duration() != 15*time.Second {
		t.Errorf("expected 15s timeout, got %v", cfg.Timeouts.FirstResponseTimeout)
	}
	if cfg.Retention.DetailedLogDays != 14 {
		t.Errorf("expected 14 days retention, got %d", cfg.Retention.DetailedLogDays)
	}
}
