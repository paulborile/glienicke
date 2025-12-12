package storage

import (
	"context"
	"errors"

	"github.com/paul/glienicke/pkg/event"
)

var ErrNotFound = errors.New("event not found")

// Store defines the interface for event storage
// Implementations can use any backend (postgres, sqlite, memory, etc.)
type Store interface {
	// SaveEvent stores an event
	SaveEvent(ctx context.Context, evt *event.Event) error

	// QueryEvents retrieves events matching the filters
	// If multiple filters are provided, they are OR'd together
	QueryEvents(ctx context.Context, filters []*event.Filter) ([]*event.Event, error)

	// DeleteEvent marks an event as deleted
	// deleterPubKey is the pubkey of the entity requesting deletion
	DeleteEvent(ctx context.Context, eventID string, deleterPubKey string) error

	// DeleteAllEventsByPubKey deletes all events from a specific pubkey (NIP-62)
	DeleteAllEventsByPubKey(ctx context.Context, pubkey string) error

	// GetEvent retrieves a single event by ID
	GetEvent(ctx context.Context, eventID string) (*event.Event, error)

	// Close closes the storage connection
	Close() error

	// CountEvents returns the count of events matching the filters
	CountEvents(ctx context.Context, filters []*event.Filter) (int, error)
}
