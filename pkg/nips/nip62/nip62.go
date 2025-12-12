package nip62

import (
	"context"
	"fmt"
	"strings"

	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/storage"
)

const (
	// KindRequestToVanish represents the kind number for Request to Vanish events
	KindRequestToVanish = 62
)

// IsRequestToVanishEvent checks if an event is a Request to Vanish event
func IsRequestToVanishEvent(evt *event.Event) bool {
	return evt.Kind == KindRequestToVanish
}

// ValidateRequestToVanish validates that an event is a proper NIP-62 Request to Vanish
func ValidateRequestToVanish(evt *event.Event) error {
	if evt.Kind != KindRequestToVanish {
		return nil // Not a Request to Vanish event, no validation needed
	}

	// Must have at least one relay tag
	hasRelayTag := false
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "relay" {
			hasRelayTag = true
			// Validate relay URL is not empty
			relayURL := strings.TrimSpace(tag[1])
			if relayURL == "" {
				return fmt.Errorf("relay tag value cannot be empty")
			}
			break
		}
	}

	if !hasRelayTag {
		return fmt.Errorf("Request to Vanish (kind 62) must include at least one relay tag")
	}

	return nil
}

// HandleRequestToVanish handles NIP-62 Request to Vanish events
func HandleRequestToVanish(ctx context.Context, store storage.Store, evt *event.Event, relayURL string) error {
	if evt.Kind != KindRequestToVanish {
		return nil // Not a Request to Vanish event
	}

	// Validate the Request to Vanish event
	if err := ValidateRequestToVanish(evt); err != nil {
		return fmt.Errorf("invalid Request to Vanish event: %w", err)
	}

	// Check if this request is for this relay
	isForThisRelay := false
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "relay" {
			relayTag := strings.TrimSpace(tag[1])
			if relayTag == "ALL_RELAYS" || strings.EqualFold(relayTag, relayURL) {
				isForThisRelay = true
				break
			}
		}
	}

	if !isForThisRelay {
		return nil // This request is not for this relay
	}

	// Delete all events from the pubkey
	if err := store.DeleteAllEventsByPubKey(ctx, evt.PubKey); err != nil {
		return fmt.Errorf("failed to delete events for pubkey %s: %w", evt.PubKey, err)
	}

	return nil
}

// IsGlobalRequest checks if the Request to Vanish event is for all relays
func IsGlobalRequest(evt *event.Event) bool {
	if !IsRequestToVanishEvent(evt) {
		return false
	}

	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "relay" && tag[1] == "ALL_RELAYS" {
			return true
		}
	}

	return false
}

// GetRelayTags extracts all relay URLs from a Request to Vanish event
func GetRelayTags(evt *event.Event) []string {
	var relays []string

	if !IsRequestToVanishEvent(evt) {
		return relays
	}

	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "relay" {
			relayURL := strings.TrimSpace(tag[1])
			if relayURL != "" {
				relays = append(relays, relayURL)
			}
		}
	}

	return relays
}
