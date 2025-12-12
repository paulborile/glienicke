package nip02

import (
	"context"
	"fmt"

	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/storage"
)

const (
	// KindFollowList represents the kind number for follow list events
	KindFollowList = 3
)

// ValidateFollowList validates that an event is a proper NIP-02 follow list
func ValidateFollowList(evt *event.Event) error {
	if evt.Kind != KindFollowList {
		return nil // Not a follow list event, no validation needed
	}

	// Follow list events must have empty content
	if evt.Content != "" {
		return fmt.Errorf("follow list (kind 3) must have empty content")
	}

	// Count and validate p tags
	pTagCount := 0
	for _, tag := range evt.Tags {
		if len(tag) >= 1 && tag[0] == "p" {
			pTagCount++
			if len(tag) < 2 {
				return fmt.Errorf("p tag must have at least 2 elements (tag name and pubkey)")
			}

			// Validate pubkey format (should be 64 hex characters)
			pubkey := tag[1]
			if len(pubkey) != 64 {
				return fmt.Errorf("p tag pubkey must be 64 hex characters, got %d characters", len(pubkey))
			}

			// Validate that pubkey is valid hex
			for _, c := range pubkey {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
					return fmt.Errorf("p tag pubkey contains invalid hex characters: %s", pubkey)
				}
			}
		}
	}

	// Follow list events must have at least one p tag (even if empty)
	if pTagCount == 0 {
		return fmt.Errorf("follow list (kind 3) must have at least one p tag")
	}

	return nil
}

// HandleFollowList handles NIP-02 follow list events
func HandleFollowList(ctx context.Context, store storage.Store, evt *event.Event) error {
	if evt.Kind != KindFollowList {
		return nil // Not a follow list event
	}

	// Validate the follow list event
	if err := ValidateFollowList(evt); err != nil {
		return fmt.Errorf("invalid follow list event: %w", err)
	}

	// For replaceable events like follow lists, we need to ensure only the latest
	// event for each (author, kind) pair is stored
	// The storage layer should handle this, but we can help by checking for existing events

	// Store the event - the storage layer should handle replaceable logic
	return nil
}

// ExtractFollowedPubkeys extracts the list of followed pubkeys from a follow list event
func ExtractFollowedPubkeys(evt *event.Event) ([]string, error) {
	if evt.Kind != KindFollowList {
		return nil, fmt.Errorf("event is not a follow list (kind %d)", evt.Kind)
	}

	pubkeys := make([]string, 0)
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			pubkeys = append(pubkeys, tag[1])
		}
	}

	return pubkeys, nil
}

// GetFollowedPubkeyWithDetails extracts detailed follow information from a follow list event
func GetFollowedPubkeyWithDetails(evt *event.Event) []FollowedPubkey {
	followed := make([]FollowedPubkey, 0)

	if evt.Kind != KindFollowList {
		return followed
	}

	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			followedPubkey := FollowedPubkey{
				PubKey: tag[1],
			}

			// Optional relay URL (third element if present)
			if len(tag) >= 3 && tag[2] != "" {
				followedPubkey.RelayURL = tag[2]
			}

			// Optional petname (fourth element if present)
			if len(tag) >= 4 && tag[3] != "" {
				followedPubkey.Petname = tag[3]
			}

			followed = append(followed, followedPubkey)
		}
	}

	return followed
}

// FollowedPubkey represents a followed user with optional metadata
type FollowedPubkey struct {
	PubKey   string // The 32-byte hex public key
	RelayURL string // Optional relay URL where this user can be found
	Petname  string // Optional local name/petname for this user
}

// IsFollowListEvent checks if an event is a follow list event
func IsFollowListEvent(evt *event.Event) bool {
	return evt.Kind == KindFollowList
}
