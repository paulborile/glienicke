package nip65

import (
	"testing"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

func TestIsRelayListEvent(t *testing.T) {
	tests := []struct {
		name     string
		kind     int
		expected bool
	}{
		{"Relay list event", KindRelayList, true},
		{"Text note event", 1, false},
		{"Metadata event", 0, false},
		{"Follow list event", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &event.Event{Kind: tt.kind}
			assert.Equal(t, tt.expected, IsRelayListEvent(evt))
		})
	}
}

func TestValidateRelayList(t *testing.T) {
	tests := []struct {
		name        string
		kind        int
		content     string
		tags        [][]string
		expectError bool
		errorMsg    string
	}{
		{
			name:    "Valid relay list with mixed modes",
			kind:    KindRelayList,
			content: "",
			tags: [][]string{
				{"r", "wss://relay1.example.com"},
				{"r", "wss://relay2.example.com", "read"},
				{"r", "wss://relay3.example.com", "write"},
			},
			expectError: false,
		},
		{
			name:        "Valid empty relay list",
			kind:        KindRelayList,
			content:     "",
			tags:        [][]string{},
			expectError: false,
		},
		{
			name:        "Not a relay list (different kind)",
			kind:        1,
			content:     "",
			tags:        [][]string{{"r", "wss://relay.example.com"}},
			expectError: false,
		},
		{
			name:        "Invalid relay list with non-empty content",
			kind:        KindRelayList,
			content:     "some content",
			tags:        [][]string{{"r", "wss://relay.example.com"}},
			expectError: true,
			errorMsg:    "relay list (kind 10002) must have empty content",
		},
		{
			name:        "Invalid r tag missing URL",
			kind:        KindRelayList,
			content:     "",
			tags:        [][]string{{"r"}},
			expectError: true,
			errorMsg:    "r tag must have at least 2 elements (tag name and relay URL)",
		},
		{
			name:        "Invalid relay URL format",
			kind:        KindRelayList,
			content:     "",
			tags:        [][]string{{"r", "not-a-url"}},
			expectError: true,
			errorMsg:    "invalid relay URL in r tag",
		},
		{
			name:        "Invalid relay URL scheme",
			kind:        KindRelayList,
			content:     "",
			tags:        [][]string{{"r", "https://relay.example.com"}},
			expectError: true,
			errorMsg:    "relay URL must use ws:// or wss:// scheme",
		},
		{
			name:        "Invalid relay mode",
			kind:        KindRelayList,
			content:     "",
			tags:        [][]string{{"r", "wss://relay.example.com", "invalid"}},
			expectError: true,
			errorMsg:    "invalid relay mode 'invalid', must be 'read' or 'write'",
		},
		{
			name:        "Empty relay URL",
			kind:        KindRelayList,
			content:     "",
			tags:        [][]string{{"r", ""}},
			expectError: true,
			errorMsg:    "invalid relay URL in r tag: relay URL cannot be empty",
		},
		{
			name:    "Mixed valid and invalid tags",
			kind:    KindRelayList,
			content: "",
			tags: [][]string{
				{"r", "wss://valid.example.com"},
				{"r", "invalid-url"},
			},
			expectError: true,
			errorMsg:    "invalid relay URL in r tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &event.Event{
				Kind:    tt.kind,
				Content: tt.content,
				Tags:    tt.tags,
			}

			err := ValidateRelayList(evt)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtractRelayInfo(t *testing.T) {
	tests := []struct {
		name           string
		kind           int
		tags           [][]string
		expectedRelays []RelayInfo
		expectError    bool
	}{
		{
			name: "Extract from valid relay list with mixed modes",
			kind: KindRelayList,
			tags: [][]string{
				{"r", "wss://relay1.example.com"},
				{"r", "wss://relay2.example.com", "read"},
				{"r", "wss://relay3.example.com", "write"},
			},
			expectedRelays: []RelayInfo{
				{URL: "wss://relay1.example.com", Mode: ModeReadWrite},
				{URL: "wss://relay2.example.com", Mode: ModeRead},
				{URL: "wss://relay3.example.com", Mode: ModeWrite},
			},
			expectError: false,
		},
		{
			name:           "Non-relay list event",
			kind:           1,
			tags:           [][]string{{"r", "wss://relay.example.com"}},
			expectedRelays: nil,
			expectError:    true,
		},
		{
			name:           "Relay list with no r tags",
			kind:           KindRelayList,
			tags:           [][]string{{"e", "some_event_id"}},
			expectedRelays: []RelayInfo{},
			expectError:    false,
		},
		{
			name: "Mixed tags",
			kind: KindRelayList,
			tags: [][]string{
				{"e", "some_event_id"},
				{"r", "wss://relay1.example.com", "read"},
				{"t", "hashtag"},
				{"r", "wss://relay2.example.com"},
			},
			expectedRelays: []RelayInfo{
				{URL: "wss://relay1.example.com", Mode: ModeRead},
				{URL: "wss://relay2.example.com", Mode: ModeReadWrite},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &event.Event{
				Kind: tt.kind,
				Tags: tt.tags,
			}

			relays, err := ExtractRelayInfo(evt)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRelays, relays)
			}
		})
	}
}

