package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Network   NetworkConfig   `yaml:"network" json:"network"`
	Database  DatabaseConfig  `yaml:"database" json:"database"`
	RateLimit RateLimitConfig `yaml:"rate_limit" json:"rate_limit"`
	Logging   LoggingConfig   `yaml:"logging" json:"logging"`
	Features  FeaturesConfig  `yaml:"features"`
}

type NetworkConfig struct {
	Address      string `yaml:"address" json:"address" env:"GLIENICKE_ADDRESS"`
	TLSCert      string `yaml:"tls_cert" json:"tls_cert" env:"GLIENICKE_TLS_CERT"`
	TLSKey       string `yaml:"tls_key" json:"tls_key" env:"GLIENICKE_TLS_KEY"`
	ReadTimeout  int    `yaml:"read_timeout" json:"read_timeout" env:"GLIENICKE_READ_TIMEOUT"`
	WriteTimeout int    `yaml:"write_timeout" json:"write_timeout" env:"GLIENICKE_WRITE_TIMEOUT"`
}

type DatabaseConfig struct {
	Path            string `yaml:"path" json:"path" env:"GLIENICKE_DB_PATH"`
	MaxOpenConns    int    `yaml:"max_open_conns" json:"max_open_conns" env:"GLIENICKE_DB_MAX_OPEN_CONNS"`
	MaxIdleConns    int    `yaml:"max_idle_conns" json:"max_idle_conns" env:"GLIENICKE_DB_MAX_IDLE_CONNS"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime" json:"conn_max_lifetime" env:"GLIENICKE_DB_CONN_MAX_LIFETIME"`
}

type RateLimitConfig struct {
	Enabled        bool `yaml:"enabled" json:"enabled" env:"GLIENICKE_RATE_LIMIT_ENABLED"`
	EventsPerSec   int  `yaml:"events_per_sec" json:"events_per_sec" env:"GLIENICKE_RATE_LIMIT_EVENTS_PER_SEC"`
	ReqPerSec      int  `yaml:"req_per_sec" json:"req_per_sec" env:"GLIENICKE_RATE_LIMIT_REQ_PER_SEC"`
	MaxConnections int  `yaml:"max_connections" json:"max_connections" env:"GLIENICKE_RATE_LIMIT_MAX_CONNECTIONS"`
	MaxEventSize   int  `yaml:"max_event_size" json:"max_event_size" env:"GLIENICKE_RATE_LIMIT_MAX_EVENT_SIZE"`
}

type LoggingConfig struct {
	Level  string `yaml:"level" json:"level" env:"GLIENICKE_LOG_LEVEL"`
	Format string `yaml:"format" json:"format" env:"GLIENICKE_LOG_FORMAT"`
}

type FeaturesConfig struct {
	NIP11 bool `yaml:"nip11" json:"nip11" env:"GLIENICKE_FEATURE_NIP11"`
	NIP42 bool `yaml:"nip42" json:"nip42" env:"GLIENICKE_FEATURE_NIP42"`
	NIP28 bool `yaml:"nip28" json:"nip28" env:"GLIENICKE_FEATURE_NIP28"`
}

func DefaultConfig() *Config {
	return &Config{
		Network: NetworkConfig{
			Address:      ":8080",
			ReadTimeout:  30,
			WriteTimeout: 30,
		},
		Database: DatabaseConfig{
			Path:            "relay.db",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 300,
		},
		RateLimit: RateLimitConfig{
			Enabled:        true,
			EventsPerSec:   10,
			ReqPerSec:      10,
			MaxConnections: 100,
			MaxEventSize:   65536,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Features: FeaturesConfig{
			NIP11: true,
			NIP42: true,
			NIP28: false,
		},
	}
}

func (c *Config) Validate() error {
	if c.Network.Address == "" {
		return fmt.Errorf("network address cannot be empty")
	}
	if c.Database.Path == "" {
		return fmt.Errorf("database path cannot be empty")
	}
	if c.Network.TLSCert != "" && c.Network.TLSKey == "" {
		return fmt.Errorf("TLS key is required when TLS cert is provided")
	}
	if c.Network.TLSKey != "" && c.Network.TLSCert == "" {
		return fmt.Errorf("TLS cert is required when TLS key is provided")
	}
	return nil
}

func (c *DatabaseConfig) ConnMaxLifetimeDuration() time.Duration {
	return time.Duration(c.ConnMaxLifetime) * time.Second
}

type Loader struct {
	configFile string
	flags      *flag.FlagSet
}

func NewLoader(configFile string) *Loader {
	return &Loader{
		configFile: configFile,
		flags:      flag.NewFlagSet("relay", flag.ContinueOnError),
	}
}

func (l *Loader) Load() (*Config, error) {
	return l.LoadWithArgs(os.Args[1:])
}

func (l *Loader) LoadWithArgs(args []string) (*Config, error) {
	cfg := DefaultConfig()

	if l.configFile != "" {
		if err := l.loadFromFile(cfg, l.configFile); err != nil {
			return nil, fmt.Errorf("failed to load config from file %s: %w", l.configFile, err)
		}
	}

	l.applyEnvironmentVariables(cfg)
	l.applyFlags(cfg, args)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func (l *Loader) loadFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

func (l *Loader) applyEnvironmentVariables(cfg *Config) {
	applyIfSet := func(envVar string, setter func(string)) {
		if val := os.Getenv(envVar); val != "" {
			setter(val)
		}
	}

	applyIfSet("GLIENICKE_ADDRESS", func(v string) { cfg.Network.Address = v })
	applyIfSet("GLIENICKE_TLS_CERT", func(v string) { cfg.Network.TLSCert = v })
	applyIfSet("GLIENICKE_TLS_KEY", func(v string) { cfg.Network.TLSKey = v })
	applyIfSet("GLIENICKE_DB_PATH", func(v string) { cfg.Database.Path = v })
	applyIfSet("GLIENICKE_LOG_LEVEL", func(v string) { cfg.Logging.Level = v })
	applyIfSet("GLIENICKE_LOG_FORMAT", func(v string) { cfg.Logging.Format = v })
	applyIfSet("GLIENICKE_RATE_LIMIT_ENABLED", func(v string) { cfg.RateLimit.Enabled = v == "true" || v == "1" })
	applyIfSet("GLIENICKE_FEATURE_NIP11", func(v string) { cfg.Features.NIP11 = v == "true" || v == "1" })
	applyIfSet("GLIENICKE_FEATURE_NIP42", func(v string) { cfg.Features.NIP42 = v == "true" || v == "1" })
	applyIfSet("GLIENICKE_FEATURE_NIP28", func(v string) { cfg.Features.NIP28 = v == "true" || v == "1" })
}

func (l *Loader) applyFlags(cfg *Config, args []string) {
	l.flags.StringVar(&cfg.Network.Address, "addr", cfg.Network.Address, "Address to listen on")
	l.flags.StringVar(&cfg.Database.Path, "db", cfg.Database.Path, "Path to SQLite database")
	l.flags.StringVar(&cfg.Network.TLSCert, "cert", cfg.Network.TLSCert, "TLS certificate file for WSS")
	l.flags.StringVar(&cfg.Network.TLSKey, "key", cfg.Network.TLSKey, "TLS private key file for WSS")

	l.flags.Parse(args)
}

func (l *Loader) Flags() *flag.FlagSet {
	return l.flags
}

func SaveExampleConfig(path string) error {
	cfg := DefaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	example := []byte("# Glienicke Nostr Relay Configuration\n# Copy this to relay.yaml and modify as needed\n\n" + string(data))

	return os.WriteFile(path, example, 0644)
}
