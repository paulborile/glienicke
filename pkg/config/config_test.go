package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Network.Address != ":8080" {
		t.Errorf("expected default address :8080, got %s", cfg.Network.Address)
	}
	if cfg.Database.Path != "relay.db" {
		t.Errorf("expected default db path relay.db, got %s", cfg.Database.Path)
	}
	if !cfg.RateLimit.Enabled {
		t.Error("expected rate limiting enabled by default")
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default log level info, got %s", cfg.Logging.Level)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				Network:  NetworkConfig{Address: ":8080"},
				Database: DatabaseConfig{Path: "test.db"},
			},
			wantErr: false,
		},
		{
			name: "empty address",
			cfg: &Config{
				Network:  NetworkConfig{Address: ""},
				Database: DatabaseConfig{Path: "test.db"},
			},
			wantErr: true,
		},
		{
			name: "empty db path",
			cfg: &Config{
				Network:  NetworkConfig{Address: ":8080"},
				Database: DatabaseConfig{Path: ""},
			},
			wantErr: true,
		},
		{
			name: "tls cert without key",
			cfg: &Config{
				Network:  NetworkConfig{Address: ":8080", TLSCert: "cert.pem"},
				Database: DatabaseConfig{Path: "test.db"},
			},
			wantErr: true,
		},
		{
			name: "tls key without cert",
			cfg: &Config{
				Network:  NetworkConfig{Address: ":8080", TLSKey: "key.pem"},
				Database: DatabaseConfig{Path: "test.db"},
			},
			wantErr: true,
		},
		{
			name: "valid tls config",
			cfg: &Config{
				Network:  NetworkConfig{Address: ":8080", TLSCert: "cert.pem", TLSKey: "key.pem"},
				Database: DatabaseConfig{Path: "test.db"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	yamlContent := `
network:
  address: ":9090"
  tls_cert: "/path/to/cert.pem"
  tls_key: "/path/to/key.pem"
database:
  path: "/var/lib/relay/db.sqlite"
  max_open_conns: 50
rate_limit:
  enabled: false
  events_per_sec: 5
logging:
  level: "debug"
  format: "json"
features:
  nip28: true
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	loader := NewLoader(configPath)
	cfg, err := loader.LoadWithArgs(nil)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Network.Address != ":9090" {
		t.Errorf("expected address :9090, got %s", cfg.Network.Address)
	}
	if cfg.Network.TLSCert != "/path/to/cert.pem" {
		t.Errorf("expected cert /path/to/cert.pem, got %s", cfg.Network.TLSCert)
	}
	if cfg.Database.Path != "/var/lib/relay/db.sqlite" {
		t.Errorf("expected db path /var/lib/relay/db.sqlite, got %s", cfg.Database.Path)
	}
	if cfg.Database.MaxOpenConns != 50 {
		t.Errorf("expected max_open_conns 50, got %d", cfg.Database.MaxOpenConns)
	}
	if cfg.RateLimit.Enabled {
		t.Error("expected rate limiting disabled")
	}
	if cfg.RateLimit.EventsPerSec != 5 {
		t.Errorf("expected events_per_sec 5, got %d", cfg.RateLimit.EventsPerSec)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("expected log format json, got %s", cfg.Logging.Format)
	}
	if !cfg.Features.NIP28 {
		t.Error("expected NIP28 enabled")
	}
}

func TestEnvironmentVariables(t *testing.T) {
	os.Setenv("GLIENICKE_ADDRESS", ":9999")
	os.Setenv("GLIENICKE_TLS_CERT", "/env/cert.pem")
	os.Setenv("GLIENICKE_TLS_KEY", "/env/key.pem")
	os.Setenv("GLIENICKE_DB_PATH", "/env/relay.db")
	os.Setenv("GLIENICKE_LOG_LEVEL", "warn")
	os.Setenv("GLIENICKE_RATE_LIMIT_ENABLED", "false")
	os.Setenv("GLIENICKE_FEATURE_NIP28", "true")
	defer func() {
		os.Unsetenv("GLIENICKE_ADDRESS")
		os.Unsetenv("GLIENICKE_TLS_CERT")
		os.Unsetenv("GLIENICKE_TLS_KEY")
		os.Unsetenv("GLIENICKE_DB_PATH")
		os.Unsetenv("GLIENICKE_LOG_LEVEL")
		os.Unsetenv("GLIENICKE_RATE_LIMIT_ENABLED")
		os.Unsetenv("GLIENICKE_FEATURE_NIP28")
	}()

	loader := NewLoader("")
	cfg, err := loader.LoadWithArgs(nil)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Network.Address != ":9999" {
		t.Errorf("expected address :9999, got %s", cfg.Network.Address)
	}
	if cfg.Network.TLSCert != "/env/cert.pem" {
		t.Errorf("expected cert /env/cert.pem, got %s", cfg.Network.TLSCert)
	}
	if cfg.Network.TLSKey != "/env/key.pem" {
		t.Errorf("expected key /env/key.pem, got %s", cfg.Network.TLSKey)
	}
	if cfg.Database.Path != "/env/relay.db" {
		t.Errorf("expected db path /env/relay.db, got %s", cfg.Database.Path)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("expected log level warn, got %s", cfg.Logging.Level)
	}
	if cfg.RateLimit.Enabled {
		t.Error("expected rate limiting disabled from env")
	}
	if !cfg.Features.NIP28 {
		t.Error("expected NIP28 enabled from env")
	}
}

func TestConnMaxLifetimeDuration(t *testing.T) {
	cfg := &DatabaseConfig{
		ConnMaxLifetime: 300,
	}

	duration := cfg.ConnMaxLifetimeDuration()
	if duration.Seconds() != 300 {
		t.Errorf("expected 300 seconds, got %v", duration)
	}
}
