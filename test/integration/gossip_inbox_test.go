package integration

import (
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGossipRelay_AnonymousInboxPosting(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	// Create authenticated client to post outbox event
	authClient, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer authClient.Close()

	user, userKP := testutil.MustNewTestEvent(1, "Test user", nil)
	err = authClient.SendEvent(user)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Create and post outbox event
	outboxEvent, _ := testutil.NewTestEventWithKey(userKP, 112, "This is an automated test of suitability of {relay_url} for inbox/outbox/dm usage. Please disregard.", nil)
	outboxEvent.CreatedAt = time.Now().Unix()
	userKP.SignEvent(outboxEvent)

	err = authClient.SendEvent(outboxEvent)
	require.NoError(t, err)

	accepted, _, err := authClient.ExpectOK(outboxEvent.ID, 2*time.Second)
	require.NoError(t, err)
	require.True(t, accepted, "Outbox event should be accepted")

	time.Sleep(100 * time.Millisecond)

	// Create anonymous client to post reaction
	anonClient, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer anonClient.Close()

	// Generate "stranger" key for anonymous posting
	strangerKP := testutil.MustGenerateKeyPair()

	// Create reaction event (anonymous inbox posting)
	reactionTags := [][]string{
		{"e", outboxEvent.ID, "wss://example.relay", user.PubKey},
		{"p", user.PubKey, "wss://example.relay"},
	}
	reactionEvent := &event.Event{
		Kind:      7, // Reaction
		PubKey:    strangerKP.PubKeyHex,
		Content:   "", // Empty reaction
		Tags:      reactionTags,
		CreatedAt: time.Now().Unix(),
	}

	// Sign with stranger key
	strangerKP.SignEvent(reactionEvent)

	// Send reaction event anonymously
	err = anonClient.SendEvent(reactionEvent)
	require.NoError(t, err)

	// Expect OK response
	accepted, msg, err := anonClient.ExpectOK(reactionEvent.ID, 2*time.Second)
	require.NoError(t, err)
	require.True(t, accepted, "Anonymous reaction should be accepted")
	require.Empty(t, msg, "Message should be empty for accepted reaction")

	t.Logf("✅ Anonymous inbox posting test passed - Reaction ID: %s", reactionEvent.ID)
}

func TestGossipRelay_AnonymousInboxReading(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	// Setup: Create user and outbox event
	authClient, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer authClient.Close()

	user, userKP := testutil.MustNewTestEvent(1, "Test user", nil)
	err = authClient.SendEvent(user)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	outboxEvent, _ := testutil.NewTestEventWithKey(userKP, 112, "This is an automated test of suitability of {relay_url} for inbox/outbox/dm usage. Please disregard.", nil)
	outboxEvent.CreatedAt = time.Now().Unix()
	userKP.SignEvent(outboxEvent)

	err = authClient.SendEvent(outboxEvent)
	require.NoError(t, err)

	accepted, _, err := authClient.ExpectOK(outboxEvent.ID, 2*time.Second)
	require.NoError(t, err)
	require.True(t, accepted, "Outbox event should be accepted")

	time.Sleep(100 * time.Millisecond)

	// Post anonymous reaction
	anonClient, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer anonClient.Close()

	strangerKP := testutil.MustGenerateKeyPair()

	reactionTags := [][]string{
		{"e", outboxEvent.ID, "wss://example.relay", user.PubKey},
		{"p", user.PubKey, "wss://example.relay"},
	}
	reactionEvent := &event.Event{
		Kind:      7, // Reaction
		PubKey:    strangerKP.PubKeyHex,
		Content:   "", // Empty reaction
		Tags:      reactionTags,
		CreatedAt: time.Now().Unix(),
	}

	strangerKP.SignEvent(reactionEvent)

	err = anonClient.SendEvent(reactionEvent)
	require.NoError(t, err)

	accepted, _, err = anonClient.ExpectOK(reactionEvent.ID, 2*time.Second)
	require.NoError(t, err)
	require.True(t, accepted, "Anonymous reaction should be accepted")

	time.Sleep(100 * time.Millisecond)

	// Create another anonymous client to read inbox
	readerClient, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer readerClient.Close()

	// Query for reaction events
	filter := &event.Filter{
		Kinds:   []int{7},
		Authors: []string{strangerKP.PubKeyHex},
		Since:   &reactionEvent.CreatedAt,
	}

	err = readerClient.SendReq("inbox-test", filter)
	require.NoError(t, err)

	// Should receive reaction event
	events, err := readerClient.CollectEvents("inbox-test", 2*time.Second)
	require.NoError(t, err)
	assert.Len(t, events, 1, "Should receive exactly one reaction event")

	if len(events) > 0 {
		assert.Equal(t, reactionEvent.ID, events[0].ID, "Should receive correct reaction event")
		t.Logf("✅ Anonymous inbox reading test passed - Found reaction: %s", events[0].ID)
	}
}

func TestGossipRelay_AuthenticatedInboxReading(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	// Setup: Create user and outbox event
	authClient, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer authClient.Close()

	user, userKP := testutil.MustNewTestEvent(1, "Test user", nil)
	err = authClient.SendEvent(user)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	outboxEvent, _ := testutil.NewTestEventWithKey(userKP, 112, "This is an automated test of suitability of {relay_url} for inbox/outbox/dm usage. Please disregard.", nil)
	outboxEvent.CreatedAt = time.Now().Unix()
	userKP.SignEvent(outboxEvent)

	err = authClient.SendEvent(outboxEvent)
	require.NoError(t, err)

	accepted, _, err := authClient.ExpectOK(outboxEvent.ID, 2*time.Second)
	require.NoError(t, err)
	require.True(t, accepted, "Outbox event should be accepted")

	time.Sleep(100 * time.Millisecond)

	// Post anonymous reaction
	anonClient, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer anonClient.Close()

	strangerKP := testutil.MustGenerateKeyPair()

	reactionTags := [][]string{
		{"e", outboxEvent.ID, "wss://example.relay", user.PubKey},
		{"p", user.PubKey, "wss://example.relay"},
	}
	reactionEvent := &event.Event{
		Kind:      7, // Reaction
		PubKey:    strangerKP.PubKeyHex,
		Content:   "", // Empty reaction
		Tags:      reactionTags,
		CreatedAt: time.Now().Unix(),
	}

	strangerKP.SignEvent(reactionEvent)

	err = anonClient.SendEvent(reactionEvent)
	require.NoError(t, err)

	accepted, _, err = anonClient.ExpectOK(reactionEvent.ID, 2*time.Second)
	require.NoError(t, err)
	require.True(t, accepted, "Anonymous reaction should be accepted")

	time.Sleep(100 * time.Millisecond)

	// Use authenticated client to read inbox
	filter := &event.Filter{
		Kinds:   []int{7},
		Authors: []string{strangerKP.PubKeyHex},
		Since:   &reactionEvent.CreatedAt,
	}

	err = authClient.SendReq("inbox-auth-test", filter)
	require.NoError(t, err)

	// Should receive reaction event
	events, err := authClient.CollectEvents("inbox-auth-test", 2*time.Second)
	require.NoError(t, err)
	assert.Len(t, events, 1, "Should receive exactly one reaction event")

	if len(events) > 0 {
		assert.Equal(t, reactionEvent.ID, events[0].ID, "Should receive correct reaction event")
		t.Logf("✅ Authenticated inbox reading test passed - Found reaction: %s", events[0].ID)
	}
}
