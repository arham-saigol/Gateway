package config

import (
	"encoding"
	"fmt"
	"os"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Duration wraps time.Duration to support TOML string decoding (e.g. "15s", "1m").
type Duration time.Duration

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

func (d *Duration) UnmarshalText(text []byte) error {
	str := string(text)
	dur, err := time.ParseDuration(str)
	if err != nil {
		return fmt.Errorf("invalid duration format: %s: %w", str, err)
	}
	*d = Duration(dur)
	return nil
}

var _ encoding.TextUnmarshaler = (*Duration)(nil)

type ServerConfig struct {
	ListenAddr          string   `toml:"listen_addr"`
	TrustedProxies      []string `toml:"trusted_proxies"`
	MaxRequestBodyBytes int64    `toml:"max_request_body_bytes"`
	ReadTimeout         Duration `toml:"read_timeout"`
	WriteTimeout        Duration `toml:"write_timeout"`
	IdleTimeout         Duration `toml:"idle_timeout"`
}

type DatabaseConfig struct {
	Path          string `toml:"path"`
	BusyTimeoutMs int    `toml:"busy_timeout_ms"`
}

type SecurityConfig struct {
	MasterKeyPath      string   `toml:"master_key_path"`
	AdminSessionExpiry Duration `toml:"admin_session_expiry"`
	CookieSecure       bool     `toml:"cookie_secure"`
}

type TimeoutsConfig struct {
	FirstResponseTimeout Duration `toml:"first_response_timeout"`
	StreamDrainTimeout   Duration `toml:"stream_drain_timeout"`
	UpstreamDialTimeout  Duration `toml:"upstream_dial_timeout"`
}

type RoutingConfig struct {
	MaxRetriesPerRequest int      `toml:"max_retries_per_request"`
	KeyCooldownDuration  Duration `toml:"key_cooldown_duration"`
}

type RetentionConfig struct {
	DetailedLogDays int      `toml:"detailed_log_days"`
	CleanupInterval Duration `toml:"cleanup_interval"`
}

type Config struct {
	Server    ServerConfig    `toml:"server"`
	Database  DatabaseConfig  `toml:"database"`
	Security  SecurityConfig  `toml:"security"`
	Timeouts  TimeoutsConfig  `toml:"timeouts"`
	Routing   RoutingConfig   `toml:"routing"`
	Retention RetentionConfig `toml:"retention"`
}

func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			ListenAddr:          "127.0.0.1:8080",
			TrustedProxies:      []string{"127.0.0.1"},
			MaxRequestBodyBytes: 10 * 1024 * 1024, // 10MB
			ReadTimeout:         Duration(30 * time.Second),
			WriteTimeout:        Duration(0), // 0 for unbounded streaming
			IdleTimeout:         Duration(120 * time.Second),
		},
		Database: DatabaseConfig{
			Path:          "/var/lib/arham-gateway/gateway.db",
			BusyTimeoutMs: 5000,
		},
		Security: SecurityConfig{
			MasterKeyPath:      "/etc/arham-gateway/master.key",
			AdminSessionExpiry: Duration(24 * time.Hour),
			CookieSecure:       false,
		},
		Timeouts: TimeoutsConfig{
			FirstResponseTimeout: Duration(30 * time.Second),
			StreamDrainTimeout:   Duration(15 * time.Second),
			UpstreamDialTimeout:  Duration(10 * time.Second),
		},
		Routing: RoutingConfig{
			MaxRetriesPerRequest: 3,
			KeyCooldownDuration:  Duration(30 * time.Second),
		},
		Retention: RetentionConfig{
			DetailedLogDays: 30,
			CleanupInterval: Duration(1 * time.Hour),
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return &cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}

	return &cfg, nil
}
