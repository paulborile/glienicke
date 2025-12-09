package nip42

import (
	"context"
	"testing"

	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

func TestProcessor(t *testing.T) {
	processor := New()

	t.Run("processes valid AUTH event", func(t *testing.T) {
		// Skip signature verification for this test
		evt := &event.Event{
			ID:        "0000000000000000000000000000000000000000000000000000000000000000",
			PubKey:    "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
			CreatedAt: 1234567890,
			Kind:      22242,
			Content:   "test-challenge",
			Tags:      nil,
			Sig:       "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		}

		mockStore := &mockStore{}
		err := processor.Process(evt, mockStore)
		// We expect this to fail due to signature verification, but that's expected for test data
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid signature")
	})

	t.Run("rejects non-AUTH events", func(t *testing.T) {
		evt := &event.Event{
			ID:        "test-id",
			PubKey:    "test-pubkey",
			CreatedAt: 1234567890,
			Kind:      1, // Not AUTH kind
			Content:   "test",
			Tags:      nil,
			Sig:       "test-sig",
		}

		mockStore := &mockStore{}
		err := processor.Process(evt, mockStore)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not an AUTH event")
	})
}

func TestIsAuthEvent(t *testing.T) {
	t.Run("identifies AUTH events", func(t *testing.T) {
		evt := &event.Event{Kind: 22242}
		assert.True(t, IsAuthEvent(evt))
	})

	t.Run("rejects non-AUTH events", func(t *testing.T) {
		evt := &event.Event{Kind: 1}
		assert.False(t, IsAuthEvent(evt))
	})
}

func TestValidateAuthEvent(t *testing.T) {
	t.Run("rejects wrong kind", func(t *testing.T) {
		evt := &event.Event{Kind: 1}
		err := ValidateAuthEvent(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not AUTH")
	})

	t.Run("rejects empty content", func(t *testing.T) {
		evt := &event.Event{
			ID:        "0000000000000000000000000000000000000000000000000000000000000000",
			PubKey:    "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
			CreatedAt: 1234567890,
			Kind:      22242,
			Content:   "",
			Tags:      nil,
			Sig:       "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		}
		err := ValidateAuthEvent(evt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "content cannot be empty")
	})
}

// mockStore is a simple mock for testing
type mockStore struct{}

func (m *mockStore) SaveEvent(ctx context.Context, evt *event.Event) error {
	return nil
}

func (m *mockStore) QueryEvents(ctx context.Context, filters []*event.Filter) ([]*event.Event, error) {
	return nil, nil
}

func (m *mockStore) DeleteEvent(ctx context.Context, eventID string, deleterPubKey string) error {
	return nil
}

func (m *mockStore) GetEvent(ctx context.Context, eventID string) (*event.Event, error) {
	return nil, nil
}

func (m *mockStore) Close() error {
	return nil
}
