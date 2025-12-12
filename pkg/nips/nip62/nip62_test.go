package nip62

import (
	"testing"

	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

func TestIsRequestToVanishEvent(t *testing.T) {
	t.Run("identifies Request to Vanish events", func(t *testing.T) {
		evt := &event.Event{Kind: KindRequestToVanish}
		assert.True(t, IsRequestToVanishEvent(evt))
	})

	t.Run("rejects non-Request to Vanish events", func(t *testing.T) {
		evt := &event.Event{Kind: 1}
		assert.False(t, IsRequestToVanishEvent(evt))
	})
}

func TestValidateRequestToVanish(t *testing.T) {
	t.Run("valid Request to Vanish with relay tag", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{{"relay", "ws://example.com"}},
		}
		err := ValidateRequestToVanish(evt)
		assert.NoError(t, err)
	})

	t.Run("valid Request to Vanish with ALL_RELAYS tag", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{{"relay", "ALL_RELAYS"}},
		}
		err := ValidateRequestToVanish(evt)
		assert.NoError(t, err)
	})

	t.Run("valid Request to Vanish with multiple relay tags", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{
				{"relay", "ws://example1.com"},
				{"relay", "ws://example2.com"},
			},
		}
		err := ValidateRequestToVanish(evt)
		assert.NoError(t, err)
	})

	t.Run("invalid Request to Vanish with no relay tags", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{{"e", "someid"}},
		}
		err := ValidateRequestToVanish(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must include at least one relay tag")
	})

	t.Run("invalid Request to Vanish with empty relay tag value", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{{"relay", ""}},
		}
		err := ValidateRequestToVanish(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "relay tag value cannot be empty")
	})

	t.Run("non-Request to Vanish event", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1,
			Tags: [][]string{{"something", "else"}},
		}
		err := ValidateRequestToVanish(evt)
		assert.NoError(t, err) // Should not error for non-Request to Vanish events
	})

	t.Run("Request to Vanish with whitespace-only relay tag", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{{"relay", "   "}},
		}
		err := ValidateRequestToVanish(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "relay tag value cannot be empty")
	})
}

func TestIsGlobalRequest(t *testing.T) {
	t.Run("identifies global Request to Vanish", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{{"relay", "ALL_RELAYS"}},
		}
		assert.True(t, IsGlobalRequest(evt))
	})

	t.Run("identifies non-global Request to Vanish", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{{"relay", "ws://example.com"}},
		}
		assert.False(t, IsGlobalRequest(evt))
	})

	t.Run("handles non-Request to Vanish events", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1,
			Tags: [][]string{{"relay", "ALL_RELAYS"}},
		}
		assert.False(t, IsGlobalRequest(evt))
	})

	t.Run("handles empty tags", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{},
		}
		assert.False(t, IsGlobalRequest(evt))
	})
}

func TestGetRelayTags(t *testing.T) {
	t.Run("extracts single relay URL", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{{"relay", "ws://example.com"}},
		}
		relays := GetRelayTags(evt)
		assert.Equal(t, []string{"ws://example.com"}, relays)
	})

	t.Run("extracts multiple relay URLs", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{
				{"relay", "ws://example1.com"},
				{"relay", "ws://example2.com"},
				{"relay", "ALL_RELAYS"},
			},
		}
		relays := GetRelayTags(evt)
		assert.Equal(t, []string{"ws://example1.com", "ws://example2.com", "ALL_RELAYS"}, relays)
	})

	t.Run("handles non-Request to Vanish events", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1,
			Tags: [][]string{{"relay", "ws://example.com"}},
		}
		relays := GetRelayTags(evt)
		assert.Empty(t, relays)
	})

	t.Run("handles empty tags", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{},
		}
		relays := GetRelayTags(evt)
		assert.Empty(t, relays)
	})

	t.Run("ignores non-relay tags", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{
				{"e", "someid"},
				{"p", "somepubkey"},
				{"relay", "ws://example.com"},
				{"d", "somedomain"},
			},
		}
		relays := GetRelayTags(evt)
		assert.Equal(t, []string{"ws://example.com"}, relays)
	})

	t.Run("trims whitespace from relay URLs", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{{"relay", "  ws://example.com  "}},
		}
		relays := GetRelayTags(evt)
		assert.Equal(t, []string{"ws://example.com"}, relays)
	})

	t.Run("skips empty relay URLs", func(t *testing.T) {
		evt := &event.Event{
			Kind: KindRequestToVanish,
			Tags: [][]string{
				{"relay", "ws://example1.com"},
				{"relay", ""},
				{"relay", "ws://example2.com"},
			},
		}
		relays := GetRelayTags(evt)
		assert.Equal(t, []string{"ws://example1.com", "ws://example2.com"}, relays)
	})
}
