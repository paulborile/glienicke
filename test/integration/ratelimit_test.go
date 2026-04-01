package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
)

func TestReqRateLimiting(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	filter := &event.Filter{Kinds: []int{1}}

	// Send a burst of REQs that exceeds the rate limit (burst=20)
	// Send 25 rapid REQs — first 20 should succeed, remaining should be rate limited
	rateLimited := 0
	succeeded := 0

	for i := 0; i < 25; i++ {
		subID := fmt.Sprintf("burst-sub-%d", i)
		if err := client.SendReq(subID, filter); err != nil {
			t.Fatalf("Failed to send REQ %d: %v", i, err)
		}
	}

	// Read all responses — we expect EOSE for successful ones and CLOSED for rate-limited ones
	deadline := time.Now().Add(5 * time.Second)
	client.SetReadDeadline(deadline)

	for i := 0; i < 25; i++ {
		msg, err := client.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message %d: %v", i, err)
		}

		if len(msg) < 2 {
			continue
		}

		msgType, _ := msg[0].(string)

		switch msgType {
		case "EOSE":
			succeeded++
		case "CLOSED":
			if len(msg) >= 3 {
				reason, _ := msg[2].(string)
				if !strings.Contains(reason, "rate-limited") {
					t.Errorf("CLOSED reason should contain 'rate-limited', got: %s", reason)
				}
			}
			rateLimited++
		}
	}

	if rateLimited == 0 {
		t.Error("Expected some REQs to be rate limited, but none were")
	}

	if succeeded == 0 {
		t.Error("Expected some REQs to succeed, but none did")
	}

	t.Logf("Results: %d succeeded, %d rate-limited", succeeded, rateLimited)
}

func TestRateLimitMetricsInHealth(t *testing.T) {
	_, _, cleanup, baseURL := setupRelay(t)
	defer cleanup()

	// First trigger some rate limiting
	client, err := testutil.NewWSClient(strings.Replace(baseURL, "http://", "ws://", 1) + "/")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	filter := &event.Filter{Kinds: []int{1}}
	for i := 0; i < 25; i++ {
		client.SendReq(fmt.Sprintf("rl-sub-%d", i), filter)
	}
	// Drain responses
	time.Sleep(500 * time.Millisecond)
	client.Close()

	// Check health endpoint
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Failed to request health endpoint: %v", err)
	}
	defer resp.Body.Close()

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	if health.RateLimitedCount == 0 {
		t.Error("Expected rate_limited_count > 0 after burst, got 0")
	}

	t.Logf("Health reports %d rate-limited requests", health.RateLimitedCount)
}

func TestMaxConcurrentSubscriptions(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	filter := &event.Filter{Kinds: []int{1}}

	// Open 20 subscriptions (the max) in two batches to stay within rate limit burst
	for i := 0; i < 20; i++ {
		subID := fmt.Sprintf("max-sub-%d", i)
		if err := client.SendReq(subID, filter); err != nil {
			t.Fatalf("Failed to send REQ %d: %v", i, err)
		}
		// Drain EOSE
		if err := client.ExpectEOSE(subID, 2*time.Second); err != nil {
			t.Fatalf("Failed to get EOSE for sub %d: %v", i, err)
		}
		// Small delay every 10 REQs to allow rate limit tokens to refill
		if i == 9 {
			time.Sleep(1100 * time.Millisecond)
		}
	}
	// Wait for tokens to refill before sending the overflow REQ
	time.Sleep(500 * time.Millisecond)

	// 21st subscription should be rejected with CLOSED
	if err := client.SendReq("overflow-sub", filter); err != nil {
		t.Fatalf("Failed to send overflow REQ: %v", err)
	}

	reason, err := client.ExpectClosed("overflow-sub", 2*time.Second)
	if err != nil {
		t.Fatalf("Expected CLOSED for overflow subscription, got error: %v", err)
	}

	if !strings.Contains(reason, "too many concurrent subscriptions") {
		t.Errorf("Expected reason about too many subscriptions, got: %s", reason)
	}

	t.Logf("Max subscription limit enforced: %s", reason)
}
