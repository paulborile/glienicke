package event

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventFilterMatchingUnit(t *testing.T) {
	// Create test events manually to avoid import cycle
	evt1 := &Event{
		ID:        "1111111111111111111111111111111111111111111111111111111111111111",
		PubKey:    "2222222222222222222222222222222222222222222222222222222222222222",
		CreatedAt: 1234567890,
		Kind:      1,
		Content:   "Event 1",
		Tags:      [][]string{},
		Sig:       "signature1",
	}

	evt2 := &Event{
		ID:        "3333333333333333333333333333333333333333333333333333333333333333",
		PubKey:    "2222222222222222222222222222222222222222222222222222222222222222",
		CreatedAt: 1234567891,
		Kind:      2,
		Content:   "Event 2",
		Tags:      [][]string{},
		Sig:       "signature2",
	}

	evt3 := &Event{
		ID:        "4444444444444444444444444444444444444444444444444444444444444444",
		PubKey:    "5555555555555555555555555555555555555555555555555555555555555555",
		CreatedAt: 1234567892,
		Kind:      1,
		Content:   "Event 3",
		Tags:      [][]string{{"e", evt1.ID}},
		Sig:       "signature3",
	}

	evt4 := &Event{
		ID:        "6666666666666666666666666666666666666666666666666666666666666666",
		PubKey:    "5555555555555555555555555555555555555555555555555555555555555555",
		CreatedAt: 1234567893,
		Kind:      3,
		Content:   "Event 4",
		Tags:      [][]string{{"p", evt2.PubKey}},
		Sig:       "signature4",
	}

	tests := []struct {
		name        string
		filter      *Filter
		event       *Event
		shouldMatch bool
	}{
		{
			name:        "Event 1 matches ID filter",
			filter:      &Filter{IDs: []string{evt1.ID}},
			event:       evt1,
			shouldMatch: true,
		},
		{
			name:        "Event 2 doesn't match ID filter",
			filter:      &Filter{IDs: []string{evt1.ID}},
			event:       evt2,
			shouldMatch: false,
		},
		{
			name:        "Event 1 matches author filter",
			filter:      &Filter{Authors: []string{evt1.PubKey}},
			event:       evt1,
			shouldMatch: true,
		},
		{
			name:        "Event 3 doesn't match author filter",
			filter:      &Filter{Authors: []string{evt1.PubKey}},
			event:       evt3,
			shouldMatch: false,
		},
		{
			name:        "Event 1 matches kind filter",
			filter:      &Filter{Kinds: []int{1}},
			event:       evt1,
			shouldMatch: true,
		},
		{
			name:        "Event 2 matches kind filter",
			filter:      &Filter{Kinds: []int{2}},
			event:       evt2,
			shouldMatch: true,
		},
		{
			name:        "Event 4 matches kind filter",
			filter:      &Filter{Kinds: []int{3}},
			event:       evt4,
			shouldMatch: true,
		},
		{
			name:        "Event 2 doesn't match kind filter",
			filter:      &Filter{Kinds: []int{1}},
			event:       evt2,
			shouldMatch: false,
		},
		{
			name:        "Event 3 matches e tag filter",
			filter:      &Filter{Tags: map[string][]string{"e": {evt1.ID}}},
			event:       evt3,
			shouldMatch: true,
		},
		{
			name:        "Event 1 doesn't match e tag filter",
			filter:      &Filter{Tags: map[string][]string{"e": {evt1.ID}}},
			event:       evt1,
			shouldMatch: false,
		},
		{
			name:        "Event 4 matches p tag filter",
			filter:      &Filter{Tags: map[string][]string{"p": {evt2.PubKey}}},
			event:       evt4,
			shouldMatch: true,
		},
		{
			name:        "Event 1 doesn't match p tag filter",
			filter:      &Filter{Tags: map[string][]string{"p": {evt2.PubKey}}},
			event:       evt1,
			shouldMatch: false,
		},
		{
			name:        "Event 1 matches multiple filter criteria (ID AND Kind)",
			filter:      &Filter{IDs: []string{evt1.ID}, Kinds: []int{1}},
			event:       evt1,
			shouldMatch: true,
		},
		{
			name:        "Event 2 doesn't match multiple filter criteria (ID AND Kind)",
			filter:      &Filter{IDs: []string{evt1.ID}, Kinds: []int{1}},
			event:       evt2,
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.event.Matches(tt.filter)
			assert.Equal(t, tt.shouldMatch, result, "Event matching failed for: %s", tt.name)
		})
	}
}

