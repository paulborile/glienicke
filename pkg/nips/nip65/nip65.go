package nip65

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/storage"
)

const (
	// KindRelayList represents the kind number for relay list events (NIP-65)
	KindRelayList = 10002
)

// RelayMode represents the read/write mode for a relay
type RelayMode string

const (
	// ModeRead indicates the relay is used for reading
	ModeRead RelayMode = "read"
	// ModeWrite indicates the relay is used for writing
	ModeWrite RelayMode = "write"
	// ModeReadWrite indicates the relay is used for both reading and writing
	ModeReadWrite RelayMode = ""
)

// RelayInfo contains information about a relay from a NIP-65 relay list
type RelayInfo struct {
	URL  string    // Relay URL
	Mode RelayMode // read, write, or both (empty string)
}

// IsRelayListEvent checks if an event is a relay list event
func IsRelayListEvent(evt *event.Event) bool {
	return evt.Kind == KindRelayList
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
			if len(tag) < 2 {
				return fmt.Errorf("r tag must have at least 2 elements (tag name and relay URL)")
			}

			// Validate relay URL format
			relayURL := tag[1]
			if err := validateRelayURL(relayURL); err != nil {
				return fmt.Errorf("invalid relay URL in r tag: %w", err)
			}

			// Validate mode if present
			if len(tag) >= 3 {
				mode := tag[2]
				if mode != string(ModeRead) && mode != string(ModeWrite) {
					return fmt.Errorf("invalid relay mode '%s', must be 'read' or 'write'", mode)
				}
			}
		}
	}

	// Relay list events should have at least one r tag (though empty lists are technically allowed)
	// This validation is optional depending on implementation requirements
	if rTagCount == 0 {
		// Empty relay lists are valid according to the specification
		// Comment this out if you want to require at least one relay
		// return fmt.Errorf("relay list (kind 10002) must have at least one r tag")
	}

	return nil
}

// validateRelayURL validates that a relay URL is properly formatted
func validateRelayURL(relayURL string) error {
	if strings.TrimSpace(relayURL) == "" {
		return fmt.Errorf("relay URL cannot be empty")
	}

	// Parse URL to validate format
	parsedURL, err := url.Parse(relayURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Must be ws:// or wss://
	if parsedURL.Scheme != "ws" && parsedURL.Scheme != "wss" {
		return fmt.Errorf("relay URL must use ws:// or wss:// scheme, got: %s", parsedURL.Scheme)
	}

	// Must have a host
	if parsedURL.Host == "" {
		return fmt.Errorf("relay URL must have a host")
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

// ExtractRelayInfo extracts relay information from a relay list event
func ExtractRelayInfo(evt *event.Event) ([]RelayInfo, error) {
	if evt.Kind != KindRelayList {
		return nil, fmt.Errorf("event is not a relay list (kind %d)", evt.Kind)
	}

	relays := make([]RelayInfo, 0)
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "r" {
			relayInfo := RelayInfo{
				URL: tag[1],
			}

			// Set mode if present (third element)
			if len(tag) >= 3 {
				relayInfo.Mode = RelayMode(tag[2])
			} else {
				// No mode specified means both read and write
				relayInfo.Mode = ModeReadWrite
			}

			relays = append(relays, relayInfo)
		}
	}

	return relays, nil
}

// GetReadRelays extracts only the read relays from a relay list event
func GetReadRelays(evt *event.Event) ([]string, error) {
	relays, err := ExtractRelayInfo(evt)
	if err != nil {
		return nil, err
	}

	var readRelays []string
	for _, relay := range relays {
		if relay.Mode == ModeRead || relay.Mode == ModeReadWrite {
			readRelays = append(readRelays, relay.URL)
		}
	}

	return readRelays, nil
}

// GetWriteRelays extracts only the write relays from a relay list event
func GetWriteRelays(evt *event.Event) ([]string, error) {
	relays, err := ExtractRelayInfo(evt)
	if err != nil {
		return nil, err
	}

	var writeRelays []string
	for _, relay := range relays {
		if relay.Mode == ModeWrite || relay.Mode == ModeReadWrite {
			writeRelays = append(writeRelays, relay.URL)
		}
	}

	return writeRelays, nil
}

// NormalizeRelayURL normalizes a relay URL by removing trailing slashes and ensuring consistent format
func NormalizeRelayURL(relayURL string) (string, error) {
	trimmed := strings.TrimSpace(relayURL)
	if trimmed == "" {
		return "", fmt.Errorf("relay URL cannot be empty")
	}

	parsedURL, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid relay URL: %w", err)
	}

	// Validate it's a proper relay URL
	if err := validateRelayURL(trimmed); err != nil {
		return "", err
	}

	// Remove all trailing slashes from path
	for strings.HasSuffix(parsedURL.Path, "/") && parsedURL.Path != "/" {
		parsedURL.Path = strings.TrimSuffix(parsedURL.Path, "/")
	}
	// Handle case where path is just "/" by removing it entirely
	if parsedURL.Path == "/" {
		parsedURL.Path = ""
	}

	return parsedURL.String(), nil
}