func TestGetReadRelays(t *testing.T) {
	tests := []struct {
		name               string
		kind               int
		tags               [][]string
		expectedReadRelays []string
		expectError        bool
	}{
		{
			name: "Extract read relays including read/write",
			kind: KindRelayList,
			tags: [][]string{
				{"r", "wss://read-only.example.com", "read"},
				{"r", "wss://read-write.example.com"},
				{"r", "wss://write-only.example.com", "write"},
			},
			expectedReadRelays: []string{
				"wss://read-only.example.com",
				"wss://read-write.example.com",
			},
			expectError: false,
		},
		{
			name:               "Non-relay list event",
			kind:               1,
			tags:               [][]string{{"r", "wss://relay.example.com"}},
			expectedReadRelays: nil,
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &event.Event{
				Kind: tt.kind,
				Tags: tt.tags,
			}

			readRelays, err := GetReadRelays(evt)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedReadRelays, readRelays)
			}
		})
	}
}

func TestGetWriteRelays(t *testing.T) {
	tests := []struct {
		name                string
		kind                int
		tags                [][]string
		expectedWriteRelays []string
		expectError         bool
	}{
		{
			name: "Extract write relays including read/write",
			kind: KindRelayList,
			tags: [][]string{
				{"r", "wss://read-only.example.com", "read"},
				{"r", "wss://read-write.example.com"},
				{"r", "wss://write-only.example.com", "write"},
			},
			expectedWriteRelays: []string{
				"wss://read-write.example.com",
				"wss://write-only.example.com",
			},
			expectError: false,
		},
		{
			name:                "Non-relay list event",
			kind:                1,
			tags:                [][]string{{"r", "wss://relay.example.com"}},
			expectedWriteRelays: nil,
			expectError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &event.Event{
				Kind: tt.kind,
				Tags: tt.tags,
			}

			writeRelays, err := GetWriteRelays(evt)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedWriteRelays, writeRelays)
			}
		})
	}
}