func TestEventFilterMatchingComplexUnit(t *testing.T) {
	// Create test events manually
	evt1 := &Event{
		ID:        "1111111111111111111111111111111111111111111111111111111111111111",
		PubKey:    "2222222222222222222222222222222222222222222222222222222222222222",
		CreatedAt: 1234567890,
		Kind:      1,
		Content:   "Event 1",
		Tags:      [][]string{},
		Sig:       "signature1",
	}

	evt2 := &Event{
		ID:        "3333333333333333333333333333333333333333333333333333333333333333",
		PubKey:    "2222222222222222222222222222222222222222222222222222222222222222",
		CreatedAt: 1234567891,
		Kind:      2,
		Content:   "Event 2",
		Tags:      [][]string{},
		Sig:       "signature2",
	}

	evt3 := &Event{
		ID:        "4444444444444444444444444444444444444444444444444444444444444444",
		PubKey:    "5555555555555555555555555555555555555555555555555555555555555555",
		CreatedAt: 1234567892,
		Kind:      1,
		Content:   "Event 3",
		Tags:      [][]string{{"e", evt1.ID}},
		Sig:       "signature3",
	}

	evt4 := &Event{
		ID:        "6666666666666666666666666666666666666666666666666666666666666666",
		PubKey:    "5555555555555555555555555555555555555555555555555555555555555555",
		CreatedAt: 1234567893,
		Kind:      3,
		Content:   "Event 4",
		Tags:      [][]string{{"p", evt1.PubKey}},
		Sig:       "signature4",
	}

	allEvents := []*Event{evt1, evt2, evt3, evt4}

	tests := []struct {
		name            string
		filter          *Filter
		expectedMatches int
		matchedIDs      []string
	}{
		{
			name:            "Filter by multiple IDs",
			filter:          &Filter{IDs: []string{evt1.ID, evt3.ID}},
			expectedMatches: 2,
			matchedIDs:      []string{evt1.ID, evt3.ID},
		},
		{
			name:            "Filter by author",
			filter:          &Filter{Authors: []string{evt1.PubKey}},
			expectedMatches: 2,
			matchedIDs:      []string{evt1.ID, evt2.ID},
		},
		{
			name:            "Filter by kinds 1 and 3",
			filter:          &Filter{Kinds: []int{1, 3}},
			expectedMatches: 3,
			matchedIDs:      []string{evt1.ID, evt3.ID, evt4.ID},
		},
		{
			name:            "Filter by e tag",
			filter:          &Filter{Tags: map[string][]string{"e": {evt1.ID}}},
			expectedMatches: 1,
			matchedIDs:      []string{evt3.ID},
		},
		{
			name:            "Filter by p tag",
			filter:          &Filter{Tags: map[string][]string{"p": {evt1.PubKey}}},
			expectedMatches: 1,
			matchedIDs:      []string{evt4.ID},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var matchedEvents []*Event
			for _, evt := range allEvents {
				if evt.Matches(tt.filter) {
					matchedEvents = append(matchedEvents, evt)
				}
			}

			assert.Equal(t, tt.expectedMatches, len(matchedEvents),
				"Expected %d matches, got %d", tt.expectedMatches, len(matchedEvents))

			// Check that all expected IDs are matched
			matchedIDMap := make(map[string]bool)
			for _, evt := range matchedEvents {
				matchedIDMap[evt.ID] = true
			}

			for _, expectedID := range tt.matchedIDs {
				assert.True(t, matchedIDMap[expectedID],
					"Expected event %s to be matched", expectedID)
			}
		})
	}
}
