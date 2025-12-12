package event_test

import (
	"testing"

	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
)

func TestEvent_Validate(t *testing.T) {
	validEvent, _ := testutil.MustNewTestEvent(1, "test content", nil)

	tests := []struct {
		name      string
		event     *event.Event
		expectErr bool
	}{
		{
			name:      "valid event",
			event:     validEvent,
			expectErr: false,
		},
		{
			name: "missing pubkey",
			event: &event.Event{
				Kind:    validEvent.Kind,
				Tags:    validEvent.Tags,
				Content: validEvent.Content,
				Sig:     validEvent.Sig,
			},
			expectErr: true,
		},
		{
			name: "missing signature",
			event: &event.Event{
				ID:        validEvent.ID,
				PubKey:    validEvent.PubKey,
				CreatedAt: validEvent.CreatedAt,
				Kind:      validEvent.Kind,
				Tags:      validEvent.Tags,
				Content:   validEvent.Content,
				Sig:       "",
			},
			expectErr: true,
		},
		{
			name: "invalid kind",
			event: &event.Event{
				ID:        validEvent.ID,
				PubKey:    validEvent.PubKey,
				CreatedAt: validEvent.CreatedAt,
				Kind:      -1,
				Tags:      validEvent.Tags,
				Content:   validEvent.Content,
				Sig:       validEvent.Sig,
			},
			expectErr: true,
		},
		{
			name: "ID mismatch",
			event: &event.Event{
				ID:        "invalidid", // Mismatched ID
				PubKey:    validEvent.PubKey,
				CreatedAt: validEvent.CreatedAt,
				Kind:      validEvent.Kind,
				Tags:      validEvent.Tags,
				Content:   validEvent.Content,
				Sig:       validEvent.Sig,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("Event.Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestEvent_Matches(t *testing.T) {
	// Create some test events
	evt1, kp1 := testutil.MustNewTestEvent(1, "content 1", nil)
	evt2, _ := testutil.NewTestEventWithKey(kp1, 2, "content 2", nil)
	evt3, kp2 := testutil.MustNewTestEvent(1, "content 3", [][]string{{"e", evt1.ID}, {"t", "test"}})
	evt4, _ := testutil.NewTestEventWithKey(kp2, 3, "content 4", [][]string{{"p", kp1.PubKeyHex}, {"t", "another"}})

	tests := []struct {
		name     string
		event    *event.Event
		filter   *event.Filter
		expected bool
	}{
		{
			name:     "match by ID",
			event:    evt1,
			filter:   &event.Filter{IDs: []string{evt1.ID}},
			expected: true,
		},
		{
			name:     "no match by ID",
			event:    evt1,
			filter:   &event.Filter{IDs: []string{evt2.ID}},
			expected: false,
		},
		{
			name:     "match by ID prefix",
			event:    evt1,
			filter:   &event.Filter{IDs: []string{evt1.ID[:8]}},
			expected: true,
		},
		{
			name:     "match by author",
			event:    evt1,
			filter:   &event.Filter{Authors: []string{kp1.PubKeyHex}},
			expected: true,
		},
		{
			name:     "no match by author",
			event:    evt1,
			filter:   &event.Filter{Authors: []string{kp2.PubKeyHex}},
			expected: false,
		},
		{
			name:     "match by author prefix",
			event:    evt1,
			filter:   &event.Filter{Authors: []string{kp1.PubKeyHex[:8]}},
			expected: true,
		},
		{
			name:     "match by kind",
			event:    evt1,
			filter:   &event.Filter{Kinds: []int{1}},
			expected: true,
		},
		{
			name:     "no match by kind",
			event:    evt1,
			filter:   &event.Filter{Kinds: []int{2}},
			expected: false,
		},
		{
			name:     "match by #e tag",
			event:    evt3,
			filter:   &event.Filter{Tags: map[string][]string{"e": {evt1.ID}}},
			expected: true,
		},
		{
			name:     "match by #e tag prefix",
			event:    evt3,
			filter:   &event.Filter{Tags: map[string][]string{"e": {evt1.ID[:8]}}},
			expected: true,
		},
		{
			name:     "no match by #e tag",
			event:    evt3,
			filter:   &event.Filter{Tags: map[string][]string{"e": {evt2.ID}}},
			expected: false,
		},
		{
			name:     "match by #p tag",
			event:    evt4,
			filter:   &event.Filter{Tags: map[string][]string{"p": {kp1.PubKeyHex}}},
			expected: true,
		},
		{
			name:     "match by #p tag prefix",
			event:    evt4,
			filter:   &event.Filter{Tags: map[string][]string{"p": {kp1.PubKeyHex[:8]}}},
			expected: true,
		},
		{
			name:     "no match by #p tag",
			event:    evt4,
			filter:   &event.Filter{Tags: map[string][]string{"p": {kp2.PubKeyHex}}},
			expected: false,
		},
		{
			name:     "match by multiple filters (AND logic)",
			event:    evt3,
			filter:   &event.Filter{Kinds: []int{1}, Tags: map[string][]string{"e": {evt1.ID}}},
			expected: true,
		},
		{
			name:     "no match by multiple filters (AND logic)",
			event:    evt3,
			filter:   &event.Filter{Kinds: []int{2}, Tags: map[string][]string{"e": {evt1.ID}}},
			expected: false,
		},
		{
			name:     "match by since",
			event:    evt1,
			filter:   &event.Filter{Since: int64Ptr(evt1.CreatedAt - 1)},
			expected: true,
		},
		{
			name:     "no match by since",
			event:    evt1,
			filter:   &event.Filter{Since: int64Ptr(evt1.CreatedAt + 1)},
			expected: false,
		},
		{
			name:     "match by until",
			event:    evt1,
			filter:   &event.Filter{Until: int64Ptr(evt1.CreatedAt + 1)},
			expected: true,
		},
		{
			name:     "no match by until",
			event:    evt1,
			filter:   &event.Filter{Until: int64Ptr(evt1.CreatedAt - 1)},
			expected: false,
		},
		{
			name:     "match by limit (not directly testable in Matches)",
			event:    evt1,
			filter:   &event.Filter{Limit: intPtr(1)},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.event.Matches(tt.filter)
			if actual != tt.expected {
				t.Errorf("Event.Matches() for %s got %v, expected %v", tt.name, actual, tt.expected)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}