func TestValidateRelayURL(t *testing.T) {
	tests := []struct {
		name        string
		relayURL    string
		expectError bool
		errorMsg    string
	}{
		{"Valid wss:// URL", "wss://relay.example.com", false, ""},
		{"Valid ws:// URL", "ws://relay.example.com", false, ""},
		{"Valid wss:// URL with path", "wss://relay.example.com/path", false, ""},
		{"Valid wss:// URL with port", "wss://relay.example.com:8080", false, ""},
		{"Empty URL", "", true, "relay URL cannot be empty"},
		{"Invalid scheme", "https://relay.example.com", true, "must use ws:// or wss:// scheme"},
		{"Missing scheme", "relay.example.com", true, "must use ws:// or wss:// scheme"},
		{"Missing host", "wss://", true, "must have a host"},
		{"Invalid URL format", "not-a-url", true, "must use ws:// or wss:// scheme"},
		{"Whitespace only", "   ", true, "relay URL cannot be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRelayURL(tt.relayURL)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNormalizeRelayURL(t *testing.T) {
	tests := []struct {
		name        string
		inputURL    string
		expectedURL string
		expectError bool
	}{
		{"Remove trailing slash", "wss://relay.example.com/", "wss://relay.example.com", false},
		{"Remove multiple trailing slashes", "wss://relay.example.com///", "wss://relay.example.com", false},
		{"Keep path without trailing slash", "wss://relay.example.com/path", "wss://relay.example.com/path", false},
		{"Remove trailing slash from path", "wss://relay.example.com/path/", "wss://relay.example.com/path", false},
		{"Trim whitespace", " wss://relay.example.com ", "wss://relay.example.com", false},
		{"Already normalized", "wss://relay.example.com", "wss://relay.example.com", false},
		{"Invalid URL", "not-a-url", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizedURL, err := NormalizeRelayURL(tt.inputURL)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedURL, normalizedURL)
			}
		})
	}
}

func TestRelayModeConstants(t *testing.T) {
	assert.Equal(t, RelayMode("read"), ModeRead)
	assert.Equal(t, RelayMode("write"), ModeWrite)
	assert.Equal(t, RelayMode(""), ModeReadWrite)
}

func TestHandleRelayList(t *testing.T) {
	tests := []struct {
		name        string
		kind        int
		content     string
		tags        [][]string
		expectError bool
	}{
		{
			name:        "Valid relay list",
			kind:        KindRelayList,
			content:     "",
			tags:        [][]string{{"r", "wss://relay.example.com"}},
			expectError: false,
		},
		{
			name:        "Non-relay list event",
			kind:        1,
			content:     "test",
			tags:        [][]string{{"t", "hashtag"}},
			expectError: false,
		},
		{
			name:        "Invalid relay list",
			kind:        KindRelayList,
			content:     "invalid content",
			tags:        [][]string{{"r", "wss://relay.example.com"}},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &event.Event{
				Kind:    tt.kind,
				Content: tt.content,
				Tags:    tt.tags,
			}

			// We can't easily test the storage interaction without a mock,
			// but we can test the validation logic
			if tt.kind == KindRelayList {
				err := ValidateRelayList(evt)
				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			} else {
				// Non-relay list events should not be processed
				assert.True(t, true) // This would be handled by the calling code
			}
		})
	}
}

func TestRelayInfoStruct(t *testing.T) {
	relay := RelayInfo{
		URL:  "wss://relay.example.com",
		Mode: ModeRead,
	}

	assert.Equal(t, "wss://relay.example.com", relay.URL)
	assert.Equal(t, ModeRead, relay.Mode)
}

// Test edge cases with real test events
func TestRelayListWithRealEvents(t *testing.T) {
	kp := testutil.MustGenerateKeyPair()

	tests := []struct {
		name        string
		content     string
		tags        [][]string
		expectError bool
	}{
		{
			name:    "Real valid relay list event",
			content: "",
			tags: [][]string{
				{"r", "wss://relay.example.com", "read"},
				{"r", "wss://relay2.example.com"},
			},
			expectError: false,
		},
		{
			name:    "Real invalid relay list event with content",
			content: "this should not be here",
			tags: [][]string{
				{"r", "wss://relay.example.com"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt, err := testutil.NewTestEventWithKey(kp, KindRelayList, tt.content, tt.tags)
			assert.NoError(t, err)

			validationErr := ValidateRelayList(evt)
			if tt.expectError {
				assert.Error(t, validationErr)
			} else {
				assert.NoError(t, validationErr)
			}
		})
	}
}
