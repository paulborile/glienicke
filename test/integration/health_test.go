package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
)

// HealthResponse represents the health check response (copied from relay package)
type HealthResponse struct {
	Status            string  `json:"status"`
	UptimeSeconds     float64 `json:"uptime_seconds"`
	Version           string  `json:"version"`
	ActiveConnections int     `json:"active_connections"`
	TotalConnections  int64   `json:"total_connections"`
	TotalEvents       int64   `json:"total_events"`
	TotalRequests     int64   `json:"total_requests"`
	PacketsPerSecond  float64 `json:"packets_per_second"`
	RateLimitedCount  int64   `json:"rate_limited_count"`
	MemoryUsageMB     float64 `json:"memory_usage_mb"`
	DatabaseStatus    string  `json:"database_status"`
	Timestamp         string  `json:"timestamp"`
}

func TestHealthEndpoint(t *testing.T) {
	_, _, cleanup, baseURL := setupRelay(t)
	defer cleanup()

	// Test health endpoint
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Failed to request health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response
	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	// Validate basic fields
	if health.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", health.Status)
	}

	if health.Version == "" {
		t.Error("Version field should not be empty")
	}

	if health.UptimeSeconds <= 0 {
		t.Error("Uptime should be positive")
	}

	if health.ActiveConnections != 0 {
		t.Errorf("Expected 0 active connections, got %d", health.ActiveConnections)
	}

	if health.DatabaseStatus != "ok" {
		t.Errorf("Expected database status 'ok', got '%s'", health.DatabaseStatus)
	}

	if health.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}

	t.Logf("Health endpoint working correctly: %+v", health)
}

func TestHealthEndpointWithActivity(t *testing.T) {
	url, _, cleanup, baseURL := setupRelay(t)
	defer cleanup()

	// Connect a WebSocket client to generate activity
	client, err := testutil.NewWSClient(url)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Send an event to generate metrics
	evt, _ := testutil.MustNewTestEvent(1, "Test event", nil)
	if err := client.SendEvent(evt); err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}

	// Wait for event to be processed
	if _, _, err := client.ExpectOK(evt.ID, 2*time.Second); err != nil {
		t.Fatalf("Failed to receive OK: %v", err)
	}

	// Give metrics time to update
	time.Sleep(100 * time.Millisecond)

	// Check health endpoint again
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Failed to request health endpoint: %v", err)
	}
	defer resp.Body.Close()

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	// Should now show activity
	if health.ActiveConnections != 1 {
		t.Errorf("Expected 1 active connection, got %d", health.ActiveConnections)
	}

	if health.TotalEvents == 0 {
		t.Error("Total events should be greater than 0")
	}

	if health.TotalConnections == 0 {
		t.Error("Total connections should be greater than 0")
	}

	t.Logf("Health metrics with activity: connections=%d, events=%d, total_conn=%d",
		health.ActiveConnections, health.TotalEvents, health.TotalConnections)
}

func TestHealthEndpointResponseFormat(t *testing.T) {
	_, _, cleanup, baseURL := setupRelay(t)
	defer cleanup()
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Failed to request health endpoint: %v", err)
	}
	defer resp.Body.Close()

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	// Parse and validate all expected fields
	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	expectedFields := []string{
		"status", "uptime_seconds", "version", "active_connections",
		"total_connections", "total_events", "total_requests",
		"packets_per_second", "rate_limited_count", "memory_usage_mb",
		"database_status", "timestamp",
	}

	for _, field := range expectedFields {
		if _, exists := health[field]; !exists {
			t.Errorf("Missing expected field: %s", field)
		}
	}

	t.Log("Health endpoint response format validation passed")
}

func TestHealthEndpointPerformance(t *testing.T) {
	_, _, cleanup, baseURL := setupRelay(t)
	defer cleanup()

	// Test response time
	start := time.Now()
	resp, err := http.Get(baseURL + "/health")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to request health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if duration > 100*time.Millisecond {
		t.Errorf("Health endpoint took too long to respond: %v (expected < 100ms)", duration)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	t.Logf("Health endpoint response time: %v", duration)
}
