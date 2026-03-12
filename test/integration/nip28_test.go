package integration

import (
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

const (
	KindChannelCreate   = 40
	KindChannelMetadata = 41
	KindChannelMessage  = 42
)

func TestNIP28_ChannelLifecycle(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Create test user (channel creator)
	creatorEvt, kp1 := testutil.MustNewTestEvent(1, "creator", nil)
	err = client.SendEvent(creatorEvt)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	channelID := "test-channel-123"

	// Step 1: Create a channel (kind 40)
	createTags := [][]string{
		{"channel_id", channelID},
	}
	createContent := `{"name": "Test Channel", "about": "A test channel for NIP-28", "picture": "https://example.com/icon.png"}`
	createEvent, err := testutil.NewTestEventWithKey(kp1, KindChannelCreate, createContent, createTags)
	assert.NoError(t, err)
	createEvent.CreatedAt = time.Now().Unix()
	kp1.SignEvent(createEvent)

	err = client.SendEvent(createEvent)
	assert.NoError(t, err)

	accepted, msg, err := client.ExpectOK(createEvent.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Channel create should be accepted: "+msg)

	// Step 2: Subscribe to channel
	subID := "channel-sub"
	filter := &event.Filter{
		Tags:  map[string][]string{"channel_id": {channelID}},
		Kinds: []int{KindChannelCreate, KindChannelMetadata, KindChannelMessage},
	}

	err = client.SendReq(subID, filter)
	assert.NoError(t, err)

	// Wait for EOSE (should have create event)
	err = client.ExpectEOSE(subID, 3*time.Second)
	assert.NoError(t, err)

	// Step 3: Set channel metadata (kind 41)
	metaTags := [][]string{
		{"channel_id", channelID},
	}
	metaContent := `{"name": "Updated Channel", "about": "Updated description"}`
	metaEvent, err := testutil.NewTestEventWithKey(kp1, KindChannelMetadata, metaContent, metaTags)
	assert.NoError(t, err)
	metaEvent.CreatedAt = time.Now().Unix()
	kp1.SignEvent(metaEvent)

	err = client.SendEvent(metaEvent)
	assert.NoError(t, err)

	accepted, msg, err = client.ExpectOK(metaEvent.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Channel metadata should be accepted: "+msg)

	// Wait for metadata to be broadcast via subscription
	receivedMeta, err := client.ExpectEvent(subID, 3*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, KindChannelMetadata, receivedMeta.Kind)

	// Step 4: Send a message to the channel (kind 42)
	msgTags := [][]string{
		{"channel_id", channelID},
	}
	msgEvent, err := testutil.NewTestEventWithKey(kp1, KindChannelMessage, "Hello from the test!", msgTags)
	assert.NoError(t, err)
	msgEvent.CreatedAt = time.Now().Unix()
	kp1.SignEvent(msgEvent)

	err = client.SendEvent(msgEvent)
	assert.NoError(t, err)

	accepted, msg, err = client.ExpectOK(msgEvent.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Channel message should be accepted: "+msg)

	// Wait for message to be broadcast
	receivedMsg, err := client.ExpectEvent(subID, 3*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, KindChannelMessage, receivedMsg.Kind)
	assert.Equal(t, "Hello from the test!", receivedMsg.Content)
}

func TestNIP28_Validation(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Create test user
	creatorEvt, kp1 := testutil.MustNewTestEvent(1, "creator", nil)
	err = client.SendEvent(creatorEvt)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Test 1: Empty channel name should be rejected
	badChannelTags := [][]string{
		{"channel_id", "bad-channel"},
	}
	badChannelEvent, err := testutil.NewTestEventWithKey(kp1, KindChannelCreate, "", badChannelTags)
	assert.NoError(t, err)
	badChannelEvent.CreatedAt = time.Now().Unix()
	kp1.SignEvent(badChannelEvent)

	err = client.SendEvent(badChannelEvent)
	assert.NoError(t, err)

	accepted, msg, err := client.ExpectOK(badChannelEvent.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.False(t, accepted, "Empty channel name should be rejected")
	assert.Contains(t, msg, "empty")

	// Test 2: Missing channel_id should be rejected
	noChannelTags := [][]string{}
	noChannelEvent, err := testutil.NewTestEventWithKey(kp1, KindChannelMessage, "test", noChannelTags)
	assert.NoError(t, err)
	noChannelEvent.CreatedAt = time.Now().Unix()
	kp1.SignEvent(noChannelEvent)

	err = client.SendEvent(noChannelEvent)
	assert.NoError(t, err)

	accepted, msg, err = client.ExpectOK(noChannelEvent.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.False(t, accepted, "Missing channel_id should be rejected")
	assert.Contains(t, msg, "channel")

	// Test 3: Empty message content should be rejected
	emptyMsgTags := [][]string{
		{"channel_id", "test-channel"},
	}
	emptyMsgEvent, err := testutil.NewTestEventWithKey(kp1, KindChannelMessage, "   ", emptyMsgTags)
	assert.NoError(t, err)
	emptyMsgEvent.CreatedAt = time.Now().Unix()
	kp1.SignEvent(emptyMsgEvent)

	err = client.SendEvent(emptyMsgEvent)
	assert.NoError(t, err)

	accepted, msg, err = client.ExpectOK(emptyMsgEvent.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.False(t, accepted, "Empty message should be rejected")
	assert.Contains(t, msg, "empty")
}

func TestNIP28_ChannelSubscription(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client1, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client1.Close()

	client2, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client2.Close()

	// Create channel creator
	creatorEvt, kp1 := testutil.MustNewTestEvent(1, "creator", nil)
	err = client1.SendEvent(creatorEvt)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	channelID := "broadcast-test-channel"

	// Create channel
	createTags := [][]string{{"channel_id", channelID}}
	createEvent, _ := testutil.NewTestEventWithKey(kp1, KindChannelCreate, `{"name": "Broadcast Test"}`, createTags)
	createEvent.CreatedAt = time.Now().Unix()
	kp1.SignEvent(createEvent)

	err = client1.SendEvent(createEvent)
	assert.NoError(t, err)
	client1.ExpectOK(createEvent.ID, 2*time.Second)

	time.Sleep(50 * time.Millisecond)

	// Client 1 subscribes to channel
	err = client1.SendReq("sub1", &event.Filter{
		Tags:  map[string][]string{"channel_id": {channelID}},
		Kinds: []int{KindChannelMessage},
	})
	assert.NoError(t, err)
	client1.ExpectEOSE("sub1", 2*time.Second)

	// Client 2 subscribes to channel
	err = client2.SendReq("sub2", &event.Filter{
		Tags:  map[string][]string{"channel_id": {channelID}},
		Kinds: []int{KindChannelMessage},
	})
	assert.NoError(t, err)
	client2.ExpectEOSE("sub2", 2*time.Second)

	time.Sleep(50 * time.Millisecond)

	// Client 1 sends a message
	msgTags := [][]string{{"channel_id", channelID}}
	msgEvent, _ := testutil.NewTestEventWithKey(kp1, KindChannelMessage, "Broadcast message", msgTags)
	msgEvent.CreatedAt = time.Now().Unix()
	kp1.SignEvent(msgEvent)

	err = client1.SendEvent(msgEvent)
	assert.NoError(t, err)
	client1.ExpectOK(msgEvent.ID, 2*time.Second)

	// Both clients should receive the broadcast via ExpectEvent (not CollectEvents)
	received1, err := client1.ExpectEvent("sub1", 3*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, KindChannelMessage, received1.Kind)
	assert.Equal(t, "Broadcast message", received1.Content)

	received2, err := client2.ExpectEvent("sub2", 3*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, KindChannelMessage, received2.Kind)
	assert.Equal(t, "Broadcast message", received2.Content)
}

func TestNIP28_ChannelDeletion(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Create test user (channel creator)
	creatorEvt, kp1 := testutil.MustNewTestEvent(1, "creator", nil)
	err = client.SendEvent(creatorEvt)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	channelID := "delete-test-channel"

	// Step 1: Create a channel (kind 40)
	createTags := [][]string{{"channel_id", channelID}}
	createEvent, err := testutil.NewTestEventWithKey(kp1, KindChannelCreate, `{"name": "Delete Test"}`, createTags)
	assert.NoError(t, err)
	createEvent.CreatedAt = time.Now().Unix()
	kp1.SignEvent(createEvent)

	err = client.SendEvent(createEvent)
	assert.NoError(t, err)

	accepted, msg, err := client.ExpectOK(createEvent.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Channel create should be accepted: "+msg)

	// Step 2: Send some messages
	for i := 0; i < 3; i++ {
		msgTags := [][]string{{"channel_id", channelID}}
		msgEvent, err := testutil.NewTestEventWithKey(kp1, KindChannelMessage, "Test message", msgTags)
		assert.NoError(t, err)
		msgEvent.CreatedAt = time.Now().Unix()
		kp1.SignEvent(msgEvent)

		err = client.SendEvent(msgEvent)
		assert.NoError(t, err)

		accepted, _, err := client.ExpectOK(msgEvent.ID, 2*time.Second)
		assert.NoError(t, err)
		assert.True(t, accepted)
	}

	// Step 3: Delete the channel (by deleting the create event with kind 40)
	deletionTags := [][]string{{"e", createEvent.ID}}
	deletionEvent, err := testutil.NewTestEventWithKey(kp1, KindDeletion, "Delete channel", deletionTags)
	assert.NoError(t, err)
	deletionEvent.CreatedAt = time.Now().Unix()
	kp1.SignEvent(deletionEvent)

	err = client.SendEvent(deletionEvent)
	assert.NoError(t, err)

	accepted, msg, err = client.ExpectOK(deletionEvent.ID, 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, accepted, "Deletion should be accepted: "+msg)

	// Wait for deletion to process
	time.Sleep(100 * time.Millisecond)

	// Step 4: Subscribe to verify channel is deleted
	subID := "after-delete-sub"
	err = client.SendReq(subID, &event.Filter{
		Tags:  map[string][]string{"channel_id": {channelID}},
		Kinds: []int{KindChannelCreate, KindChannelMetadata, KindChannelMessage},
	})
	assert.NoError(t, err)

	// Wait for EOSE - should receive 0 events (channel was deleted)
	err = client.ExpectEOSE(subID, 2*time.Second)
	assert.NoError(t, err, "Should receive EOSE even with 0 events")
}
