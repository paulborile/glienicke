package integration

import (
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNIP22_CommentThreads(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer client.Close()

	// Create test users
	author, authorKP := testutil.MustNewTestEvent(KindTextNote, "Author's blog post", nil)
	commenter, commenterKP := testutil.MustNewTestEvent(KindTextNote, "Commenter's note", nil)

	// Send initial events to establish users
	err = client.SendEvent(author)
	require.NoError(t, err)
	err = client.SendEvent(commenter)
	require.NoError(t, err)

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	// Test 1: Create a top-level comment on a blog post (kind 30023)
	blogPostTags := [][]string{
		{"A", "30023:3c9849383bdea883b0bd16fece1ed36d37e37cdde3ce43b17ea4e9192ec11289:f9347ca7", "wss://example.relay"},
		{"K", "30023"},
		{"P", author.PubKey, "wss://example.relay"},
		{"a", "30023:3c9849383bdea883b0bd16fece1ed36d37e37cdde3ce43b17ea4e9192ec11289:f9347ca7", "wss://example.relay"},
		{"e", "5b4fc7fed15672fefe65d2426f67197b71ccc82aa0cc8a9e94f683eb78e07651", "wss://example.relay"},
		{"k", "30023"},
		{"p", author.PubKey, "wss://example.relay"},
	}

	commentEvent, _ := testutil.NewTestEventWithKey(commenterKP, 1111, "Great blog post!", blogPostTags)
	commentEvent.CreatedAt = time.Now().Unix()
	commenterKP.SignEvent(commentEvent)

	// Send comment event
	err = client.SendEvent(commentEvent)
	require.NoError(t, err)

	// Expect OK response
	accepted, msg, err := client.ExpectOK(commentEvent.ID, 2*time.Second)
	require.NoError(t, err)
	assert.True(t, accepted, "Comment event should be accepted")
	assert.Empty(t, msg, "Message should be empty")

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Query for the comment
	filter := &event.Filter{
		Kinds: []int{1111},
		Limit: func() *int { l := 1; return &l }(),
	}
	err = client.SendReq("comment-sub", filter)
	require.NoError(t, err)

	events, err := client.CollectEvents("comment-sub", 2*time.Second)
	require.NoError(t, err)
	assert.Len(t, events, 1, "Should receive exactly one comment event")

	receivedComment := events[0]
	assert.Equal(t, 1111, receivedComment.Kind)
	assert.Equal(t, "Great blog post!", receivedComment.Content)
	assert.Equal(t, commenter.PubKey, receivedComment.PubKey)

	// Test 2: Create a reply to the comment
	replyTags := [][]string{
		{"E", "5b4fc7fed15672fefe65d2426f67197b71ccc82aa0cc8a9e94f683eb78e07651", "wss://example.relay", author.PubKey},
		{"K", "30023"},
		{"P", author.PubKey, "wss://example.relay"},
		{"e", commentEvent.ID, "wss://example.relay", commenter.PubKey},
		{"k", "1111"},
		{"p", commenter.PubKey, "wss://example.relay"},
	}

	replyEvent, _ := testutil.NewTestEventWithKey(authorKP, 1111, "Thanks for your comment!", replyTags)
	replyEvent.CreatedAt = time.Now().Unix() + 1
	authorKP.SignEvent(replyEvent)

	// Send reply event
	err = client.SendEvent(replyEvent)
	require.NoError(t, err)

	// Expect OK response
	accepted, msg, err = client.ExpectOK(replyEvent.ID, 2*time.Second)
	require.NoError(t, err)
	assert.True(t, accepted, "Reply event should be accepted")
	assert.Empty(t, msg, "Message should be empty")

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Query for all comments
	err = client.SendReq("all-comments-sub", &event.Filter{
		Kinds: []int{1111},
		Limit: func() *int { l := 10; return &l }(),
	})
	require.NoError(t, err)

	allEvents, err := client.CollectEvents("all-comments-sub", 2*time.Second)
	require.NoError(t, err)
	assert.Len(t, allEvents, 2, "Should receive exactly two comment events")

	// Test 3: Create a comment on a website URL
	urlCommentTags := [][]string{
		{"I", "https://abc.com/articles/1"},
		{"K", "web"},
		{"i", "https://abc.com/articles/1"},
		{"k", "web"},
	}

	urlCommentEvent, _ := testutil.NewTestEventWithKey(commenterKP, 1111, "Nice article!", urlCommentTags)
	urlCommentEvent.CreatedAt = time.Now().Unix() + 2
	commenterKP.SignEvent(urlCommentEvent)

	// Send URL comment event
	err = client.SendEvent(urlCommentEvent)
	require.NoError(t, err)

	// Expect OK response
	accepted, msg, err = client.ExpectOK(urlCommentEvent.ID, 2*time.Second)
	require.NoError(t, err)
	assert.True(t, accepted, "URL comment event should be accepted")
	assert.Empty(t, msg, "Message should be empty")
}

func TestNIP22_CommentValidation(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer client.Close()

	// Test invalid comment events
	testCases := []struct {
		name         string
		tags         [][]string
		content      string
		shouldAccept bool
		errorMsg     string
	}{
		{
			name: "Valid comment on file",
			tags: [][]string{
				{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
				{"K", "1063"},
				{"P", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
				{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6", "wss://example.relay", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
				{"k", "1063"},
				{"p", "3721e07b079525289877c366ccab47112bdff3d1b44758ca333feb2dbbbbe5bb"},
			},
			content:      "Great file!",
			shouldAccept: true,
		},
		{
			name: "Invalid comment with empty content",
			tags: [][]string{
				{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
				{"K", "1063"},
				{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
				{"k", "1063"},
			},
			content:      "   ",
			shouldAccept: false,
			errorMsg:     "must have non-empty content",
		},
		{
			name: "Invalid comment missing K tag",
			tags: [][]string{
				{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
				{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
				{"k", "1063"},
			},
			content:      "Some comment",
			shouldAccept: false,
			errorMsg:     "must have K tag",
		},
		{
			name: "Invalid comment missing k tag",
			tags: [][]string{
				{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
				{"K", "1063"},
				{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
			},
			content:      "Some comment",
			shouldAccept: false,
			errorMsg:     "must have k tag",
		},
		{
			name: "Invalid comment with empty K tag value",
			tags: [][]string{
				{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
				{"K", ""},
				{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
				{"k", "1063"},
			},
			content:      "Some comment",
			shouldAccept: false,
			errorMsg:     "K tag must have a kind value",
		},
		{
			name: "Valid comment on website URL",
			tags: [][]string{
				{"I", "https://abc.com/articles/1"},
				{"K", "web"},
				{"i", "https://abc.com/articles/1"},
				{"k", "web"},
			},
			content:      "Nice article!",
			shouldAccept: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, kp := testutil.MustNewTestEvent(KindTextNote, "Test note", nil)

			commentEvent, _ := testutil.NewTestEventWithKey(kp, 1111, tc.content, tc.tags)
			commentEvent.CreatedAt = time.Now().Unix()
			kp.SignEvent(commentEvent)

			// Send comment event
			err = client.SendEvent(commentEvent)
			require.NoError(t, err)

			// Expect OK response
			accepted, msg, err := client.ExpectOK(commentEvent.ID, 2*time.Second)
			require.NoError(t, err)

			if tc.shouldAccept {
				assert.True(t, accepted, "Event should be accepted: %s", tc.name)
				assert.Empty(t, msg, "Message should be empty for accepted event: %s", tc.name)
			} else {
				assert.False(t, accepted, "Event should be rejected: %s", tc.name)
				assert.NotEmpty(t, msg, "Message should explain rejection: %s", tc.name)
				if tc.errorMsg != "" {
					assert.Contains(t, msg, tc.errorMsg, "Error message should contain expected text: %s", tc.name)
				}
			}
		})
	}
}

func TestNIP22_CommentOnKind1ShouldFail(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer client.Close()

	// Test that comments on kind 1 (text notes) should be rejected
	// since NIP-10 should be used instead
	_, kp := testutil.MustNewTestEvent(KindTextNote, "Test note", nil)

	commentOnKind1Tags := [][]string{
		{"E", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
		{"K", "1"}, // Kind 1 - text note
		{"e", "768ac8720cdeb59227cf95e98b66560ef03d8bc9a90d721779e76e68fb42f5e6"},
		{"k", "1"},
	}

	commentEvent, _ := testutil.NewTestEventWithKey(kp, 1111, "This should fail", commentOnKind1Tags)
	commentEvent.CreatedAt = time.Now().Unix()
	kp.SignEvent(commentEvent)

	// Send comment event
	err = client.SendEvent(commentEvent)
	require.NoError(t, err)

	// Expect rejection
	accepted, msg, err := client.ExpectOK(commentEvent.ID, 2*time.Second)
	require.NoError(t, err)
	assert.False(t, accepted, "Comment on kind 1 should be rejected")
	assert.NotEmpty(t, msg, "Should provide rejection message")
	assert.Contains(t, msg, "must not be used to reply to kind 1 notes", "Should mention NIP-10")
}
