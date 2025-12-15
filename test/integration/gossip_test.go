package integration

import (
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGossipRelay_OutboxPosting(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer client.Close()

	// Create authenticated user
	user, userKP := testutil.MustNewTestEvent(1, "Test user", nil)

	// Send user event first to establish
	err = client.SendEvent(user)
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Create outbox test event (kind 112)
	outboxEvent, _ := testutil.NewTestEventWithKey(userKP, 112, "This is an automated test of suitability of {relay_url} for inbox/outbox/dm usage. Please disregard.", nil)
	outboxEvent.CreatedAt = time.Now().Unix()
	userKP.SignEvent(outboxEvent)

	// Send outbox event
	err = client.SendEvent(outboxEvent)
	require.NoError(t, err)

	// Expect OK response
	accepted, msg, err := client.ExpectOK(outboxEvent.ID, 2*time.Second)
	require.NoError(t, err)
	require.True(t, accepted, "Outbox event should be accepted")
	require.Empty(t, msg, "Message should be empty for accepted outbox event")

	t.Logf("✅ Outbox posting test passed - Event ID: %s", outboxEvent.ID)
}

func TestGossipRelay_AnonymousOutboxReading(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	// Create authenticated client to post event
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

	// Create anonymous client to read
	anonClient, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer anonClient.Close()

	// Query for user's outbox event
	filter := &event.Filter{
		Kinds:   []int{112},
		Authors: []string{user.PubKey},
		Since:   &outboxEvent.CreatedAt,
	}

	err = anonClient.SendReq("outbox-test", filter)
	require.NoError(t, err)

	// Should receive the outbox event
	events, err := anonClient.CollectEvents("outbox-test", 2*time.Second)
	require.NoError(t, err)
	assert.Len(t, events, 1, "Should receive exactly one outbox event")

	if len(events) > 0 {
		assert.Equal(t, outboxEvent.ID, events[0].ID, "Should receive the correct outbox event")
		t.Logf("✅ Anonymous outbox reading test passed - Found event: %s", events[0].ID)
	}
}
