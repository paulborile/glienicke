package integration

import (
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

const (
	KindRequestToVanish = 62
)

func TestNIP62_RequestToVanish_RelaxSpecific(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Create a test event
	evt, kp := testutil.MustNewTestEvent(KindTextNote, "Hello, Nostr!", nil)

	// Send EVENT message
	err = client.SendEvent(evt)
	assert.NoError(t, err)

	// Expect OK response
	accepted, msg, err := client.ExpectOK(evt.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Event should be accepted")
	assert.Empty(t, msg, "Message should be empty")

	// Verify the event can be queried
	filter := &event.Filter{
		Authors: []string{evt.PubKey},
	}
	err = client.SendReq("test-sub", filter)
	assert.NoError(t, err)

	events, err := client.CollectEvents("test-sub", 2*time.Second)
	assert.NoError(t, err)
	assert.Len(t, events, 1, "Should receive the event")

	// Create a Request to Vanish event for this specific relay
	tags := [][]string{{"relay", "ws://localhost:8080"}}
	vanishEvt, _ := testutil.NewTestEventWithKey(kp, KindRequestToVanish, "Please delete my data", tags)

	// Send the Request to Vanish event
	err = client.SendEvent(vanishEvt)
	assert.NoError(t, err)

	// Expect OK response for vanish request
	accepted, msg, err = client.ExpectOK(vanishEvt.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Request to Vanish should be accepted")

	// Give the relay a moment to process the deletion
	time.Sleep(100 * time.Millisecond)

	// Now, try to query events from the pubkey again
	err = client.SendReq("test-sub-2", filter)
	assert.NoError(t, err)

	// We should not receive any events, only EOSE
	events, err = client.CollectEvents("test-sub-2", 2*time.Second)
	assert.NoError(t, err)
	assert.Empty(t, events, "Should not receive any events after Request to Vanish")
}

func TestNIP62_RequestToVanish_Global(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Create a test event
	evt, kp := testutil.MustNewTestEvent(KindTextNote, "Hello, Nostr!", nil)

	// Send EVENT message
	err = client.SendEvent(evt)
	assert.NoError(t, err)

	// Expect OK response
	accepted, msg, err := client.ExpectOK(evt.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Event should be accepted")
	assert.Empty(t, msg, "Message should be empty")

	// Verify the event can be queried
	filter := &event.Filter{
		Authors: []string{evt.PubKey},
	}
	err = client.SendReq("test-sub", filter)
	assert.NoError(t, err)

	events, err := client.CollectEvents("test-sub", 2*time.Second)
	assert.NoError(t, err)
	assert.Len(t, events, 1, "Should receive the event")

	// Create a global Request to Vanish event
	tags := [][]string{{"relay", "ALL_RELAYS"}}
	vanishEvt, _ := testutil.NewTestEventWithKey(kp, KindRequestToVanish, "Global delete request", tags)

	// Send the Request to Vanish event
	err = client.SendEvent(vanishEvt)
	assert.NoError(t, err)

	// Expect OK response for vanish request
	accepted, msg, err = client.ExpectOK(vanishEvt.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Request to Vanish should be accepted")

	// Give the relay a moment to process the deletion
	time.Sleep(100 * time.Millisecond)

	// Now, try to query events from the pubkey again
	err = client.SendReq("test-sub-2", filter)
	assert.NoError(t, err)

	// We should not receive any events, only EOSE
	events, err = client.CollectEvents("test-sub-2", 2*time.Second)
	assert.NoError(t, err)
	assert.Empty(t, events, "Should not receive any events after Request to Vanish")
}

func TestNIP62_RequestToVanish_IgnoreOtherRelays(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Create a test event
	evt, kp := testutil.MustNewTestEvent(KindTextNote, "Hello, Nostr!", nil)

	// Send EVENT message
	err = client.SendEvent(evt)
	assert.NoError(t, err)

	// Expect OK response
	accepted, msg, err := client.ExpectOK(evt.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Event should be accepted")
	assert.Empty(t, msg, "Message should be empty")

	// Verify the event can be queried
	filter := &event.Filter{
		Authors: []string{evt.PubKey},
	}
	err = client.SendReq("test-sub", filter)
	assert.NoError(t, err)

	events, err := client.CollectEvents("test-sub", 2*time.Second)
	assert.NoError(t, err)
	assert.Len(t, events, 1, "Should receive the event")

	// Create a Request to Vanish event for a DIFFERENT relay
	tags := [][]string{{"relay", "ws://different-relay.com"}}
	vanishEvt, _ := testutil.NewTestEventWithKey(kp, KindRequestToVanish, "Delete from other relay", tags)

	// Send the Request to Vanish event
	err = client.SendEvent(vanishEvt)
	assert.NoError(t, err)

	// Expect OK response for vanish request
	accepted, msg, err = client.ExpectOK(vanishEvt.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Request to Vanish should be accepted")

	// Give the relay a moment to process
	time.Sleep(100 * time.Millisecond)

	// The event should still be there since the request was for a different relay
	err = client.SendReq("test-sub-2", filter)
	assert.NoError(t, err)

	// We should still receive the event
	events, err = client.CollectEvents("test-sub-2", 2*time.Second)
	assert.NoError(t, err)
	assert.Len(t, events, 1, "Should still receive the event since request was for different relay")
}

func TestNIP62_RequestToVanish_InvalidEvent(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Create a Request to Vanish event without relay tags (invalid)
	evt, _ := testutil.MustNewTestEvent(KindRequestToVanish, "Invalid request", nil)

	// Send the invalid Request to Vanish event
	err = client.SendEvent(evt)
	assert.NoError(t, err)

	// Expect rejection
	accepted, msg, err := client.ExpectOK(evt.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.False(t, accepted, "Invalid Request to Vanish should be rejected")
	assert.Contains(t, msg, "must include at least one relay tag")
}
