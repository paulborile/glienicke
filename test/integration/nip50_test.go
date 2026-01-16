package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
)

func TestNIP50_Search(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Create test events with searchable content
	evt1, _ := testutil.MustNewTestEvent(1, "Hello world, this is a test about blockchain technology", nil)
	evt2, _ := testutil.MustNewTestEvent(1, "Another post about artificial intelligence and machine learning", nil)
	evt3, _ := testutil.MustNewTestEvent(1, "Discussion about cryptocurrency and decentralized finance", [][]string{{"t", "blockchain"}, {"t", "crypto"}})
	evt4, _ := testutil.MustNewTestEvent(1, "General discussion about information technology trends", nil)

	testEvents := []*event.Event{evt1, evt2, evt3, evt4}

	// Save test events to store
	for i, evt := range testEvents {
		if err := client.SendEvent(evt); err != nil {
			t.Fatalf("Failed to send test event %d: %v", i, err)
		}
		accepted, msg, err := client.ExpectOK(evt.ID, 2*time.Second)
		if err != nil {
			t.Fatalf("Failed to receive OK for test event %d: %v", i, err)
		}
		if !accepted {
			t.Fatalf("Test event %d was not accepted: %s", i, msg)
		}
		t.Logf("Successfully saved test event %d: %s - content: %s", i, evt.ID, evt.Content)
	}

	// Test 1: Basic search by content
	t.Run("BasicContentSearch", func(t *testing.T) {
		subID := "search1"
		filter := &event.Filter{
			Search: "blockchain",
		}

		// Send REQ with search filter
		if err := client.SendReq(subID, filter); err != nil {
			t.Fatalf("Failed to send REQ: %v", err)
		}

		// Wait for events
		events, err := client.CollectEvents(subID, 2*time.Second)
		if err != nil {
			t.Fatalf("Failed to receive events: %v", err)
		}

		// Should find events containing "blockchain"
		expectedCount := 2 // events with "blockchain" in content or tags
		if len(events) != expectedCount {
			t.Errorf("Expected %d events for 'blockchain' search, got %d", expectedCount, len(events))
		}

		// Verify all returned events contain the search term
		for _, evt := range events {
			if !containsIgnoreCase(evt.Content, "blockchain") && !hasTagWithValue(evt.Tags, "t", "blockchain") {
				t.Errorf("Event %s does not contain 'blockchain' in content or tags", evt.ID)
			}
		}
	})

	// Test 2: Search by tag
	t.Run("TagSearch", func(t *testing.T) {
		subID := "search2"
		filter := &event.Filter{
			Search: "crypto",
		}

		// Send REQ with search filter
		if err := client.SendReq(subID, filter); err != nil {
			t.Fatalf("Failed to send REQ: %v", err)
		}

		// Wait for events
		events, err := client.CollectEvents(subID, 2*time.Second)
		if err != nil {
			t.Fatalf("Failed to receive events: %v", err)
		}

		// Should find events with "crypto" in content or tags
		if len(events) < 1 {
			t.Errorf("Expected at least 1 event for 'crypto' search, got %d", len(events))
		}

		// Verify all returned events contain the search term
		for _, evt := range events {
			if !containsIgnoreCase(evt.Content, "crypto") && !hasTagWithValue(evt.Tags, "t", "crypto") {
				t.Errorf("Event %s does not contain 'crypto' in content or tags", evt.ID)
			}
		}
	})

	// Test 3: Search with negation
	t.Run("NegationSearch", func(t *testing.T) {
		subID := "search3"
		filter := &event.Filter{
			Search: "technology -blockchain",
		}

		// Send REQ with search filter
		if err := client.SendReq(subID, filter); err != nil {
			t.Fatalf("Failed to send REQ: %v", err)
		}

		// Wait for events
		events, err := client.CollectEvents(subID, 2*time.Second)
		if err != nil {
			t.Fatalf("Failed to receive events: %v", err)
		}

		// Should find events containing "technology" but not "blockchain"
		if len(events) < 1 {
			t.Errorf("Expected at least 1 event for 'technology -blockchain' search, got %d", len(events))
		}

		// Verify all returned events contain "technology" but not "blockchain"
		for _, evt := range events {
			if !containsIgnoreCase(evt.Content, "technology") {
				t.Errorf("Event %s does not contain 'technology'", evt.ID)
			}
			if containsIgnoreCase(evt.Content, "blockchain") {
				t.Errorf("Event %s contains 'blockchain' which should be excluded", evt.ID)
			}
		}
	})
}

// Helper functions for search testing

func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}

func hasTagWithValue(tags [][]string, tagName, tagValue string) bool {
	for _, tag := range tags {
		if len(tag) >= 2 && tag[0] == tagName {
			if strings.EqualFold(tag[1], tagValue) {
				return true
			}
		}
	}
	return false
}
