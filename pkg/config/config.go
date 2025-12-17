package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// RateLimitConfig contains rate limiting configuration
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
		GlobalReqLimit:     "10/s",
		GlobalCountLimit:   "5/s",
		IPEventLimit:       "1/minute",
		IPReqLimit:         "10/s",
		IPCountLimit:       "1/s",
		MaxConnections:     10,
		MaxGlobal:          1000,
		MaxEventSize:       10000,
		MaxEventsPerMinute: 60,
		MaxContentLength:   1000,
		MaxPerIP:           10,
		MaxGlobal:          1000,
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
			return nil, fmt.Errorf("failed to read rate limit config file: %w", err)
		}

		// Simple JSON parsing for now
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

// ValidateRateLimitConfig validates configuration
func ValidateRateLimitConfig(config *RateLimitConfig) error {
	if config.MaxConnections <= 0 {
		return fmt.Errorf("max connections must be positive")
	}
	return nil
}
