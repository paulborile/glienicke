package integration

import (
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

const (
	KindFollowList = 3
)

func TestNIP02_FollowList(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Create test users
	user1, kp1 := testutil.MustNewTestEvent(KindTextNote, "User1 test note", nil)
	user2, _ := testutil.MustNewTestEvent(KindTextNote, "User2 test note", nil)

	// Send initial events to establish users
	err = client.SendEvent(user1)
	assert.NoError(t, err)
	err = client.SendEvent(user2)
	assert.NoError(t, err)

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	// Create a follow list event for user1 following user2
	tags := [][]string{
		{"p", user2.PubKey, "wss://relay.example.com", "user2"},                   // Full tag with relay and petname
		{"p", "0000000000000000000000000000000000000000000000000000000000000001"}, // Minimal tag with just pubkey
	}

	followEvent, _ := testutil.NewTestEventWithKey(kp1, KindFollowList, "", tags)
	// Set a realistic timestamp
	followEvent.CreatedAt = time.Now().Unix()
	// Re-sign the event with the new timestamp
	kp1.SignEvent(followEvent)

	// Send the follow list event
	err = client.SendEvent(followEvent)
	assert.NoError(t, err)

	// Expect OK response
	accepted, msg, err := client.ExpectOK(followEvent.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Follow list event should be accepted")
	assert.Empty(t, msg, "Message should be empty")

	// Give the relay a moment to process the follow list
	time.Sleep(50 * time.Millisecond)

	// Test 1: Query for user1's follow list
	filter := &event.Filter{
		Authors: []string{user1.PubKey},
		Kinds:   []int{KindFollowList},
		Limit:   func() *int { l := 1; return &l }(),
	}
	err = client.SendReq("follow-sub", filter)
	assert.NoError(t, err)

	// Should receive the follow list event
	events, err := client.CollectEvents("follow-sub", 2*time.Second)
	assert.NoError(t, err)
	assert.Len(t, events, 1, "Should receive exactly one follow list event")

	receivedFollowEvent := events[0]
	assert.Equal(t, KindFollowList, receivedFollowEvent.Kind)
	assert.Equal(t, "", receivedFollowEvent.Content, "Follow list content should be empty")
	assert.Equal(t, user1.PubKey, receivedFollowEvent.PubKey)

	// Verify the follow list contains the expected p tags
	assert.Len(t, receivedFollowEvent.Tags, 2, "Should have 2 p tags")

	// Check first p tag (full format)
	assert.Equal(t, "p", receivedFollowEvent.Tags[0][0])
	assert.Equal(t, user2.PubKey, receivedFollowEvent.Tags[0][1])
	assert.Equal(t, "wss://relay.example.com", receivedFollowEvent.Tags[0][2])
	assert.Equal(t, "user2", receivedFollowEvent.Tags[0][3])

	// Check second p tag (minimal format)
	assert.Equal(t, "p", receivedFollowEvent.Tags[1][0])
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000000000001", receivedFollowEvent.Tags[1][1])

	// Test 2: Create a new follow list that should replace the old one
	newTags := [][]string{
		{"p", user2.PubKey}, // Keep user2 but with minimal tag
		{"p", "0000000000000000000000000000000000000000000000000000000000000002"}, // Add new user
	}

	// Wait a moment to ensure different CreatedAt timestamp
	time.Sleep(10 * time.Millisecond)

	newFollowEvent, _ := testutil.NewTestEventWithKey(kp1, KindFollowList, "", newTags)
	// Set a newer timestamp
	newFollowEvent.CreatedAt = followEvent.CreatedAt + 1
	// Re-sign the event with the new timestamp
	kp1.SignEvent(newFollowEvent)

	// Send the new follow list event
	err = client.SendEvent(newFollowEvent)
	assert.NoError(t, err)

	// Expect OK response
	accepted, msg, err = client.ExpectOK(newFollowEvent.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "New follow list event should be accepted")
	assert.Empty(t, msg, "Message should be empty")

	// Give the relay a moment to process
	time.Sleep(50 * time.Millisecond)

	// Query for user1's follow list again - should only get the newest one
	err = client.SendReq("follow-sub-2", filter)
	assert.NoError(t, err)

	events, err = client.CollectEvents("follow-sub-2", 2*time.Second)
	assert.NoError(t, err)
	assert.Len(t, events, 1, "Should receive exactly one follow list event (the newest)")

	newestFollowEvent := events[0]
	assert.Equal(t, newFollowEvent.ID, newestFollowEvent.ID, "Should receive the newest follow list event")
	assert.Len(t, newestFollowEvent.Tags, 2, "New follow list should have 2 p tags")
}

func TestNIP02_FollowListValidation(t *testing.T) {
	url, _, cleanup := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Test invalid follow list events
	testCases := []struct {
		name         string
		tags         [][]string
		content      string
		shouldAccept bool
	}{
		{
			name:         "Valid follow list with empty content",
			tags:         [][]string{{"p", "0000000000000000000000000000000000000000000000000000000000000001"}},
			content:      "",
			shouldAccept: true,
		},
		{
			name:         "Invalid follow list with non-empty content",
			tags:         [][]string{{"p", "0000000000000000000000000000000000000000000000000000000000000001"}},
			content:      "some content",
			shouldAccept: false,
		},
		{
			name:         "Invalid follow list with no p tags",
			tags:         [][]string{{"e", "some_event_id"}},
			content:      "",
			shouldAccept: false,
		},
		{
			name:         "Invalid p tag missing pubkey",
			tags:         [][]string{{"p"}},
			content:      "",
			shouldAccept: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, kp := testutil.MustNewTestEvent(KindTextNote, "Test note", nil)

			followEvent, _ := testutil.NewTestEventWithKey(kp, KindFollowList, tc.content, tc.tags)
			followEvent.CreatedAt = time.Now().Unix()
			kp.SignEvent(followEvent)

			// Send the follow list event
			err = client.SendEvent(followEvent)
			assert.NoError(t, err)

			// Expect OK response
			accepted, msg, err := client.ExpectOK(followEvent.ID, 2*time.Second)
			assert.NoError(t, err)

			if tc.shouldAccept {
				assert.True(t, accepted, "Event should be accepted: %s", tc.name)
				assert.Empty(t, msg, "Message should be empty for accepted event: %s", tc.name)
			} else {
				assert.False(t, accepted, "Event should be rejected: %s", tc.name)
				assert.NotEmpty(t, msg, "Message should explain rejection: %s", tc.name)
			}
		})
	}
}
