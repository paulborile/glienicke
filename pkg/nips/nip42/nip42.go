package nip42

import (
	"fmt"
	"strings"

	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/storage"
)

// Processor handles NIP-42 authentication events
type Processor struct{}

// New creates a new NIP-42 processor
func New() *Processor {
	return &Processor{}
}

// Process handles AUTH events (kind 22242)
func (p *Processor) Process(evt *event.Event, store storage.Store) error {
	// Only process AUTH events
	if evt.Kind != 22242 {
		return fmt.Errorf("not an AUTH event: kind %d", evt.Kind)
	}

	// Validate event signature
	if err := evt.VerifySignature(); err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	// AUTH events are not stored, they're just validated
	// The relay will handle authentication state based on the validated event
	return nil
}

// IsAuthEvent checks if an event is an AUTH event
func IsAuthEvent(evt *event.Event) bool {
	return evt.Kind == 22242
}

// ValidateAuthEvent validates an AUTH event without storing it
func ValidateAuthEvent(evt *event.Event) error {
	if !IsAuthEvent(evt) {
		return fmt.Errorf("event kind %d is not AUTH (22242)", evt.Kind)
	}

	// Check for required challenge in content (if relay sent one)
	// This is a simplified validation - real implementation would
	// check against the challenge sent in AUTH_REQUIRED message
	if strings.TrimSpace(evt.Content) == "" {
		return fmt.Errorf("AUTH event content cannot be empty")
	}

	if err := evt.VerifySignature(); err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	return nil
}
