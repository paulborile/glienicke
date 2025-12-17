package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLConfig represents the YAML configuration file structure
type YAMLConfig struct {
	Global GlobalConfig `yaml:"global"`
	IP     IPConfig     `yaml:"ip"`
	Event  EventConfig  `yaml:"event_object_limits"`
}

// GlobalConfig contains global rate limit settings
type GlobalConfig struct {
	EventLimit     string `yaml:"event_limit"`
	ReqLimit       string `yaml:"req_limit"`
	CountLimit     string `yaml:"count_limit"`
	MaxConnections int    `yaml:"max_connections"`
	Timeout        string `yaml:"timeout"`
}

// IPConfig contains per-IP rate limit settings
type IPConfig struct {
	EventLimit     string `yaml:"event_limit"`
	ReqLimit       string `yaml:"req_limit"`
	CountLimit     string `yaml:"count_limit"`
	MaxConnections int    `yaml:"max_connections"`
}

// EventConfig contains event validation limits
type EventConfig struct {
	MaxSize          int `yaml:"max_size"`
	MaxContentLength int `yaml:"max_content_length"`
}

// RateLimitConfig contains rate limiting configuration (legacy format)
type RateLimitConfig struct {
	GlobalEventLimit   string `json:"global_event_limit"`
	GlobalReqLimit     string `json:"global_req_limit"`
	GlobalCountLimit   string `json:"global_count_limit"`
	IPEventLimit       string `json:"ip_event_limit"`
	IPReqLimit         string `json:"ip_req_limit"`
	IPCountLimit       string `json:"ip_count_limit"`
	MaxConnections     int    `json:"max_connections"`
	MaxGlobal          int    `json:"max_global"`
	MaxEventSize       int    `json:"max_event_size"`
	MaxEventsPerMinute int    `json:"max_events_per_minute"`
	MaxContentLength   int    `json:"max_content_length"`
	MaxPerIP           int    `json:"max_per_ip"`
	Timeout            string `json:"timeout"`
}

// DefaultConfig returns default configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		GlobalEventLimit:   "1000/s",
		GlobalReqLimit:     "100/s",
		GlobalCountLimit:   "50/s",
		IPEventLimit:       "10/s",
		IPReqLimit:         "10/s",
		IPCountLimit:       "5/s",
		MaxConnections:     100,
		MaxGlobal:          1000,
		MaxEventSize:       100000, // 100KB to accommodate encrypted events
		MaxEventsPerMinute: 60,
		MaxContentLength:   10000, // 10KB content length limit
		MaxPerIP:           10,
		Timeout:            "5m",
	}
}

// LoadRateLimitConfig loads configuration from file and environment
func LoadRateLimitConfig(path string) (*RateLimitConfig, error) {
	config := DefaultRateLimitConfig()

	// Load from file if exists
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			// If file doesn't exist, return defaults silently
			if os.IsNotExist(err) {
				return config, nil
			}
			return nil, fmt.Errorf("failed to read rate limit config file: %w", err)
		}

		// Try YAML parsing first
		var yamlConfig YAMLConfig
		if err := yaml.Unmarshal(data, &yamlConfig); err == nil && !isEmptyYAMLConfig(yamlConfig) {
			// Update config from YAML structure
			if yamlConfig.Global.EventLimit != "" {
				config.GlobalEventLimit = yamlConfig.Global.EventLimit
			}
			if yamlConfig.Global.ReqLimit != "" {
				config.GlobalReqLimit = yamlConfig.Global.ReqLimit
			}
			if yamlConfig.Global.CountLimit != "" {
				config.GlobalCountLimit = yamlConfig.Global.CountLimit
			}
			if yamlConfig.Global.MaxConnections > 0 {
				config.MaxGlobal = yamlConfig.Global.MaxConnections
			}
			if yamlConfig.Global.Timeout != "" {
				config.Timeout = yamlConfig.Global.Timeout
			}

			if yamlConfig.IP.EventLimit != "" {
				config.IPEventLimit = yamlConfig.IP.EventLimit
			}
			if yamlConfig.IP.ReqLimit != "" {
				config.IPReqLimit = yamlConfig.IP.ReqLimit
			}
			if yamlConfig.IP.CountLimit != "" {
				config.IPCountLimit = yamlConfig.IP.CountLimit
			}
			if yamlConfig.IP.MaxConnections > 0 {
				config.MaxPerIP = yamlConfig.IP.MaxConnections
			}

			if yamlConfig.Event.MaxSize > 0 {
				config.MaxEventSize = yamlConfig.Event.MaxSize
			}
			if yamlConfig.Event.MaxContentLength > 0 {
				config.MaxContentLength = yamlConfig.Event.MaxContentLength
			}
		} else {
			// Fall back to simple key:value parsing for backward compatibility
			content := string(data)
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || line[0] == '#' {
					continue
				}

				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])

					switch key {
					case "global_event_limit":
						config.GlobalEventLimit = value
					case "global_req_limit":
						config.GlobalReqLimit = value
					case "max_connections":
						if val, err := strconv.Atoi(value); err == nil {
							config.MaxConnections = val
						}
					case "max_global":
						if val, err := strconv.Atoi(value); err == nil {
							config.MaxGlobal = val
						}
					case "max_event_size":
						if val, err := strconv.Atoi(value); err == nil {
							config.MaxEventSize = val
						}
					case "max_events_per_minute":
						if val, err := strconv.Atoi(value); err == nil {
							config.MaxEventsPerMinute = val
						}
					case "max_content_length":
						if val, err := strconv.Atoi(value); err == nil {
							config.MaxContentLength = val
						}
					case "max_per_ip":
						if val, err := strconv.Atoi(value); err == nil {
							config.MaxPerIP = val
						}
					case "timeout":
						config.Timeout = value
					}
				}
			}
		}
	}

	// Override with environment variables
	if val := os.Getenv("GLIENICKE_RATE_LIMITS_GLOBAL_EVENT"); val != "" {
		config.GlobalEventLimit = val
	}
	if val := os.Getenv("GLIENICKE_RATE_LIMITS_GLOBAL_REQ"); val != "" {
		config.GlobalReqLimit = val
	}
	if val := os.Getenv("GLIENICKE_RATE_LIMITS_MAX_PER_IP"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.MaxPerIP = parsed
		}
	}
	if val := os.Getenv("GLIENICKE_CONNECTION_LIMITS_MAX_GLOBAL"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.MaxGlobal = parsed
		}
	}

	return config, nil
}

// isEmptyYAMLConfig checks if YAML config is effectively empty
func isEmptyYAMLConfig(yamlConfig YAMLConfig) bool {
	return yamlConfig.Global.EventLimit == "" &&
		yamlConfig.Global.ReqLimit == "" &&
		yamlConfig.Global.CountLimit == "" &&
		yamlConfig.Global.MaxConnections == 0 &&
		yamlConfig.Global.Timeout == "" &&
		yamlConfig.IP.EventLimit == "" &&
		yamlConfig.IP.ReqLimit == "" &&
		yamlConfig.IP.CountLimit == "" &&
		yamlConfig.IP.MaxConnections == 0 &&
		yamlConfig.Event.MaxSize == 0 &&
		yamlConfig.Event.MaxContentLength == 0
}

// ValidateRateLimitConfig validates configuration
func ValidateRateLimitConfig(config *RateLimitConfig) error {
	if config.MaxConnections <= 0 {
		return fmt.Errorf("max connections must be positive")
	}
	return nil
}
