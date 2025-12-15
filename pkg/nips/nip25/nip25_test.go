package nip25

import (
	"testing"

	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateReaction(t *testing.T) {
	tests := []struct {
		name        string
		event       *event.Event
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid like reaction",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "+",
				Tags: [][]string{
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"p", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb", "wss://example.relay"},
					{"k", "1"},
				},
			},
			expectError: false,
		},
		{
			name: "Valid dislike reaction",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "-",
				Tags: [][]string{
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"p", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb", "wss://example.relay"},
					{"k", "1"},
				},
			},
			expectError: false,
		},
		{
			name: "Valid empty content (interpreted as like)",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "",
				Tags: [][]string{
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"p", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb", "wss://example.relay"},
					{"k", "1"},
				},
			},
			expectError: false,
		},
		{
			name: "Valid emoji reaction",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "❤️",
				Tags: [][]string{
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"p", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb", "wss://example.relay"},
					{"k", "1"},
				},
			},
			expectError: false,
		},
		{
			name: "Valid custom emoji shortcode",
			event: &event.Event{
				Kind:    KindReaction,
				Content: ":shortcode:soapbox:",
				Tags: [][]string{
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"p", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb", "wss://example.relay"},
					{"k", "1"},
					{"emoji", "soapbox", "https://gleasonator.com/emoji/Gleasonator/soapbox.png"},
				},
			},
			expectError: false,
		},
		{
			name: "Missing e tag should fail",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "+",
				Tags: [][]string{
					{"p", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb", "wss://example.relay"},
					{"k", "1"},
				},
			},
			expectError: true,
			errorMsg:    "must have at least one e tag",
		},
		{
			name: "Empty e tag value should fail",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "+",
				Tags: [][]string{
					{"e", "", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"p", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb", "wss://example.relay"},
					{"k", "1"},
				},
			},
			expectError: true,
			errorMsg:    "e tag must have event ID",
		},
		{
			name: "Invalid e tag ID (too short) should fail",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "+",
				Tags: [][]string{
					{"e", "short", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"p", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb", "wss://example.relay"},
					{"k", "1"},
				},
			},
			expectError: true,
			errorMsg:    "e tag event ID must be 64 hex characters",
		},
		{
			name: "Invalid e tag ID (invalid hex) should fail",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "+",
				Tags: [][]string{
					{"e", "gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"p", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb", "wss://example.relay"},
					{"k", "1"},
				},
			},
			expectError: true,
			errorMsg:    "e tag event ID contains invalid hex characters",
		},
		{
			name: "Invalid p tag pubkey (too short) should fail",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "+",
				Tags: [][]string{
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"p", "short", "wss://example.relay"},
					{"k", "1"},
				},
			},
			expectError: true,
			errorMsg:    "p tag pubkey must be 64 hex characters",
		},
		{
			name: "Invalid k tag value (not number) should fail",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "+",
				Tags: [][]string{
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"p", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb", "wss://example.relay"},
					{"k", "notanumber"},
				},
			},
			expectError: true,
			errorMsg:    "k tag must be a stringified number",
		},
		{
			name: "Invalid emoji shortcode format should fail",
			event: &event.Event{
				Kind:    KindReaction,
				Content: ":invalidformat",
				Tags: [][]string{
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"p", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb", "wss://example.relay"},
					{"k", "1"},
				},
			},
			expectError: true,
			errorMsg:    "invalid emoji shortcode format",
		},
		{
			name: "Non-reaction event should pass",
			event: &event.Event{
				Kind:    1, // Text note
				Content: "Hello world",
				Tags:    [][]string{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReaction(tt.event)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIsReactionEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    *event.Event
		expected bool
	}{
		{
			name: "Reaction event",
			event: &event.Event{
				Kind: KindReaction,
			},
			expected: true,
		},
		{
			name: "Text note event",
			event: &event.Event{
				Kind: 1,
			},
			expected: false,
		},
		{
			name: "Follow list event",
			event: &event.Event{
				Kind: 3,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsReactionEvent(tt.event))
		})
	}
}

