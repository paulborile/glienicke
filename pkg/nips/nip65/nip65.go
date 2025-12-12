package nip65

import (
	"context"
	"fmt"
	"strings"

	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/storage"
)

const (
	// KindRelayList represents the kind number for relay list metadata events
	KindRelayList = 10002
)

// RelayInfo contains information about a relay from a relay list event
type RelayInfo struct {
	URL    string // The relay URL
	Read   bool   // Whether this relay is used for reading
	Write  bool   // Whether this relay is used for writing
	Marker string // The original marker (read/write/empty/invalid)
}

// ValidateRelayList validates that an event is a proper NIP-65 relay list
func ValidateRelayList(evt *event.Event) error {
	if evt.Kind != KindRelayList {
		return nil // Not a relay list event, no validation needed
	}

	// Relay list events must have empty content
	if evt.Content != "" {
		return fmt.Errorf("relay list (kind 10002) must have empty content")
	}

	// Count and validate r tags
	rTagCount := 0
	for _, tag := range evt.Tags {
		if len(tag) >= 1 && tag[0] == "r" {
			rTagCount++

			// r tag must have at least 2 elements (tag name and relay URL)
			if len(tag) < 2 {
				return fmt.Errorf("r tag must have at least 2 elements (tag name and relay URL)")
			}

			// Validate relay URL is not empty
			relayURL := strings.TrimSpace(tag[1])
			if relayURL == "" {
				return fmt.Errorf("r tag relay URL cannot be empty")
			}

			// Optional: Basic URL validation (should start with ws:// or wss://)
			if !strings.HasPrefix(relayURL, "ws://") && !strings.HasPrefix(relayURL, "wss://") {
				// This is not strictly required by the NIP but is a reasonable validation
				return fmt.Errorf("r tag relay URL should start with ws:// or wss://, got: %s", relayURL)
			}

			// Validate marker if present (third element)
			if len(tag) >= 3 {
				marker := strings.ToLower(tag[2])
				if marker != "read" && marker != "write" {
					// Allow unknown markers but note them as invalid per spec
					// The NIP suggests only "read" and "write" are valid
					// We'll accept unknown markers for now but this could be stricter
				}
			}
		}
	}

	// Relay list events should have at least one r tag
	// This is not strictly required by the NIP but is reasonable
	if rTagCount == 0 {
		return fmt.Errorf("relay list (kind 10002) should have at least one r tag")
	}

	return nil
}

// HandleRelayList handles NIP-65 relay list events
func HandleRelayList(ctx context.Context, store storage.Store, evt *event.Event) error {
	if evt.Kind != KindRelayList {
		return nil // Not a relay list event
	}

	// Validate the relay list event
	if err := ValidateRelayList(evt); err != nil {
		return fmt.Errorf("invalid relay list event: %w", err)
	}

	// For replaceable events like relay lists, we need to ensure only the latest
	// event for each (author, kind) pair is stored
	// The storage layer should handle this, but we can help by checking for existing events

	// Store the event - the storage layer should handle replaceable logic
	return nil
}

// ExtractRelayInfo extracts detailed relay information from a relay list event
func ExtractRelayInfo(evt *event.Event) []RelayInfo {
	relays := make([]RelayInfo, 0)

	if evt.Kind != KindRelayList {
		return relays
	}

	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "r" {
			relayURL := strings.TrimSpace(tag[1])

			relayInfo := RelayInfo{
				URL:    relayURL,
				Read:   true, // Default: read relay
				Write:  true, // Default: write relay
				Marker: "",   // Default: no marker
			}

			// Check for marker (third element)
			if len(tag) >= 3 {
				marker := strings.ToLower(tag[2])
				relayInfo.Marker = marker

				switch marker {
				case "read":
					relayInfo.Write = false
				case "write":
					relayInfo.Read = false
				default:
					// Unknown marker - keep as read+write but preserve the marker
				}
			}

			relays = append(relays, relayInfo)
		}
	}

	return relays
}

// ExtractReadRelays extracts the list of read relays from a relay list event
func ExtractReadRelays(evt *event.Event) []string {
	if evt.Kind != KindRelayList {
		return nil
	}

	var relays []string
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "r" {
			// Include if: no marker (default read+write), marker is "read", or unknown marker (treated as read+write)
			// Only exclude if marker is explicitly "write"
			if len(tag) == 2 || strings.ToLower(tag[2]) != "write" {
				relays = append(relays, strings.TrimSpace(tag[1]))
			}
		}
	}

	return relays
}

// ExtractWriteRelays extracts the list of write relays from a relay list event
func ExtractWriteRelays(evt *event.Event) []string {
	if evt.Kind != KindRelayList {
		return nil
	}

	var relays []string
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "r" {
			// Include if: no marker (default read+write), marker is "write", or unknown marker (treated as read+write)
			// Only exclude if marker is explicitly "read"
			if len(tag) == 2 || strings.ToLower(tag[2]) != "read" {
				relays = append(relays, strings.TrimSpace(tag[1]))
			}
		}
	}

	return relays
}

// ExtractAllRelays extracts all relay URLs from a relay list event
func ExtractAllRelays(evt *event.Event) []string {
	if evt.Kind != KindRelayList {
		return nil
	}

	var relays []string
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "r" {
			relays = append(relays, strings.TrimSpace(tag[1]))
		}
	}

	return relays
}

// IsRelayListEvent checks if an event is a relay list event
func IsRelayListEvent(evt *event.Event) bool {
	return evt.Kind == KindRelayList
}
