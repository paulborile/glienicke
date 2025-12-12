package nip65

import (
	"testing"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

func TestValidateRelayList(t *testing.T) {
	testCases := []struct {
		name        string
		tags        [][]string
		content     string
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid relay list with default r tag",
			tags: [][]string{
				{"r", "wss://relay.example.com"},
			},
			content:     "",
			expectError: false,
		},
		{
			name: "Valid relay list with read marker",
			tags: [][]string{
				{"r", "wss://relay.example.com", "read"},
			},
			content:     "",
			expectError: false,
		},
		{
			name: "Valid relay list with write marker",
			tags: [][]string{
				{"r", "wss://relay.example.com", "write"},
			},
			content:     "",
			expectError: false,
		},
		{
			name: "Valid relay list with multiple relays",
			tags: [][]string{
				{"r", "wss://relay1.example.com"},
				{"r", "wss://relay2.example.com", "read"},
				{"r", "wss://relay3.example.com", "write"},
			},
			content:     "",
			expectError: false,
		},
		{
			name: "Valid relay list with mixed case marker",
			tags: [][]string{
				{"r", "wss://relay.example.com", "READ"},
			},
			content:     "",
			expectError: false,
		},
		{
			name: "Invalid relay list with non-empty content",
			tags: [][]string{
				{"r", "wss://relay.example.com"},
			},
			content:     "some content",
			expectError: true,
			errorMsg:    "relay list (kind 10002) must have empty content",
		},
		{
			name: "Invalid relay list with empty relay URL",
			tags: [][]string{
				{"r", ""},
			},
			content:     "",
			expectError: true,
			errorMsg:    "r tag relay URL cannot be empty",
		},
		{
			name: "Invalid relay list with missing relay URL",
			tags: [][]string{
				{"r"},
			},
			content:     "",
			expectError: true,
			errorMsg:    "r tag must have at least 2 elements",
		},
		{
			name: "Invalid relay list with invalid URL scheme",
			tags: [][]string{
				{"r", "http://relay.example.com"},
			},
			content:     "",
			expectError: true,
			errorMsg:    "r tag relay URL should start with ws:// or wss://",
		},
		{
			name: "Invalid relay list with no r tags",
			tags: [][]string{
				{"p", "0000000000000000000000000000000000000000000000000000000000000000"},
			},
			content:     "",
			expectError: true,
			errorMsg:    "relay list (kind 10002) should have at least one r tag",
		},
		{
			name: "Valid relay list with whitespace-only URL (should be trimmed and fail)",
			tags: [][]string{
				{"r", "   "},
			},
			content:     "",
			expectError: true,
			errorMsg:    "r tag relay URL cannot be empty",
		},
		{
			name: "Valid relay list with unknown marker (should be accepted)",
			tags: [][]string{
				{"r", "wss://relay.example.com", "unknown"},
			},
			content:     "",
			expectError: false,
		},
		{
			name: "Not a relay list event (should return nil)",
			tags: [][]string{
				{"r", "wss://relay.example.com"},
			},
			content:     "",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evt := &event.Event{
				Kind:    KindRelayList,
				Tags:    tc.tags,
				Content: tc.content,
			}

			// For the test case "Not a relay list event", use kind 1
			if tc.name == "Not a relay list event (should return nil)" {
				evt.Kind = 1
			}

			err := ValidateRelayList(evt)

			if tc.expectError {
				assert.Error(t, err, "Expected validation error")
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg, "Error message mismatch")
				}
			} else {
				assert.NoError(t, err, "Expected no validation error")
			}
		})
	}
}

func TestExtractRelayInfo(t *testing.T) {
	tags := [][]string{
		{"r", "wss://readwrite.example.com"},             // Default (read+write)
		{"r", "wss://read-only.example.com", "read"},     // Read only
		{"r", "wss://write-only.example.com", "write"},   // Write only
		{"r", "wss://unknown-marker.example.com", "foo"}, // Unknown marker (read+write)
		{"r", "wss://spaced.example.com", "   "},         // Whitespace marker (read+write)
	}

	evt := &event.Event{
		Kind: KindRelayList,
		Tags: tags,
	}

	relayInfo := ExtractRelayInfo(evt)

	expectedRelayInfo := []RelayInfo{
		{
			URL:    "wss://readwrite.example.com",
			Read:   true,
			Write:  true,
			Marker: "",
		},
		{
			URL:    "wss://read-only.example.com",
			Read:   true,
			Write:  false,
			Marker: "read",
		},
		{
			URL:    "wss://write-only.example.com",
			Read:   false,
			Write:  true,
			Marker: "write",
		},
		{
			URL:    "wss://unknown-marker.example.com",
			Read:   true,
			Write:  true,
			Marker: "foo",
		},
		{
			URL:    "wss://spaced.example.com",
			Read:   true,
			Write:  true,
			Marker: "   ",
		},
	}

	assert.Equal(t, expectedRelayInfo, relayInfo, "Relay info extraction mismatch")
}

func TestExtractRelayInfoFromNonRelayListEvent(t *testing.T) {
	evt := &event.Event{
		Kind: 1, // Not a relay list
		Tags: [][]string{
			{"r", "wss://relay.example.com"},
		},
	}

	relayInfo := ExtractRelayInfo(evt)
	assert.Empty(t, relayInfo, "Should return empty slice for non-relay list event")
}

