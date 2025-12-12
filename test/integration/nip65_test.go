package integration

import (
	"context"
	"testing"

	"github.com/paul/glienicke/internal/store/memory"
	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/nips/nip65"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNIP65Integration(t *testing.T) {
	ctx := context.Background()

	// Create a test store
	store := createTestStore(t)
	defer store.Close()

	// Generate test keypair
	kp := testutil.MustGenerateKeyPair()

	t.Run("Complete relay list workflow", func(t *testing.T) {
		// Create a valid NIP-65 relay list event
		content := ""
		tags := [][]string{
			{"r", "wss://read-only.example.com", "read"},
			{"r", "wss://write-only.example.com", "write"},
			{"r", "wss://read-write.example.com"},
			{"r", "wss://another-read.example.com", "read"},
		}

		evt, err := testutil.NewTestEventWithKey(kp, nip65.KindRelayList, content, tags)
		require.NoError(t, err)

		// Validate it's a relay list event
		assert.True(t, nip65.IsRelayListEvent(evt))

		// Validate the event structure
		err = nip65.ValidateRelayList(evt)
		assert.NoError(t, err)

		// Store the event
		err = store.SaveEvent(ctx, evt)
		assert.NoError(t, err)

		// Extract relay information
		relays, err := nip65.ExtractRelayInfo(evt)
		assert.NoError(t, err)
		assert.Len(t, relays, 4)

		// Test read relay extraction
		readRelays, err := nip65.GetReadRelays(evt)
		assert.NoError(t, err)
		assert.Contains(t, readRelays, "wss://read-only.example.com")
		assert.Contains(t, readRelays, "wss://read-write.example.com")
		assert.Contains(t, readRelays, "wss://another-read.example.com")
		assert.NotContains(t, readRelays, "wss://write-only.example.com")

		// Test write relay extraction
		writeRelays, err := nip65.GetWriteRelays(evt)
		assert.NoError(t, err)
		assert.Contains(t, writeRelays, "wss://write-only.example.com")
		assert.Contains(t, writeRelays, "wss://read-write.example.com")
		assert.NotContains(t, writeRelays, "wss://read-only.example.com")
		assert.NotContains(t, writeRelays, "wss://another-read.example.com")
	})

	t.Run("Query stored relay list events", func(t *testing.T) {
		// Query for the author's relay list
		limit := 1
		filter := &event.Filter{
			Authors: []string{kp.PubKeyHex},
			Kinds:   []int{nip65.KindRelayList},
			Limit:   &limit,
		}

		events, err := store.QueryEvents(ctx, []*event.Filter{filter})
		assert.NoError(t, err)
		assert.Len(t, events, 1)

		// Verify it's our relay list event
		storedEvt := events[0]
		assert.Equal(t, kp.PubKeyHex, storedEvt.PubKey)
		assert.Equal(t, nip65.KindRelayList, storedEvt.Kind)

		// Verify we can extract relay info from stored event
		relays, err := nip65.ExtractRelayInfo(storedEvt)
		assert.NoError(t, err)
		assert.Len(t, relays, 4)
	})

	t.Run("Replaceable event behavior", func(t *testing.T) {
		// Create a new relay list event with different relays
		newTags := [][]string{
			{"r", "wss://new-read.example.com", "read"},
			{"r", "wss://new-write.example.com", "write"},
		}

		newEvt, err := testutil.NewTestEventWithKey(kp, nip65.KindRelayList, "", newTags)
		require.NoError(t, err)

		// Store the new event
		err = store.SaveEvent(ctx, newEvt)
		assert.NoError(t, err)

		// Query again - should get the latest event for this (author, kind) pair
		limit := 10
		filter := &event.Filter{
			Authors: []string{kp.PubKeyHex},
			Kinds:   []int{nip65.KindRelayList},
			Limit:   &limit, // Get all to see replaceable behavior
		}

		events, err := store.QueryEvents(ctx, []*event.Filter{filter})
		assert.NoError(t, err)

		// The storage layer should handle replaceable events by keeping only the latest
		// This depends on the storage implementation, but ideally we'd only have 1 event
		foundLatest := false
		for _, evt := range events {
			if evt.CreatedAt == newEvt.CreatedAt && evt.ID == newEvt.ID {
				foundLatest = true
				break
			}
		}
		assert.True(t, foundLatest, "Latest relay list event should be stored")
	})

	t.Run("Invalid relay list rejection", func(t *testing.T) {
		// Test various invalid relay list scenarios
		invalidScenarios := []struct {
			name    string
			content string
			tags    [][]string
		}{
			{
				name:    "Non-empty content",
				content: "invalid content",
				tags:    [][]string{{"r", "wss://relay.example.com"}},
			},
			{
				name:    "Invalid URL scheme",
				content: "",
				tags:    [][]string{{"r", "https://relay.example.com"}},
			},
			{
				name:    "Invalid mode",
				content: "",
				tags:    [][]string{{"r", "wss://relay.example.com", "invalid"}},
			},
		}

		for _, scenario := range invalidScenarios {
			t.Run(scenario.name, func(t *testing.T) {
				evt, err := testutil.NewTestEventWithKey(kp, nip65.KindRelayList, scenario.content, scenario.tags)
				require.NoError(t, err)

				// Should fail validation
				err = nip65.ValidateRelayList(evt)
				assert.Error(t, err)

				// Should still be storable if we skip validation (storage layer doesn't validate)
				// But the relay logic should prevent invalid events
			})
		}
	})

	t.Run("URL normalization", func(t *testing.T) {
		testURLs := []struct {
			input    string
			expected string
			hasError bool
		}{
			{"wss://relay.example.com/", "wss://relay.example.com", false},
			{"wss://relay.example.com///", "wss://relay.example.com", false},
			{"wss://relay.example.com/path/", "wss://relay.example.com/path", false},
			{" wss://relay.example.com ", "wss://relay.example.com", false},
			{"not-a-url", "", true},
		}

		for _, test := range testURLs {
			normalized, err := nip65.NormalizeRelayURL(test.input)
			if test.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, normalized)
			}
		}
	})

	t.Run("Mixed tags handling", func(t *testing.T) {
		// Test that non-r tags are properly ignored
		tags := [][]string{
			{"e", "event_id"},
			{"p", "pubkey"},
			{"t", "hashtag"},
			{"r", "wss://relay1.example.com", "read"},
			{"r", "wss://relay2.example.com"},
			{"d", "identifier"},
		}

		evt, err := testutil.NewTestEventWithKey(kp, nip65.KindRelayList, "", tags)
		require.NoError(t, err)

		// Should validate successfully
		err = nip65.ValidateRelayList(evt)
		assert.NoError(t, err)

		// Should extract only r tags
		relays, err := nip65.ExtractRelayInfo(evt)
		assert.NoError(t, err)
		assert.Len(t, relays, 2)

		relayURLs := make([]string, len(relays))
		for i, relay := range relays {
			relayURLs[i] = relay.URL
		}
		assert.Contains(t, relayURLs, "wss://relay1.example.com")
		assert.Contains(t, relayURLs, "wss://relay2.example.com")
	})
}

// Helper function to create a test store
func createTestStore(t *testing.T) *memory.Store {
	// Use the same test store creation pattern as other integration tests
	store := memory.New()
	return store
}
