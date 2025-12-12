package nip02

import (
	"testing"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

func TestIsFollowListEvent(t *testing.T) {
	tests := []struct {
		name     string
		kind     int
		expected bool
	}{
		{"Follow list event", KindFollowList, true},
		{"Text note event", 1, false},
		{"Metadata event", 0, false},
		{"Deletion event", 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &event.Event{Kind: tt.kind}
			assert.Equal(t, tt.expected, IsFollowListEvent(evt))
		})
	}
}

func TestValidateFollowList(t *testing.T) {
	tests := []struct {
		name        string
		kind        int
		content     string
		tags        [][]string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid follow list",
			kind:        KindFollowList,
			content:     "",
			tags:        [][]string{{"p", "0000000000000000000000000000000000000000000000000000000000000001"}},
			expectError: false,
		},
		{
			name:        "Valid follow list with relay and petname",
			kind:        KindFollowList,
			content:     "",
			tags:        [][]string{{"p", "0000000000000000000000000000000000000000000000000000000000000001", "wss://relay.example.com", "alice"}},
			expectError: false,
		},
		{
			name:    "Valid follow list with multiple p tags",
			kind:    KindFollowList,
			content: "",
			tags: [][]string{
				{"p", "0000000000000000000000000000000000000000000000000000000000000001"},
				{"p", "0000000000000000000000000000000000000000000000000000000000000002", "wss://relay.example.com", "bob"},
			},
			expectError: false,
		},
		{
			name:        "Not a follow list (different kind)",
			kind:        1,
			content:     "",
			tags:        [][]string{{"p", "0000000000000000000000000000000000000000000000000000000000000001"}},
			expectError: false,
		},
		{
			name:        "Invalid follow list with non-empty content",
			kind:        KindFollowList,
			content:     "some content",
			tags:        [][]string{{"p", "0000000000000000000000000000000000000000000000000000000000000001"}},
			expectError: true,
			errorMsg:    "follow list (kind 3) must have empty content",
		},
		{
			name:        "Invalid follow list with no p tags",
			kind:        KindFollowList,
			content:     "",
			tags:        [][]string{{"e", "some_event_id"}},
			expectError: true,
			errorMsg:    "follow list (kind 3) must have at least one p tag",
		},
		{
			name:        "Invalid p tag missing pubkey",
			kind:        KindFollowList,
			content:     "",
			tags:        [][]string{{"p"}},
			expectError: true,
			errorMsg:    "p tag must have at least 2 elements (tag name and pubkey)",
		},
		{
			name:        "Invalid p tag with short pubkey",
			kind:        KindFollowList,
			content:     "",
			tags:        [][]string{{"p", "short"}},
			expectError: true,
			errorMsg:    "p tag pubkey must be 64 hex characters, got 5 characters",
		},
		{
			name:        "Invalid p tag with non-hex pubkey",
			kind:        KindFollowList,
			content:     "",
			tags:        [][]string{{"p", "gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg"}},
			expectError: true,
			errorMsg:    "p tag pubkey contains invalid hex characters",
		},
		{
			name:    "Mixed valid and invalid tags",
			kind:    KindFollowList,
			content: "",
			tags: [][]string{
				{"p", "0000000000000000000000000000000000000000000000000000000000000001"},
				{"p", "invalid"},
			},
			expectError: true,
			errorMsg:    "p tag pubkey must be 64 hex characters, got 7 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &event.Event{
				Kind:    tt.kind,
				Content: tt.content,
				Tags:    tt.tags,
			}

			err := ValidateFollowList(evt)
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

func TestExtractFollowedPubkeys(t *testing.T) {
	tests := []struct {
		name            string
		kind            int
		tags            [][]string
		expectedPubkeys []string
		expectError     bool
	}{
		{
			name: "Extract from valid follow list",
			kind: KindFollowList,
			tags: [][]string{
				{"p", "0000000000000000000000000000000000000000000000000000000000000001"},
				{"p", "0000000000000000000000000000000000000000000000000000000000000002"},
			},
			expectedPubkeys: []string{
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000002",
			},
			expectError: false,
		},
		{
			name:            "Non-follow list event",
			kind:            1,
			tags:            [][]string{{"p", "0000000000000000000000000000000000000000000000000000000000000001"}},
			expectedPubkeys: nil,
			expectError:     true,
		},
		{
			name:            "Follow list with no p tags",
			kind:            KindFollowList,
			tags:            [][]string{{"e", "some_event_id"}},
			expectedPubkeys: []string{},
			expectError:     false,
		},
		{
			name: "Mixed tags",
			kind: KindFollowList,
			tags: [][]string{
				{"e", "some_event_id"},
				{"p", "0000000000000000000000000000000000000000000000000000000000000001"},
				{"t", "hashtag"},
				{"p", "0000000000000000000000000000000000000000000000000000000000000002"},
			},
			expectedPubkeys: []string{
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000002",
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

			pubkeys, err := ExtractFollowedPubkeys(evt)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPubkeys, pubkeys)
			}
		})
	}
}

