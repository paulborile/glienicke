package integration

import (
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/paul/glienicke/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNIP42Authentication(t *testing.T) {
	// Test AUTH command handling
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	// Connect client
	client, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer client.Close()

	// Create a valid AUTH event
	sk := nostr.GeneratePrivateKey()
	pk, err := nostr.GetPublicKey(sk)
	require.NoError(t, err)

	authEvent := &nostr.Event{
		Kind:      22242, // AUTH kind
		Content:   "test auth",
		CreatedAt: nostr.Now(),
		Tags:      nostr.Tags{},
	}

	authEvent.PubKey = pk
	err = authEvent.Sign(sk)
	require.NoError(t, err)

	// Convert to local event format
	localAuthEvent := convertNostrEventToLocalEvent(authEvent)

	// Send AUTH command
	err = client.SendEvent(localAuthEvent)
	require.NoError(t, err)

	// Should receive OK response
	accepted, msg, err := client.ExpectOK(localAuthEvent.ID, 2*time.Second)
	require.NoError(t, err)
	assert.True(t, accepted)
	assert.NotEmpty(t, msg)
}
