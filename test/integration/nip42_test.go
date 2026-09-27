package integration

import (
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/paulborile/glienicke/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNIP42Authentication(t *testing.T) {
	// Test AUTH command handling
	url, _, cleanup, _ := setupRelay(t)
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

// TestNIP42AuthMessageType verifies that a client sending the auth event
// wrapped in the spec's own ["AUTH", <event>] message (rather than
// ["EVENT", <event>]) is authenticated. Regression test for a bug where the
// relay only handled the EVENT-wrapped form and rejected literal AUTH
// messages with "unknown message type: AUTH".
func TestNIP42AuthMessageType(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer client.Close()

	sk := nostr.GeneratePrivateKey()
	pk, err := nostr.GetPublicKey(sk)
	require.NoError(t, err)

	authEvent := &nostr.Event{
		Kind:      22242,
		Content:   "test auth via AUTH message",
		CreatedAt: nostr.Now(),
		Tags:      nostr.Tags{},
	}
	authEvent.PubKey = pk
	err = authEvent.Sign(sk)
	require.NoError(t, err)

	localAuthEvent := convertNostrEventToLocalEvent(authEvent)

	err = client.SendAuth(localAuthEvent)
	require.NoError(t, err)

	accepted, msg, err := client.ExpectOK(localAuthEvent.ID, 2*time.Second)
	require.NoError(t, err)
	assert.True(t, accepted)
	assert.NotEmpty(t, msg)
}

// TestNIP42AuthMessageTypeWithRequireAuth reproduces the production bug: with
// NIP-42 required, a client's very first message is a literal
// ["AUTH", <event>] reply to the challenge (not ["EVENT", <event>]). Before
// the fix this hit the "reject everything else" branch of the pre-auth gate
// (or the dispatcher's default case) and was rejected as an unknown message
// type, leaving the client stuck unauthenticated.
func TestNIP42AuthMessageTypeWithRequireAuth(t *testing.T) {
	url, r, cleanup, _ := setupRelay(t)
	r.SetRequireAuth(true)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	require.NoError(t, err)
	defer client.Close()

	// Relay pushes an ["AUTH", <challenge>] message on connect.
	msg, err := client.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "AUTH", msg[0])
	challenge, ok := msg[1].(string)
	require.True(t, ok)
	require.NotEmpty(t, challenge)

	sk := nostr.GeneratePrivateKey()
	pk, err := nostr.GetPublicKey(sk)
	require.NoError(t, err)

	authEvent := &nostr.Event{
		Kind:      22242,
		Content:   challenge,
		CreatedAt: nostr.Now(),
		Tags:      nostr.Tags{},
	}
	authEvent.PubKey = pk
	err = authEvent.Sign(sk)
	require.NoError(t, err)

	localAuthEvent := convertNostrEventToLocalEvent(authEvent)

	err = client.SendAuth(localAuthEvent)
	require.NoError(t, err)

	accepted, respMsg, err := client.ExpectOK(localAuthEvent.ID, 2*time.Second)
	require.NoError(t, err)
	assert.True(t, accepted)
	assert.NotEmpty(t, respMsg)
}
