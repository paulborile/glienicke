package integration

import (
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

const (
	KindRelayList = 10002
)

func TestNIP65_RelayListMetadata(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Create test user
	user1, kp1 := testutil.MustNewTestEvent(KindTextNote, "User1 test note", nil)

	// Send initial event to establish user
	err = client.SendEvent(user1)
	assert.NoError(t, err)

	// Wait for event to be processed
	time.Sleep(50 * time.Millisecond)

	// Create a relay list event for user1
	tags := [][]string{
		{"r", "wss://alicerelay.example.com"},                // Default (read+write)
		{"r", "wss://brando-relay.com"},                      // Default (read+write)
		{"r", "wss://expensive-relay.example2.com", "write"}, // Write only
		{"r", "wss://nostr-relay.example.com", "read"},       // Read only
		{"r", "wss://invalid-relay", "invalid"},              // Invalid marker (should still be accepted)
	}

	relayListEvent, _ := testutil.NewTestEventWithKey(kp1, KindRelayList, "", tags)
	// Set a realistic timestamp
	relayListEvent.CreatedAt = time.Now().Unix()
	// Re-sign the event with the new timestamp
	kp1.SignEvent(relayListEvent)

	// Send the relay list event
	err = client.SendEvent(relayListEvent)
	assert.NoError(t, err)

	// Expect OK response
	accepted, msg, err := client.ExpectOK(relayListEvent.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Relay list event should be accepted")
	assert.Empty(t, msg, "Message should be empty")

	// Give the relay a moment to process the relay list
	time.Sleep(50 * time.Millisecond)

	// Test 1: Query for user1's relay list
	filter := &event.Filter{
		Authors: []string{user1.PubKey},
		Kinds:   []int{KindRelayList},
		Limit:   func() *int { l := 1; return &l }(),
	}
	err = client.SendReq("relay-list-sub", filter)
	assert.NoError(t, err)

	// Should receive the relay list event
	events, err := client.CollectEvents("relay-list-sub", 2*time.Second)
	assert.NoError(t, err)
	assert.Len(t, events, 1, "Should receive exactly one relay list event")

	receivedRelayListEvent := events[0]
	assert.Equal(t, KindRelayList, receivedRelayListEvent.Kind)
	assert.Equal(t, "", receivedRelayListEvent.Content, "Relay list content should be empty")
	assert.Equal(t, user1.PubKey, receivedRelayListEvent.PubKey)

	// Verify the relay list contains the expected r tags
	assert.Len(t, receivedRelayListEvent.Tags, 5, "Should have 5 r tags")

	// Check first r tag (default - read+write)
	assert.Equal(t, "r", receivedRelayListEvent.Tags[0][0])
	assert.Equal(t, "wss://alicerelay.example.com", receivedRelayListEvent.Tags[0][1])
	assert.Len(t, receivedRelayListEvent.Tags[0], 2, "Default tag should have only 2 elements")

	// Check second r tag (default - read+write)
	assert.Equal(t, "r", receivedRelayListEvent.Tags[1][0])
	assert.Equal(t, "wss://brando-relay.com", receivedRelayListEvent.Tags[1][1])
	assert.Len(t, receivedRelayListEvent.Tags[1], 2, "Default tag should have only 2 elements")

	// Check third r tag (write only)
	assert.Equal(t, "r", receivedRelayListEvent.Tags[2][0])
	assert.Equal(t, "wss://expensive-relay.example2.com", receivedRelayListEvent.Tags[2][1])
	assert.Equal(t, "write", receivedRelayListEvent.Tags[2][2])

	// Check fourth r tag (read only)
	assert.Equal(t, "r", receivedRelayListEvent.Tags[3][0])
	assert.Equal(t, "wss://nostr-relay.example.com", receivedRelayListEvent.Tags[3][1])
	assert.Equal(t, "read", receivedRelayListEvent.Tags[3][2])

	// Check fifth r tag (invalid marker - should still be stored)
	assert.Equal(t, "r", receivedRelayListEvent.Tags[4][0])
	assert.Equal(t, "wss://invalid-relay", receivedRelayListEvent.Tags[4][1])
	assert.Equal(t, "invalid", receivedRelayListEvent.Tags[4][2])

	// Test 2: Create a new relay list that should replace the old one
	newTags := [][]string{
		{"r", "wss://new-relay1.example.com", "read"},  // Read only
		{"r", "wss://new-relay2.example.com", "write"}, // Write only
	}

	// Wait a moment to ensure different CreatedAt timestamp
	time.Sleep(10 * time.Millisecond)

	newRelayListEvent, _ := testutil.NewTestEventWithKey(kp1, KindRelayList, "", newTags)
	// Set a newer timestamp
	newRelayListEvent.CreatedAt = relayListEvent.CreatedAt + 1
	// Re-sign the event with the new timestamp
	kp1.SignEvent(newRelayListEvent)

	// Send the new relay list event
	err = client.SendEvent(newRelayListEvent)
	assert.NoError(t, err)

	// Expect OK response
	accepted, msg, err = client.ExpectOK(newRelayListEvent.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "New relay list event should be accepted")
	assert.Empty(t, msg, "Message should be empty")

	// Give the relay a moment to process
	time.Sleep(50 * time.Millisecond)

	// Query for user1's relay list again - should only get the newest one
	err = client.SendReq("relay-list-sub-2", filter)
	assert.NoError(t, err)

	events, err = client.CollectEvents("relay-list-sub-2", 2*time.Second)
	assert.NoError(t, err)
	assert.Len(t, events, 1, "Should receive exactly one relay list event (the newest)")

	newestRelayListEvent := events[0]
	assert.Equal(t, newRelayListEvent.ID, newestRelayListEvent.ID, "Should receive the newest relay list event")
	assert.Len(t, newestRelayListEvent.Tags, 2, "New relay list should have 2 r tags")
}

func TestNIP65_RelayListValidation(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Test invalid relay list events
	testCases := []struct {
		name         string
		tags         [][]string
		content      string
		shouldAccept bool
	}{
		{
			name:         "Valid relay list with empty content",
			tags:         [][]string{{"r", "wss://relay.example.com"}},
			content:      "",
			shouldAccept: true,
		},
		{
			name:         "Valid relay list with read marker",
			tags:         [][]string{{"r", "wss://relay.example.com", "read"}},
			content:      "",
			shouldAccept: true,
		},
		{
			name:         "Valid relay list with write marker",
			tags:         [][]string{{"r", "wss://relay.example.com", "write"}},
			content:      "",
			shouldAccept: true,
		},
		{
			name:         "Valid relay list with multiple r tags",
			tags:         [][]string{{"r", "wss://relay1.example.com"}, {"r", "wss://relay2.example.com", "read"}},
			content:      "",
			shouldAccept: true,
		},
		{
			name:         "Invalid relay list with non-empty content",
			tags:         [][]string{{"r", "wss://relay.example.com"}},
			content:      "some content",
			shouldAccept: false,
		},
		{
			name:         "Invalid relay list with no r tags",
			tags:         [][]string{{"e", "some_event_id"}},
			content:      "",
			shouldAccept: false,
		},
		{
			name:         "Invalid r tag missing relay URL",
			tags:         [][]string{{"r"}},
			content:      "",
			shouldAccept: false,
		},
		{
			name:         "Valid relay list with minimal r tag",
			tags:         [][]string{{"r", "wss://minimal.example.com"}},
			content:      "",
			shouldAccept: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, kp := testutil.MustNewTestEvent(KindTextNote, "Test note", nil)

			relayListEvent, _ := testutil.NewTestEventWithKey(kp, KindRelayList, tc.content, tc.tags)
			relayListEvent.CreatedAt = time.Now().Unix()
			kp.SignEvent(relayListEvent)

			// Send the relay list event
			err = client.SendEvent(relayListEvent)
			assert.NoError(t, err)

			// Expect OK response
			accepted, msg, err := client.ExpectOK(relayListEvent.ID, 2*time.Second)
			assert.NoError(t, err)

			if tc.shouldAccept {
				assert.True(t, accepted, "Event should be accepted: %s", tc.name)
				assert.Empty(t, msg, "Message should be empty for accepted event: %s", tc.name)
			} else {
				assert.False(t, accepted, "Event should be rejected: %s", tc.name)
				assert.NotEmpty(t, msg, "Message should explain rejection: %s", tc.name)
			}
		})
	}
}

func TestNIP65_RelayListExtraction(t *testing.T) {
	// This test verifies the extraction logic works correctly
	// We'll create a relay list event and verify we can extract the relays correctly

	tags := [][]string{
		{"r", "wss://readwrite.example.com"},           // Default (read+write)
		{"r", "wss://read-only.example.com", "read"},   // Read only
		{"r", "wss://write-only.example.com", "write"}, // Write only
		{"r", "wss://another.example.com"},             // Default (read+write)
	}

	_, kp := testutil.MustNewTestEvent(KindTextNote, "Test note", nil)
	relayListEvent, _ := testutil.NewTestEventWithKey(kp, KindRelayList, "", tags)

	// Test extracting read relays
	readRelays := extractReadRelays(relayListEvent)
	expectedReadRelays := []string{
		"wss://readwrite.example.com",
		"wss://read-only.example.com",
		"wss://another.example.com",
	}
	assert.Equal(t, expectedReadRelays, readRelays)

	// Test extracting write relays
	writeRelays := extractWriteRelays(relayListEvent)
	expectedWriteRelays := []string{
		"wss://readwrite.example.com",
		"wss://write-only.example.com",
		"wss://another.example.com",
	}
	assert.Equal(t, expectedWriteRelays, writeRelays)

	// Test extracting all relays
	allRelays := extractAllRelays(relayListEvent)
	expectedAllRelays := []string{
		"wss://readwrite.example.com",
		"wss://read-only.example.com",
		"wss://write-only.example.com",
		"wss://another.example.com",
	}
	assert.Equal(t, expectedAllRelays, allRelays)
}

// Helper functions for extracting relays from a relay list event
// These would be part of the NIP-65 implementation
func extractReadRelays(evt *event.Event) []string {
	var relays []string
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "r" {
			// If no marker or marker is "read", this is a read relay
			if len(tag) == 2 || tag[2] == "read" {
				relays = append(relays, tag[1])
			}
		}
	}
	return relays
}

func extractWriteRelays(evt *event.Event) []string {
	var relays []string
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "r" {
			// If no marker or marker is "write", this is a write relay
			if len(tag) == 2 || tag[2] == "write" {
				relays = append(relays, tag[1])
			}
		}
	}
	return relays
}

func extractAllRelays(evt *event.Event) []string {
	var relays []string
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "r" {
			relays = append(relays, tag[1])
		}
	}
	return relays
}
