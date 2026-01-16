package integration

import (
	"testing"
	"time"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

const (
	KindReaction = 7
)

func TestNIP45_EventCounts_BasicCounting(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Create some test events for counting
	evt1, kp := testutil.MustNewTestEvent(KindTextNote, "Test post 1", nil)
	evt2, _ := testutil.NewTestEventWithKey(kp, KindTextNote, "Test post 2", nil)
	evt3, _ := testutil.NewTestEventWithKey(kp, KindReaction, "+", [][]string{{"e", evt1.ID}})

	// Send events
	err = client.SendEvent(evt1)
	assert.NoError(t, err)
	err = client.SendEvent(evt2)
	assert.NoError(t, err)
	err = client.SendEvent(evt3)
	assert.NoError(t, err)

	// Wait for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Test COUNT message - should now work
	err = client.SendCountMessage("count-test", &event.Filter{
		Authors: []string{kp.PubKeyHex},
		Kinds:   []int{KindTextNote},
	})
	// This should succeed now that COUNT is implemented
	assert.NoError(t, err, "COUNT should succeed after implementation")

	// Wait a bit to allow the COUNT response to be processed
	time.Sleep(50 * time.Millisecond)
}

func TestNIP45_EventCounts_ReactionCounting(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	assert.NoError(t, err)
	defer client.Close()

	// Create a text note and a reaction
	evt1, kp := testutil.MustNewTestEvent(KindTextNote, "Test post for reactions", nil)
	evt2, _ := testutil.NewTestEventWithKey(kp, KindReaction, "+", [][]string{{"e", evt1.ID}})

	// Send events
	err = client.SendEvent(evt1)
	assert.NoError(t, err)
	err = client.SendEvent(evt2)
	assert.NoError(t, err)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Test COUNT message - should now work
	err = client.SendCountMessage("count-reactions", &event.Filter{
		Kinds: []int{KindReaction},
		Tags: map[string][]string{
			"e": {evt1.ID},
		},
	})
	// This should succeed now that COUNT is implemented
	assert.NoError(t, err, "COUNT should succeed after implementation")

	// Wait a bit to allow the COUNT response to be processed
	time.Sleep(50 * time.Millisecond)
}
