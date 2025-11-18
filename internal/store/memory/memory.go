package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/storage"
)

// Store is an in-memory implementation of storage.Store
// This is intended for testing only - not for production use
type Store struct {
	mu      sync.RWMutex
	events  map[string]*event.Event
	deleted map[string]bool // event IDs that have been deleted
}

// Ensure Store implements storage.Store
var _ storage.Store = (*Store)(nil)

// New creates a new in-memory store
func New() *Store {
	return &Store{
		events:  make(map[string]*event.Event),
		deleted: make(map[string]bool),
	}
}

// SaveEvent stores an event in memory
func (s *Store) SaveEvent(ctx context.Context, evt *event.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if event is already deleted
	if s.deleted[evt.ID] {
		return fmt.Errorf("event has been deleted")
	}

	// Store event
	s.events[evt.ID] = evt
	return nil
}

// QueryEvents retrieves events matching the filters
func (s *Store) QueryEvents(ctx context.Context, filters []*event.Filter) ([]*event.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*event.Event
	seen := make(map[string]bool)

	// Process each filter (OR'd together)
	for _, filter := range filters {
		for _, evt := range s.events {
			// Skip if already included or deleted
			if seen[evt.ID] || s.deleted[evt.ID] {
				continue
			}

			// Check if event matches filter
			if evt.Matches(filter) {
				results = append(results, evt)
				seen[evt.ID] = true
			}
		}
	}

	// Sort by created_at descending (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt > results[j].CreatedAt
	})

	// Apply limit if specified (use first filter's limit)
	if len(filters) > 0 && filters[0].Limit != nil {
		limit := *filters[0].Limit
		if len(results) > limit {
			results = results[:limit]
		}
	}

	return results, nil
}

// DeleteEvent marks an event as deleted
func (s *Store) DeleteEvent(ctx context.Context, eventID string, deleterPubKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if event exists
	evt, exists := s.events[eventID]
	if !exists {
		return fmt.Errorf("event not found")
	}

	// Verify deletion authorization (only author can delete)
	if evt.PubKey != deleterPubKey {
		return fmt.Errorf("unauthorized: only event author can delete")
	}

	// Mark as deleted
	s.deleted[eventID] = true
	return nil
}

// GetEvent retrieves a single event by ID
func (s *Store) GetEvent(ctx context.Context, eventID string) (*event.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.deleted[eventID] {
		return nil, fmt.Errorf("event has been deleted")
	}

	evt, exists := s.events[eventID]
	if !exists {
		return nil, storage.ErrNotFound
	}

	return evt, nil
}

// Close is a no-op for in-memory store
func (s *Store) Close() error {
	return nil
}

// Count returns the number of stored events (for testing)
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events) - len(s.deleted)
}
