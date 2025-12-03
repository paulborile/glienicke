package integration

import (
	"strconv"
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

func TestNIP40_ExpirationTimestamp(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Test 1: Event with future expiration should be accepted
	futureTime := time.Now().Add(1 * time.Hour).Unix()
	tags := [][]string{{"expiration", strconv.FormatInt(futureTime, 10)}}
	evt, _ := testutil.MustNewTestEvent(1, "This expires in 1 hour", tags)

	err = client.SendEvent(evt)
	assert.NoError(t, err)

	// Should be accepted
	accepted, msg, err := client.ExpectOK(evt.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Event with future expiration should be accepted")
	assert.Empty(t, msg, "Message should be empty")

	// Test 2: Event with past expiration should be rejected
	pastTime := time.Now().Add(-1 * time.Hour).Unix()
	pastTags := [][]string{{"expiration", strconv.FormatInt(pastTime, 10)}}
	pastEvt, _ := testutil.MustNewTestEvent(1, "This expired 1 hour ago", pastTags)

	err = client.SendEvent(pastEvt)
	assert.NoError(t, err)

	// Should be rejected
	accepted, msg, err = client.ExpectOK(pastEvt.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.False(t, accepted, "Event with past expiration should be rejected")
	assert.NotEmpty(t, msg, "Should have rejection message")

	// Test 3: Query should not return expired events
	// Create an event that expires in 2 seconds (enough time to be accepted)
	verySoon := time.Now().Add(2 * time.Second).Unix()
	soonTags := [][]string{{"expiration", strconv.FormatInt(verySoon, 10)}}
	soonEvt, _ := testutil.MustNewTestEvent(1, "This expires very soon", soonTags)

	err = client.SendEvent(soonEvt)
	assert.NoError(t, err)

	// Should be accepted initially
	accepted, msg, err = client.ExpectOK(soonEvt.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Event with near-future expiration should be accepted")

	// Wait for it to expire
	time.Sleep(3 * time.Second)

	// Try to query for it
	filter := &event.Filter{
		IDs: []string{soonEvt.ID},
	}
	err = client.SendReq("test-sub", filter)
	assert.NoError(t, err)

	// Should not receive the expired event
	events, err := client.CollectEvents("test-sub", 2*time.Second)
	assert.NoError(t, err)
	assert.Empty(t, events, "Should not receive expired event in query")

	// Test 4: Event without expiration tag should work normally
	normalEvt, _ := testutil.MustNewTestEvent(1, "Normal event", nil)

	err = client.SendEvent(normalEvt)
	assert.NoError(t, err)

	accepted, msg, err = client.ExpectOK(normalEvt.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Normal event should be accepted")

	// Should be queryable
	filter = &event.Filter{
		IDs: []string{normalEvt.ID},
	}
	err = client.SendReq("test-sub2", filter)
	assert.NoError(t, err)

	events, err = client.CollectEvents("test-sub2", 2*time.Second)
	assert.NoError(t, err)
	assert.Len(t, events, 1, "Should receive normal event")
	assert.Equal(t, normalEvt.ID, events[0].ID)
}
