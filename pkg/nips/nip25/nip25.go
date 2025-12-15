package nip25

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/paul/glienicke/pkg/event"
)

const (
	// KindReaction represents the kind number for reaction events
	KindReaction = 7
)

// ValidateReaction validates that an event is a proper NIP-25 reaction
func ValidateReaction(evt *event.Event) error {
	if evt.Kind != KindReaction {
		return nil // Not a reaction event, no validation needed
	}

	// Check for required e tag (event being reacted to)
	hasETag := false
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			hasETag = true
			// Validate event ID format (should be hex)
			if tag[1] == "" {
				return fmt.Errorf("e tag must have event ID")
			}
			// Basic hex validation for event ID
			if len(tag[1]) != 64 {
				return fmt.Errorf("e tag event ID must be 64 hex characters")
			}
			for _, c := range tag[1] {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
					return fmt.Errorf("e tag event ID contains invalid hex characters")
				}
			}
			break
		}
	}

	if !hasETag {
		return fmt.Errorf("reaction must have at least one e tag pointing to the event being reacted to")
	}

	// Validate reaction content
	if err := validateReactionContent(evt.Content); err != nil {
		return fmt.Errorf("invalid reaction content: %w", err)
	}

	// Validate optional p tag (pubkey of event author)
	// This is recommended but not strictly required
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			// Validate pubkey format (should be 64 hex characters)
			if len(tag[1]) != 64 {
				return fmt.Errorf("p tag pubkey must be 64 hex characters")
			}
			for _, c := range tag[1] {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
					return fmt.Errorf("p tag pubkey contains invalid hex characters")
				}
			}
			break
		}
	}

	// Validate optional k tag (kind of reacted event)
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "k" {
			// Should be a stringified number
			if _, err := strconv.Atoi(tag[1]); err != nil {
				return fmt.Errorf("k tag must be a stringified number")
			}
			break
		}
	}

	return nil
}

// validateReactionContent validates the reaction content according to NIP-25
func validateReactionContent(content string) error {
	// Empty content is interpreted as "+"
	if strings.TrimSpace(content) == "" {
		return nil // Valid (interpreted as like)
	}

	// Single character reactions
	if len(content) == 1 {
		switch content {
		case "+", "-":
			return nil // Valid like/dislike
		default:
			// Could be an emoji, which is valid
			return nil
		}
	}

	// Multi-character content
	if strings.HasPrefix(content, ":") {
		// Custom emoji shortcode format - be more permissive
		parts := strings.Split(content, ":")
		if len(parts) >= 3 && parts[0] == "" && parts[len(parts)-1] == "" {
			// Basic validation - just ensure there's content between the colons
			shortcode := strings.Join(parts[1:len(parts)-1], ":")
			if strings.TrimSpace(shortcode) == "" {
				return fmt.Errorf("emoji shortcode cannot be empty")
			}
			return nil
		}
		return fmt.Errorf("invalid emoji shortcode format, expected :shortcode:")
	}

	// For other content, allow it (could be emoji or other valid reaction)
	return nil
}

// IsReactionEvent checks if an event is a reaction event
func IsReactionEvent(evt *event.Event) bool {
	return evt.Kind == KindReaction
}

// GetReactedEventIDs extracts the event IDs that this reaction is reacting to
func GetReactedEventIDs(evt *event.Event) []string {
	if !IsReactionEvent(evt) {
		return nil
	}

	eventIDs := make([]string, 0)
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			eventIDs = append(eventIDs, tag[1])
		}
	}

	return eventIDs
}

// GetReactedEventAuthors extracts the pubkeys of authors whose events are being reacted to
func GetReactedEventAuthors(evt *event.Event) []string {
	if !IsReactionEvent(evt) {
		return nil
	}

	pubkeys := make([]string, 0)
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			pubkeys = append(pubkeys, tag[1])
		}
	}

	return pubkeys
}

// GetReactionType extracts the type of reaction (+, -, emoji, etc.)
func GetReactionType(evt *event.Event) string {
	if !IsReactionEvent(evt) {
		return ""
	}

	content := strings.TrimSpace(evt.Content)
	if content == "" {
		return "+" // Empty content is interpreted as like
	}
	return content
}

// IsLikeReaction checks if the reaction is a "like" (+ or empty)
func IsLikeReaction(evt *event.Event) bool {
	if !IsReactionEvent(evt) {
		return false
	}

	content := strings.TrimSpace(evt.Content)
	return content == "" || content == "+"
}

// IsDislikeReaction checks if the reaction is a "dislike" (-)
func IsDislikeReaction(evt *event.Event) bool {
	if !IsReactionEvent(evt) {
		return false
	}

	return strings.TrimSpace(evt.Content) == "-"
}

// IsEmojiReaction checks if the reaction is an emoji or custom emoji
func IsEmojiReaction(evt *event.Event) bool {
	if !IsReactionEvent(evt) {
		return false
	}

	content := strings.TrimSpace(evt.Content)
	if content == "" || content == "+" || content == "-" {
		return false
	}

	// Check if it's an emoji shortcode
	if strings.HasPrefix(content, ":shortcode:") {
		return true
	}

	// For simplicity, consider any other content as emoji reaction
	// In a real implementation, you might want more sophisticated emoji detection
	return true
}

// GetReactedEventKind extracts the kind of the event being reacted to
func GetReactedEventKind(evt *event.Event) (int, error) {
	if !IsReactionEvent(evt) {
		return 0, fmt.Errorf("event is not a reaction")
	}

	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "k" {
			kind, err := strconv.Atoi(tag[1])
			if err != nil {
				return 0, fmt.Errorf("invalid k tag value: %w", err)
			}
			return kind, nil
		}
	}

	return 0, fmt.Errorf("k tag not found")
}
