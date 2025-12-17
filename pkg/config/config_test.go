package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	assert.NotEmpty(t, config.GlobalEventLimit)
	assert.NotEmpty(t, config.GlobalReqLimit)
	assert.NotEmpty(t, config.GlobalCountLimit)
	assert.NotEmpty(t, config.IPEventLimit)
	assert.NotEmpty(t, config.IPReqLimit)
	assert.NotEmpty(t, config.IPCountLimit)
	assert.Greater(t, config.MaxConnections, 0)
	assert.Greater(t, config.MaxGlobal, 0)
	assert.Greater(t, config.MaxEventSize, 0)
	assert.Greater(t, config.MaxContentLength, 0)
	assert.Greater(t, config.MaxPerIP, 0)
	assert.NotEmpty(t, config.Timeout)
}

func TestLoadRateLimitConfig_WithYAML(t *testing.T) {
	// Create temporary YAML config
	yamlContent := `
global:
  event_limit: "500/s"
  req_limit: "50/s"
  count_limit: "10/s"
  max_connections: 500
  timeout: "10m"

ip:
  event_limit: "5/s"
  req_limit: "20/s"
  count_limit: "2/s"
  max_connections: 5

event_object_limits:
  max_size: 5000
  max_content_length: 500
`

	tmpFile := filepath.Join(t.TempDir(), "test.yaml")
	err := os.WriteFile(tmpFile, []byte(yamlContent), 0644)
	require.NoError(t, err)

	config, err := LoadRateLimitConfig(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, "500/s", config.GlobalEventLimit)
	assert.Equal(t, "50/s", config.GlobalReqLimit)
	assert.Equal(t, "10/s", config.GlobalCountLimit)
	assert.Equal(t, 500, config.MaxGlobal)
	assert.Equal(t, "10m", config.Timeout)

	assert.Equal(t, "5/s", config.IPEventLimit)
	assert.Equal(t, "20/s", config.IPReqLimit)
	assert.Equal(t, "2/s", config.IPCountLimit)
	assert.Equal(t, 5, config.MaxPerIP)

	assert.Equal(t, 5000, config.MaxEventSize)
	assert.Equal(t, 500, config.MaxContentLength)
}

func TestLoadRateLimitConfig_WithFallback(t *testing.T) {
	// Create old-style config
	oldContent := `global_event_limit: 200/s
global_req_limit: 20/s
max_global: 200
max_per_ip: 3
max_event_size: 2000
`

	tmpFile := filepath.Join(t.TempDir(), "test.conf")
	err := os.WriteFile(tmpFile, []byte(oldContent), 0644)
	require.NoError(t, err)

	config, err := LoadRateLimitConfig(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, "200/s", config.GlobalEventLimit)
	assert.Equal(t, "20/s", config.GlobalReqLimit)
	assert.Equal(t, 200, config.MaxGlobal)
	assert.Equal(t, 3, config.MaxPerIP)
	assert.Equal(t, 2000, config.MaxEventSize)

	// Should have defaults for missing fields from DefaultRateLimitConfig()
	defaultConfig := DefaultRateLimitConfig()
	assert.Equal(t, defaultConfig.GlobalCountLimit, config.GlobalCountLimit)
	assert.Equal(t, defaultConfig.IPEventLimit, config.IPEventLimit)
	assert.Equal(t, defaultConfig.IPReqLimit, config.IPReqLimit)
}

func TestLoadRateLimitConfig_WithEnvironmentOverride(t *testing.T) {
	// Set environment variables
	os.Setenv("GLIENICKE_RATE_LIMITS_GLOBAL_EVENT", "1000/s")
	os.Setenv("GLIENICKE_RATE_LIMITS_GLOBAL_REQ", "100/s")
	os.Setenv("GLIENICKE_RATE_LIMITS_MAX_PER_IP", "50")
	os.Setenv("GLIENICKE_CONNECTION_LIMITS_MAX_GLOBAL", "2000")
	defer func() {
		os.Unsetenv("GLIENICKE_RATE_LIMITS_GLOBAL_EVENT")
		os.Unsetenv("GLIENICKE_RATE_LIMITS_GLOBAL_REQ")
		os.Unsetenv("GLIENICKE_RATE_LIMITS_MAX_PER_IP")
		os.Unsetenv("GLIENICKE_CONNECTION_LIMITS_MAX_GLOBAL")
	}()

	config, err := LoadRateLimitConfig("")
	require.NoError(t, err)

	assert.Equal(t, "1000/s", config.GlobalEventLimit)
	assert.Equal(t, "100/s", config.GlobalReqLimit)
	assert.Equal(t, 50, config.MaxPerIP)
	assert.Equal(t, 2000, config.MaxGlobal)
}

func TestLoadRateLimitConfig_FileNotFound(t *testing.T) {
	config, err := LoadRateLimitConfig("/nonexistent/file.yaml")
	require.NoError(t, err) // Should return defaults when file not found

	// Should have default values
	defaultConfig := DefaultRateLimitConfig()
	assert.Equal(t, defaultConfig.GlobalEventLimit, config.GlobalEventLimit)
	assert.Equal(t, defaultConfig.MaxGlobal, config.MaxGlobal)
}

func TestValidateRateLimitConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := DefaultRateLimitConfig()
		err := ValidateRateLimitConfig(config)
		assert.NoError(t, err)
	})

	t.Run("invalid max connections", func(t *testing.T) {
		config := DefaultRateLimitConfig()
		config.MaxConnections = -1
		err := ValidateRateLimitConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max connections must be positive")
	})

	t.Run("zero max connections", func(t *testing.T) {
		config := DefaultRateLimitConfig()
		config.MaxConnections = 0
		err := ValidateRateLimitConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max connections must be positive")
	})
}