func TestExtractReadRelays(t *testing.T) {
	tags := [][]string{
		{"r", "wss://readwrite.example.com"},           // Default (read)
		{"r", "wss://read-only.example.com", "read"},   // Read only
		{"r", "wss://write-only.example.com", "write"}, // Write only (not included)
		{"r", "wss://unknown.example.com", "unknown"},  // Unknown marker (read+write)
		{"r", "wss://another.example.com"},             // Default (read)
	}

	evt := &event.Event{
		Kind: KindRelayList,
		Tags: tags,
	}

	readRelays := ExtractReadRelays(evt)
	expectedReadRelays := []string{
		"wss://readwrite.example.com",
		"wss://read-only.example.com",
		"wss://unknown.example.com",
		"wss://another.example.com",
	}

	assert.Equal(t, expectedReadRelays, readRelays, "Read relays extraction mismatch")
}

func TestExtractWriteRelays(t *testing.T) {
	tags := [][]string{
		{"r", "wss://readwrite.example.com"},           // Default (write)
		{"r", "wss://read-only.example.com", "read"},   // Read only (not included)
		{"r", "wss://write-only.example.com", "write"}, // Write only
		{"r", "wss://unknown.example.com", "unknown"},  // Unknown marker (read+write)
		{"r", "wss://another.example.com"},             // Default (write)
	}

	evt := &event.Event{
		Kind: KindRelayList,
		Tags: tags,
	}

	writeRelays := ExtractWriteRelays(evt)
	expectedWriteRelays := []string{
		"wss://readwrite.example.com",
		"wss://write-only.example.com",
		"wss://unknown.example.com",
		"wss://another.example.com",
	}

	assert.Equal(t, expectedWriteRelays, writeRelays, "Write relays extraction mismatch")
}

func TestExtractAllRelays(t *testing.T) {
	tags := [][]string{
		{"r", "wss://relay1.example.com"},
		{"r", "wss://relay2.example.com", "read"},
		{"r", "wss://relay3.example.com", "write"},
		{"r", "wss://relay4.example.com", "unknown"},
	}

	evt := &event.Event{
		Kind: KindRelayList,
		Tags: tags,
	}

	allRelays := ExtractAllRelays(evt)
	expectedAllRelays := []string{
		"wss://relay1.example.com",
		"wss://relay2.example.com",
		"wss://relay3.example.com",
		"wss://relay4.example.com",
	}

	assert.Equal(t, expectedAllRelays, allRelays, "All relays extraction mismatch")
}

func TestExtractFromNonRelayListEvents(t *testing.T) {
	evt := &event.Event{
		Kind: 1, // Not a relay list
		Tags: [][]string{
			{"r", "wss://relay.example.com"},
		},
	}

	assert.Empty(t, ExtractReadRelays(evt), "Should return empty for non-relay list event")
	assert.Empty(t, ExtractWriteRelays(evt), "Should return empty for non-relay list event")
	assert.Empty(t, ExtractAllRelays(evt), "Should return empty for non-relay list event")
}

func TestIsRelayListEvent(t *testing.T) {
	relayListEvt := &event.Event{Kind: KindRelayList}
	nonRelayListEvt := &event.Event{Kind: 1}

	assert.True(t, IsRelayListEvent(relayListEvt), "Should identify relay list event")
	assert.False(t, IsRelayListEvent(nonRelayListEvt), "Should not identify non-relay list event")
}

func TestHandleRelayList(t *testing.T) {
	// This test is more of an integration test since we need a mock storage
	// We'll just test the validation part here

	_, kp := testutil.MustNewTestEvent(KindRelayList, "", [][]string{
		{"r", "wss://relay.example.com"},
	})

	// Create a valid relay list event
	evt, err := testutil.NewTestEventWithKey(kp, KindRelayList, "", [][]string{
		{"r", "wss://relay.example.com"},
	})
	assert.NoError(t, err)

	// Test handling - this should not return an error for a valid event
	err = HandleRelayList(nil, nil, evt)
	assert.NoError(t, err, "Valid relay list event should be handled without error")

	// Test with invalid event (non-empty content)
	invalidEvt, err := testutil.NewTestEventWithKey(kp, KindRelayList, "invalid content", [][]string{
		{"r", "wss://relay.example.com"},
	})
	assert.NoError(t, err)

	err = HandleRelayList(nil, nil, invalidEvt)
	assert.Error(t, err, "Invalid relay list event should return error")
	assert.Contains(t, err.Error(), "invalid relay list event", "Error should mention validation failure")

	// Test with non-relay list event
	nonRelayListEvt, err := testutil.NewTestEventWithKey(kp, 1, "test", nil)
	assert.NoError(t, err)

	err = HandleRelayList(nil, nil, nonRelayListEvt)
	assert.NoError(t, err, "Non-relay list event should be handled without error")
}

func TestRelayURLWhitespaceHandling(t *testing.T) {
	evt := &event.Event{
		Kind: KindRelayList,
		Tags: [][]string{
			{"r", "  wss://whitespace.example.com  "},
		},
	}

	relayInfo := ExtractRelayInfo(evt)
	assert.Len(t, relayInfo, 1, "Should extract one relay")
	assert.Equal(t, "wss://whitespace.example.com", relayInfo[0].URL, "Should trim whitespace from URL")

	readRelays := ExtractReadRelays(evt)
	assert.Len(t, readRelays, 1, "Should extract one read relay")
	assert.Equal(t, "wss://whitespace.example.com", readRelays[0], "Should trim whitespace from read relay URL")
}
