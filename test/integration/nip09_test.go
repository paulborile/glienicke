package integration

import (
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

const (
	KindTextNote = 1
	KindDeletion = 5
)

func TestNIP09_EventDeletion(t *testing.T) {
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

	// Create a deletion event
	tags := [][]string{{"e", evt.ID}}
	delEvt, _ := testutil.NewTestEventWithKey(kp, KindDeletion, "", tags)

	// Send the deletion event
	err = client.SendEvent(delEvt)
	assert.NoError(t, err)

	// Give the relay a moment to process the deletion
	time.Sleep(50 * time.Millisecond)

	// Now, try to subscribe to the original event
	filter := &event.Filter{
		IDs: []string{evt.ID},
	}
	err = client.SendReq("test-sub", filter)
	assert.NoError(t, err)

	// We should not receive any event, only EOSE
	events, err := client.CollectEvents("test-sub", 2*time.Second)
	assert.NoError(t, err)
	assert.Empty(t, events, "Should not receive any events after deletion")
}
