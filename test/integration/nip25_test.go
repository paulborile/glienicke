package integration

import (
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNIP25_BasicReactionValidation(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer client.Close()

	// Create test users
	author, _ := testutil.MustNewTestEvent(1, "Author's test note", nil)
	reacter, reacterKP := testutil.MustNewTestEvent(1, "Reacter's test note", nil)

	// Send initial events to establish users
	err = client.SendEvent(author)
	require.NoError(t, err)
	err = client.SendEvent(reacter)
	require.NoError(t, err)

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	// Test valid like reaction
	reactionTags := [][]string{
		{"e", author.ID, "wss://example.relay", author.PubKey},
		{"p", author.PubKey, "wss://example.relay"},
		{"k", "1"},
	}
	reactionEvent, _ := testutil.NewTestEventWithKey(reacterKP, 7, "+", reactionTags)
	reactionEvent.CreatedAt = time.Now().Unix()
	reacterKP.SignEvent(reactionEvent)

	// Send reaction event
	err = client.SendEvent(reactionEvent)
	require.NoError(t, err)

	// Expect OK response
	accepted, msg, err := client.ExpectOK(reactionEvent.ID, 2*time.Second)
	require.NoError(t, err)
	require.True(t, accepted, "Like reaction should be accepted")
	require.Empty(t, msg, "Message should be empty for accepted reaction")

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Query for reactions
	filter := &event.Filter{
		Kinds: []int{7},
		Limit: func() *int { l := 10; return &l }(),
	}
	err = client.SendReq("reaction-sub", filter)
	require.NoError(t, err)

	// Should receive the reaction event
	events, err := client.CollectEvents("reaction-sub", 2*time.Second)
	require.NoError(t, err)
	assert.Len(t, events, 1, "Should receive exactly one reaction event")

	receivedReaction := events[0]
	assert.Equal(t, 7, receivedReaction.Kind, "Should be kind 7")
	assert.Equal(t, "+", receivedReaction.Content, "Should have + content")
	assert.Equal(t, reacter.PubKey, receivedReaction.PubKey, "Should have correct author")
}

func TestNIP25_RejectionValidation(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer client.Close()

	// Create test user
	author, authorKP := testutil.MustNewTestEvent(1, "Author's test note", nil)

	// Send initial event to establish user
	err = client.SendEvent(author)
	require.NoError(t, err)

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	// Test invalid reaction (missing e tag)
	reactionTags := [][]string{
		{"p", author.PubKey, "wss://example.relay"},
		{"k", "1"},
	}
	reactionEvent, _ := testutil.NewTestEventWithKey(authorKP, 7, "+", reactionTags)
	reactionEvent.CreatedAt = time.Now().Unix()
	authorKP.SignEvent(reactionEvent)

	// Send reaction event
	err = client.SendEvent(reactionEvent)
	require.NoError(t, err)

	// Expect rejection
	accepted, msg, err := client.ExpectOK(reactionEvent.ID, 2*time.Second)
	require.NoError(t, err)
	assert.False(t, accepted, "Reaction missing e tag should be rejected")
	assert.NotEmpty(t, msg, "Should provide rejection message")
	assert.Contains(t, msg, "must have at least one e tag")
}

func TestNIP25_ReactionBroadcasting(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer client.Close()

	// Create test users and events
	author, _ := testutil.MustNewTestEvent(1, "Author's test note", nil)
	reacter, reacterKP := testutil.MustNewTestEvent(1, "Reacter's test note", nil)

	// Send initial events
	err = client.SendEvent(author)
	require.NoError(t, err)
	err = client.SendEvent(reacter)
	require.NoError(t, err)

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	// Subscribe to reactions first
	filter := &event.Filter{
		Kinds: []int{7},
	}
	err = client.SendReq("reaction-sub", filter)
	require.NoError(t, err)

	// Wait for EOSE
	time.Sleep(100 * time.Millisecond)

	// Create and send reaction
	reactionTags := [][]string{
		{"e", author.ID, "wss://example.relay", author.PubKey},
		{"p", author.PubKey, "wss://example.relay"},
		{"k", "1"},
	}
	reactionEvent, _ := testutil.NewTestEventWithKey(reacterKP, 7, "❤️", reactionTags)
	reactionEvent.CreatedAt = time.Now().Unix()
	reacterKP.SignEvent(reactionEvent)

	// Send reaction event
	err = client.SendEvent(reactionEvent)
	require.NoError(t, err)

	// Expect OK response
	accepted, msg, err := client.ExpectOK(reactionEvent.ID, 2*time.Second)
	require.NoError(t, err)
	require.True(t, accepted, "Emoji reaction should be accepted")
	require.Empty(t, msg, "Message should be empty for accepted reaction")

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	// Create a new subscription to query for stored reactions
	err = client.SendReq("query-reaction-sub", &event.Filter{
		Kinds: []int{7},
		Limit: func() *int { l := 10; return &l }(),
	})
	require.NoError(t, err)

	// Should get the reaction event from storage
	events, err := client.CollectEvents("query-reaction-sub", 2*time.Second)
	require.NoError(t, err)
	assert.Len(t, events, 1, "Should receive exactly one reaction event")

	receivedReaction := events[0]
	assert.Equal(t, 7, receivedReaction.Kind, "Should be kind 7")
	assert.Equal(t, "❤️", receivedReaction.Content, "Should have emoji content")
	assert.Equal(t, reacter.PubKey, receivedReaction.PubKey, "Should have correct author")
}
