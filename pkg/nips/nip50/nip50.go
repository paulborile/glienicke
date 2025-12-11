package nip50

import (
	"context"
	"fmt"
	"strings"

	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/storage"
)

// SearchQuery represents a parsed search query
type SearchQuery struct {
	Terms      []string          // Basic search terms (AND logic)
	Exclusions []string          // Terms to exclude (NOT logic)
	Extensions map[string]string // key:value extensions
}

// ParseSearchQuery parses a search query string into components
func ParseSearchQuery(query string) *SearchQuery {
	if query == "" {
		return &SearchQuery{}
	}

	// Remove extra whitespace and split by spaces
	words := strings.Fields(strings.TrimSpace(query))

	sq := &SearchQuery{
		Extensions: make(map[string]string),
	}

	for _, word := range words {
		// Check for key:value extensions (colon separated)
		if strings.Contains(word, ":") && !strings.HasPrefix(word, "-") {
			parts := strings.SplitN(word, ":", 2)
			if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				sq.Extensions[parts[0]] = parts[1]
				continue
			}
		}

		// Check for exclusion terms (prefixed with -)
		if strings.HasPrefix(word, "-") && len(word) > 1 {
			exclusion := strings.TrimPrefix(word, "-")
			if exclusion != "" {
				sq.Exclusions = append(sq.Exclusions, exclusion)
			}
			continue
		}

		// Regular search term
		if word != "" {
			sq.Terms = append(sq.Terms, word)
		}
	}

	return sq
}

// SearchFilter represents a filter with search capabilities
type SearchFilter struct {
	*event.Filter
	Query *SearchQuery
}

// NewSearchFilter creates a new search filter
func NewSearchFilter(filter *event.Filter) *SearchFilter {
	sf := &SearchFilter{
		Filter: filter,
	}

	// Extract search query from filter if present
	if searchValue, ok := getSearchField(filter); ok {
		sf.Query = ParseSearchQuery(searchValue)
	}

	return sf
}

// Matches checks if an event matches the search criteria
func (sf *SearchFilter) Matches(evt *event.Event) bool {
	// First check if event matches the base filter
	if !evt.Matches(sf.Filter) {
		return false
	}

	// If no search query, just use base filter matching
	if sf.Query == nil {
		return true
	}

	// Check search terms (AND logic - all terms must be present)
	for _, term := range sf.Query.Terms {
		if !eventContainsTerm(evt, term) {
			return false
		}
	}

	// Check exclusion terms (NOT logic - none should be present)
	for _, exclusion := range sf.Query.Exclusions {
		if eventContainsTerm(evt, exclusion) {
			return false
		}
	}

	// Check extensions
	for key, value := range sf.Query.Extensions {
		if !eventMatchesExtension(evt, key, value) {
			return false
		}
	}

	return true
}

// eventContainsTerm checks if an event contains a search term
func eventContainsTerm(evt *event.Event, term string) bool {
	term = strings.ToLower(term)

	// Search in content (case-insensitive)
	if strings.Contains(strings.ToLower(evt.Content), term) {
		return true
	}

	// Search in tag values (case-insensitive)
	for _, tag := range evt.Tags {
		if len(tag) >= 2 {
			if strings.Contains(strings.ToLower(tag[1]), term) {
				return true
			}
		}
	}

	return false
}

// eventMatchesExtension checks if an event matches a search extension
func eventMatchesExtension(evt *event.Event, key, value string) bool {
	key = strings.ToLower(key)
	value = strings.ToLower(value)

	switch key {
	case "domain":
		// Check NIP-05 domain (if present in tags)
		for _, tag := range evt.Tags {
			if len(tag) >= 2 && tag[0] == "nip05" {
				if strings.HasSuffix(strings.ToLower(tag[1]), "@"+value) {
					return true
				}
			}
		}
		return false

	case "language":
		// Check language tag
		for _, tag := range evt.Tags {
			if len(tag) >= 2 && tag[0] == "language" {
				if strings.EqualFold(tag[1], value) {
					return true
				}
			}
		}
		return false

	case "nsfw":
		// Check content warning tags
		isNSFW := false
		for _, tag := range evt.Tags {
			if len(tag) >= 2 && tag[0] == "content-warning" {
				isNSFW = true
				break
			}
		}

		// Parse nsfw value (true/false/default)
		switch value {
		case "true":
			return isNSFW
		case "false":
			return !isNSFW
		default:
			return true // default behavior
		}

	case "sentiment":
		// This would require sentiment analysis - for now, ignore
		return true

	default:
		// For unknown extensions, ignore them (as per NIP-50)
		return true
	}
}

// SearchEvents performs a search query against the storage
func SearchEvents(ctx context.Context, store storage.Store, filters []*event.Filter) ([]*event.Event, error) {
	// Convert regular filters to search filters
	searchFilters := make([]*SearchFilter, len(filters))
	for i, filter := range filters {
		searchFilters[i] = NewSearchFilter(filter)
	}

	// Get all events matching the base filters (without search)
	events, err := store.QueryEvents(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	// Filter events based on search criteria
	var matchedEvents []*event.Event
	for _, evt := range events {
		// Check if event matches any of the search filters
		for _, sf := range searchFilters {
			if sf.Matches(evt) {
				matchedEvents = append(matchedEvents, evt)
				break // Don't add duplicate events
			}
		}
	}

	return matchedEvents, nil
}

// getSearchField extracts the search field from a filter
func getSearchField(filter *event.Filter) (string, bool) {
	if filter.Search != "" {
		return filter.Search, true
	}
	return "", false
}