func TestGetReactedEventIDs(t *testing.T) {
	tests := []struct {
		name     string
		event    *event.Event
		expected []string
	}{
		{
			name: "Single e tag",
			event: &event.Event{
				Kind: KindReaction,
				Tags: [][]string{
					{"e", "event1"},
					{"p", "author1"},
				},
			},
			expected: []string{"event1"},
		},
		{
			name: "Multiple e tags",
			event: &event.Event{
				Kind: KindReaction,
				Tags: [][]string{
					{"e", "event1"},
					{"e", "event2"},
					{"p", "author1"},
				},
			},
			expected: []string{"event1", "event2"},
		},
		{
			name: "No e tags",
			event: &event.Event{
				Kind: KindReaction,
				Tags: [][]string{
					{"p", "author1"},
				},
			},
			expected: []string{},
		},
		{
			name: "Non-reaction event",
			event: &event.Event{
				Kind: 1,
				Tags: [][]string{
					{"e", "event1"},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetReactedEventIDs(tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetReactedEventAuthors(t *testing.T) {
	tests := []struct {
		name     string
		event    *event.Event
		expected []string
	}{
		{
			name: "Single p tag",
			event: &event.Event{
				Kind: KindReaction,
				Tags: [][]string{
					{"e", "event1"},
					{"p", "author1"},
				},
			},
			expected: []string{"author1"},
		},
		{
			name: "Multiple p tags",
			event: &event.Event{
				Kind: KindReaction,
				Tags: [][]string{
					{"e", "event1"},
					{"p", "author1"},
					{"p", "author2"},
				},
			},
			expected: []string{"author1", "author2"},
		},
		{
			name: "No p tags",
			event: &event.Event{
				Kind: KindReaction,
				Tags: [][]string{
					{"e", "event1"},
				},
			},
			expected: []string{},
		},
		{
			name: "Non-reaction event",
			event: &event.Event{
				Kind: 1,
				Tags: [][]string{
					{"p", "author1"},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetReactedEventAuthors(tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetReactionType(t *testing.T) {
	tests := []struct {
		name     string
		event    *event.Event
		expected string
	}{
		{
			name: "Like reaction",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "+",
			},
			expected: "+",
		},
		{
			name: "Dislike reaction",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "-",
			},
			expected: "-",
		},
		{
			name: "Empty content (like)",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "",
			},
			expected: "+",
		},
		{
			name: "Emoji reaction",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "❤️",
			},
			expected: "❤️",
		},
		{
			name: "Custom emoji shortcode",
			event: &event.Event{
				Kind:    KindReaction,
				Content: ":shortcode:soapbox:",
			},
			expected: ":shortcode:soapbox:",
		},
		{
			name: "Non-reaction event",
			event: &event.Event{
				Kind:    1,
				Content: "Hello",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetReactionType(tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsLikeReaction(t *testing.T) {
	tests := []struct {
		name     string
		event    *event.Event
		expected bool
	}{
		{
			name: "Plus sign",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "+",
			},
			expected: true,
		},
		{
			name: "Empty content",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "",
			},
			expected: true,
		},
		{
			name: "Minus sign",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "-",
			},
			expected: false,
		},
		{
			name: "Emoji",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "❤️",
			},
			expected: false,
		},
		{
			name: "Non-reaction event",
			event: &event.Event{
				Kind:    1,
				Content: "+",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsLikeReaction(tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsDislikeReaction(t *testing.T) {
	tests := []struct {
		name     string
		event    *event.Event
		expected bool
	}{
		{
			name: "Minus sign",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "-",
			},
			expected: true,
		},
		{
			name: "Plus sign",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "+",
			},
			expected: false,
		},
		{
			name: "Empty content",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "",
			},
			expected: false,
		},
		{
			name: "Emoji",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "❤️",
			},
			expected: false,
		},
		{
			name: "Non-reaction event",
			event: &event.Event{
				Kind:    1,
				Content: "-",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDislikeReaction(tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsEmojiReaction(t *testing.T) {
	tests := []struct {
		name     string
		event    *event.Event
		expected bool
	}{
		{
			name: "Emoji reaction",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "❤️",
			},
			expected: true,
		},
		{
			name: "Custom emoji shortcode",
			event: &event.Event{
				Kind:    KindReaction,
				Content: ":shortcode:soapbox:",
			},
			expected: true,
		},
		{
			name: "Plus sign",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "+",
			},
			expected: false,
		},
		{
			name: "Minus sign",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "-",
			},
			expected: false,
		},
		{
			name: "Empty content",
			event: &event.Event{
				Kind:    KindReaction,
				Content: "",
			},
			expected: false,
		},
		{
			name: "Non-reaction event",
			event: &event.Event{
				Kind:    1,
				Content: "❤️",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmojiReaction(tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetReactedEventKind(t *testing.T) {
	tests := []struct {
		name        string
		event       *event.Event
		expected    int
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid k tag",
			event: &event.Event{
				Kind: KindReaction,
				Tags: [][]string{
					{"e", "event1"},
					{"p", "author1"},
					{"k", "1"},
				},
			},
			expected:    1,
			expectError: false,
		},
		{
			name: "Missing k tag",
			event: &event.Event{
				Kind: KindReaction,
				Tags: [][]string{
					{"e", "event1"},
					{"p", "author1"},
				},
			},
			expected:    0,
			expectError: true,
			errorMsg:    "k tag not found",
		},
		{
			name: "Invalid k tag value",
			event: &event.Event{
				Kind: KindReaction,
				Tags: [][]string{
					{"e", "event1"},
					{"p", "author1"},
					{"k", "notanumber"},
				},
			},
			expected:    0,
			expectError: true,
			errorMsg:    "invalid k tag value",
		},
		{
			name: "Non-reaction event",
			event: &event.Event{
				Kind: 1,
				Tags: [][]string{
					{"k", "1"},
				},
			},
			expected:    0,
			expectError: true,
			errorMsg:    "event is not a reaction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetReactedEventKind(tt.event)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
