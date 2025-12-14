package nip22

import (
	"testing"

	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateComment(t *testing.T) {
	tests := []struct {
		name        string
		event       *event.Event
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid top-level comment on blog post",
			event: &event.Event{
				Kind:    KindComment,
				Content: "Great blog post!",
				Tags: [][]string{
					{"A", "30023:3c9849383bdea883b0bd16fece1ed36d37e37cdde3ce43b17ea4e9192ec11289:f9347ca7", "wss://example.relay"},
					{"K", "30023"},
					{"P", "3c9849383bdea883b0bd16fece1ed36d37e37cdde3ce43b17ea4e9192ec11289", "wss://example.relay"},
					{"a", "30023:3c9849383bdea883b0bd16fece1ed36d37e37cdde3ce43b17ea4e9192ec11289:f9347ca7", "wss://example.relay"},
					{"e", "5b4fc7fed15672fefe65d2426f67197b71ccc82aa0cc8a9e94f683eb78e07651", "wss://example.relay"},
					{"k", "30023"},
					{"p", "3c9849383bdea883b0bd16fece1ed36d37e37cdde3ce43b17ea4e9192ec11289", "wss://example.relay"},
				},
			},
			expectError: false,
		},
		{
			name: "Valid comment on file",
			event: &event.Event{
				Kind:    KindComment,
				Content: "Great file!",
				Tags: [][]string{
					{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"K", "1063"},
					{"P", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
					{"k", "1063"},
					{"p", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
				},
			},
			expectError: false,
		},
		{
			name: "Valid reply to comment",
			event: &event.Event{
				Kind:    KindComment,
				Content: "This is a reply to \"Great file!\"",
				Tags: [][]string{
					{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6", "wss://example.relay", "fd913cd6fa9edb8405750cd02a8bbe16e158b8676c0e69fdc27436cc4a54cc9a"},
					{"K", "1063"},
					{"P", "fd913cd6fa9edb8405750cd02a8bbe16e158b8676c0e69fdc27436cc4a54cc9a"},
					{"e", "5c83da77af1dec6d7289834998ad7aafbd9e2191396d75ec3cc27f5a77226f36", "wss://example.relay", "93ef2ebaaf9554661f33e79949007900bbc535d239a4c801c33a4d67d3e7f546"},
					{"k", "1111"},
					{"p", "93ef2ebaaf9554661f33e79949007900bbc535d239a4c801c33a4d67d3e7f546"},
				},
			},
			expectError: false,
		},
		{
			name: "Valid comment on website URL",
			event: &event.Event{
				Kind:    KindComment,
				Content: "Nice article!",
				Tags: [][]string{
					{"I", "https://abc.com/articles/1"},
					{"K", "web"},
					{"i", "https://abc.com/articles/1"},
					{"k", "web"},
				},
			},
			expectError: false,
		},
		{
			name: "Valid podcast comment",
			event: &event.Event{
				Kind:    KindComment,
				Content: "This was a great episode!",
				Tags: [][]string{
					{"I", "podcast:item:guid:d98d189b-dc7b-45b1-8720-d4b98690f31f", "https://fountain.fm/episode/z1y9TMQRuqXl2awyrQxg"},
					{"K", "podcast:item:guid"},
					{"i", "podcast:item:guid:d98d189b-dc7b-45b1-8720-d4b98690f31f", "https://fountain.fm/episode/z1y9TMQRuqXl2awyrQxg"},
					{"k", "podcast:item:guid"},
				},
			},
			expectError: false,
		},
		{
			name: "Empty content should fail",
			event: &event.Event{
				Kind:    KindComment,
				Content: "   ",
				Tags: [][]string{
					{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
					{"K", "1063"},
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
					{"k", "1063"},
				},
			},
			expectError: true,
			errorMsg:    "must have non-empty content",
		},
		{
			name: "Missing K tag should fail",
			event: &event.Event{
				Kind:    KindComment,
				Content: "Great file!",
				Tags: [][]string{
					{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
					{"k", "1063"},
				},
			},
			expectError: true,
			errorMsg:    "must have K tag",
		},
		{
			name: "Missing k tag should fail",
			event: &event.Event{
				Kind:    KindComment,
				Content: "Great file!",
				Tags: [][]string{
					{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
					{"K", "1063"},
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
				},
			},
			expectError: true,
			errorMsg:    "must have k tag",
		},
		{
			name: "Empty K tag value should fail",
			event: &event.Event{
				Kind:    KindComment,
				Content: "Great file!",
				Tags: [][]string{
					{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
					{"K", ""},
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
					{"k", "1063"},
				},
			},
			expectError: true,
			errorMsg:    "K tag must have a kind value",
		},
		{
			name: "Non-comment event should pass",
			event: &event.Event{
				Kind:    1, // Regular text note
				Content: "Hello world",
				Tags:    [][]string{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateComment(tt.event)
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

func TestExtractCommentThreadInfo(t *testing.T) {
	tests := []struct {
		name         string
		event        *event.Event
		expectedInfo *CommentThreadInfo
		expectError  bool
	}{
		{
			name: "Top-level comment",
			event: &event.Event{
				Kind:    KindComment,
				Content: "Great blog post!",
				Tags: [][]string{
					{"A", "30023:3c9849383bdea883b0bd16fece1ed36d37e37cdde3ce43b17ea4e9192ec11289:f9347ca7"},
					{"K", "30023"},
					{"P", "3c9849383bdea883b0bd16fece1ed36d37e37cdde3ce43b17ea4e9192ec11289"},
					{"a", "30023:3c9849383bdea883b0bd16fece1ed36d37e37cdde3ce43b17ea4e9192ec11289:f9347ca7"},
					{"e", "5b4fc7fed15672fefe65d2426f67197b71ccc82aa0cc8a9e94f683eb78e07651"},
					{"k", "30023"},
					{"p", "3c9849383bdea883b0bd16fece1ed36d37e37cdde3ce43b17ea4e9192ec11289"},
				},
			},
			expectedInfo: &CommentThreadInfo{
				RootTags:      []string{"A"},
				ParentTags:    []string{"a", "e"},
				IsReply:       false,
				RootEventID:   "",
				ParentEventID: "5b4fc7fed15672fefe65d2426f67197b71ccc82aa0cc8a9e94f683eb78e07651",
			},
			expectError: false,
		},
		{
			name: "Reply to comment",
			event: &event.Event{
				Kind:    KindComment,
				Content: "This is a reply",
				Tags: [][]string{
					{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
					{"K", "1063"},
					{"P", "fd913cd6fa9edb8405750cd02a8bbe16e158b8676c0e69fdc27436cc4a54cc9a"},
					{"e", "5c83da77af1dec6d7289834998ad7aafbd9e2191396d75ec3cc27f5a77226f36"},
					{"k", "1111"},
					{"p", "93ef2ebaaf9554661f33e79949007900bbc535d239a4c801c33a4d67d3e7f546"},
				},
			},
			expectedInfo: &CommentThreadInfo{
				RootTags:      []string{"E"},
				ParentTags:    []string{"e"},
				IsReply:       true,
				RootEventID:   "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6",
				ParentEventID: "5c83da77af1dec6d7289834998ad7aafbd9e2191396d75ec3cc27f5a77226f36",
			},
			expectError: false,
		},
		{
			name: "Non-comment event should fail",
			event: &event.Event{
				Kind:    1,
				Content: "Hello",
				Tags:    [][]string{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ExtractCommentThreadInfo(tt.event)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedInfo.RootTags, info.RootTags)
				assert.Equal(t, tt.expectedInfo.ParentTags, info.ParentTags)
				assert.Equal(t, tt.expectedInfo.IsReply, info.IsReply)
				assert.Equal(t, tt.expectedInfo.RootEventID, info.RootEventID)
				assert.Equal(t, tt.expectedInfo.ParentEventID, info.ParentEventID)
			}
		})
	}
}

func TestIsCommentEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    *event.Event
		expected bool
	}{
		{
			name: "Comment event",
			event: &event.Event{
				Kind: KindComment,
			},
			expected: true,
		},
		{
			name: "Text note",
			event: &event.Event{
				Kind: 1,
			},
			expected: false,
		},
		{
			name: "Follow list",
			event: &event.Event{
				Kind: 3,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsCommentEvent(tt.event))
		})
	}
}

func TestGetRootKind(t *testing.T) {
	tests := []struct {
		name        string
		event       *event.Event
		expected    string
		expectError bool
	}{
		{
			name: "Valid K tag",
			event: &event.Event{
				Kind: KindComment,
				Tags: [][]string{
					{"K", "30023"},
					{"k", "1111"},
				},
			},
			expected:    "30023",
			expectError: false,
		},
		{
			name: "Missing K tag",
			event: &event.Event{
				Kind: KindComment,
				Tags: [][]string{
					{"k", "1111"},
				},
			},
			expectError: true,
		},
		{
			name: "Non-comment event",
			event: &event.Event{
				Kind: 1,
				Tags: [][]string{
					{"K", "30023"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetRootKind(tt.event)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetParentKind(t *testing.T) {
	tests := []struct {
		name        string
		event       *event.Event
		expected    string
		expectError bool
	}{
		{
			name: "Valid k tag",
			event: &event.Event{
				Kind: KindComment,
				Tags: [][]string{
					{"K", "30023"},
					{"k", "1111"},
				},
			},
			expected:    "1111",
			expectError: false,
		},
		{
			name: "Missing k tag",
			event: &event.Event{
				Kind: KindComment,
				Tags: [][]string{
					{"K", "30023"},
				},
			},
			expectError: true,
		},
		{
			name: "Non-comment event",
			event: &event.Event{
				Kind: 1,
				Tags: [][]string{
					{"k", "1111"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetParentKind(tt.event)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestIsTopLevelComment(t *testing.T) {
	tests := []struct {
		name        string
		event       *event.Event
		expected    bool
		expectError bool
	}{
		{
			name: "Top-level comment (same root and parent)",
			event: &event.Event{
				Kind: KindComment,
				Tags: [][]string{
					{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
					{"K", "1063"},
					{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
					{"k", "1063"},
				},
			},
			expected:    true,
			expectError: false,
		},
		{
			name: "Reply comment (different parent)",
			event: &event.Event{
				Kind: KindComment,
				Tags: [][]string{
					{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
					{"K", "1063"},
					{"e", "5c83da77af1dec6d7289834998ad7aafbd9e2191396d75ec3cc27f5a77226f36"},
					{"k", "1111"},
				},
			},
			expected:    false,
			expectError: false,
		},
		{
			name: "Non-comment event",
			event: &event.Event{
				Kind: 1,
				Tags: [][]string{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IsTopLevelComment(tt.event)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestValidateCommentForKind(t *testing.T) {
	tests := []struct {
		name        string
		event       *event.Event
		rootKind    int
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid comment for kind 30023",
			event: &event.Event{
				Kind: KindComment,
				Tags: [][]string{
					{"K", "30023"},
					{"k", "1111"},
				},
			},
			rootKind:    30023,
			expectError: false,
		},
		{
			name: "Invalid comment for kind 1 (should use NIP-10)",
			event: &event.Event{
				Kind: KindComment,
				Tags: [][]string{
					{"K", "1"},
					{"k", "1111"},
				},
			},
			rootKind:    1,
			expectError: true,
			errorMsg:    "must not be used to reply to kind 1 notes",
		},
		{
			name: "Non-comment event",
			event: &event.Event{
				Kind: 1,
				Tags: [][]string{},
			},
			rootKind:    1,
			expectError: true,
			errorMsg:    "event is not a comment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommentForKind(tt.event, tt.rootKind)
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

func TestParseKind(t *testing.T) {
	tests := []struct {
		name        string
		kindStr     string
		expected    int
		expectError bool
	}{
		{
			name:     "Regular number",
			kindStr:  "30023",
			expected: 30023,
		},
		{
			name:     "Web kind",
			kindStr:  "web",
			expected: -1,
		},
		{
			name:     "Web kind uppercase",
			kindStr:  "WEB",
			expected: -1,
		},
		{
			name:        "Invalid number",
			kindStr:     "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseKind(tt.kindStr)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