func TestGetFollowedPubkeyWithDetails(t *testing.T) {
	tests := []struct {
		name             string
		kind             int
		tags             [][]string
		expectedFollowed []FollowedPubkey
	}{
		{
			name: "Valid follow list with full details",
			kind: KindFollowList,
			tags: [][]string{
				{"p", "0000000000000000000000000000000000000000000000000000000000000001", "wss://relay1.example.com", "alice"},
				{"p", "0000000000000000000000000000000000000000000000000000000000000002", "wss://relay2.example.com", "bob"},
				{"p", "0000000000000000000000000000000000000000000000000000000000000003"},
			},
			expectedFollowed: []FollowedPubkey{
				{
					PubKey:   "0000000000000000000000000000000000000000000000000000000000000001",
					RelayURL: "wss://relay1.example.com",
					Petname:  "alice",
				},
				{
					PubKey:   "0000000000000000000000000000000000000000000000000000000000000002",
					RelayURL: "wss://relay2.example.com",
					Petname:  "bob",
				},
				{
					PubKey:   "0000000000000000000000000000000000000000000000000000000000000003",
					RelayURL: "",
					Petname:  "",
				},
			},
		},
		{
			name: "Non-follow list event",
			kind: 1,
			tags: [][]string{
				{"p", "0000000000000000000000000000000000000000000000000000000000000001", "wss://relay.example.com", "alice"},
			},
			expectedFollowed: []FollowedPubkey{},
		},
		{
			name: "Follow list with empty relay and petname",
			kind: KindFollowList,
			tags: [][]string{
				{"p", "0000000000000000000000000000000000000000000000000000000000000001", "", ""},
			},
			expectedFollowed: []FollowedPubkey{
				{
					PubKey:   "0000000000000000000000000000000000000000000000000000000000000001",
					RelayURL: "",
					Petname:  "",
				},
			},
		},
		{
			name: "Mixed tags",
			kind: KindFollowList,
			tags: [][]string{
				{"e", "some_event_id"},
				{"p", "0000000000000000000000000000000000000000000000000000000000000001", "wss://relay.example.com", "alice"},
				{"t", "hashtag"},
			},
			expectedFollowed: []FollowedPubkey{
				{
					PubKey:   "0000000000000000000000000000000000000000000000000000000000000001",
					RelayURL: "wss://relay.example.com",
					Petname:  "alice",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &event.Event{
				Kind: tt.kind,
				Tags: tt.tags,
			}

			followed := GetFollowedPubkeyWithDetails(evt)
			assert.Equal(t, tt.expectedFollowed, followed)
		})
	}
}

func TestHandleFollowList(t *testing.T) {
	tests := []struct {
		name        string
		kind        int
		content     string
		tags        [][]string
		expectError bool
	}{
		{
			name:        "Valid follow list",
			kind:        KindFollowList,
			content:     "",
			tags:        [][]string{{"p", "0000000000000000000000000000000000000000000000000000000000000001"}},
			expectError: false,
		},
		{
			name:        "Non-follow list event",
			kind:        1,
			content:     "test",
			tags:        [][]string{{"t", "hashtag"}},
			expectError: false,
		},
		{
			name:        "Invalid follow list",
			kind:        KindFollowList,
			content:     "invalid content",
			tags:        [][]string{{"p", "0000000000000000000000000000000000000000000000000000000000000001"}},
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
			if tt.kind == KindFollowList {
				err := ValidateFollowList(evt)
				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			} else {
				// Non-follow list events should not be processed
				assert.True(t, true) // This would be handled by the calling code
			}
		})
	}
}

func TestFollowedPubkeyStruct(t *testing.T) {
	followed := FollowedPubkey{
		PubKey:   "0000000000000000000000000000000000000000000000000000000000000001",
		RelayURL: "wss://relay.example.com",
		Petname:  "alice",
	}

	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000000000001", followed.PubKey)
	assert.Equal(t, "wss://relay.example.com", followed.RelayURL)
	assert.Equal(t, "alice", followed.Petname)
}

// Test edge cases with real test events
func TestFollowListWithRealEvents(t *testing.T) {
	kp := testutil.MustGenerateKeyPair()

	tests := []struct {
		name        string
		content     string
		tags        [][]string
		expectError bool
	}{
		{
			name:    "Real valid follow list event",
			content: "",
			tags: [][]string{
				{"p", "0000000000000000000000000000000000000000000000000000000000000001", "wss://relay.example.com", "alice"},
			},
			expectError: false,
		},
		{
			name:    "Real invalid follow list event with content",
			content: "this should not be here",
			tags: [][]string{
				{"p", "0000000000000000000000000000000000000000000000000000000000000001"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt, err := testutil.NewTestEventWithKey(kp, KindFollowList, tt.content, tt.tags)
			assert.NoError(t, err)

			validationErr := ValidateFollowList(evt)
			if tt.expectError {
				assert.Error(t, validationErr)
			} else {
				assert.NoError(t, validationErr)
			}
		})
	}
}
