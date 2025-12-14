package nip22

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/paul/glienicke/pkg/event"
)

const (
	// KindComment represents the kind number for comment events
	KindComment = 1111
)

// CommentThreadInfo represents information about a comment's thread structure
type CommentThreadInfo struct {
	RootTags      []string // Uppercase tags (E, A, I, K, P)
	ParentTags    []string // Lowercase tags (e, a, i, k, p)
	IsReply       bool     // Whether this is a reply to another comment
	RootEventID   string   // The root event ID (if applicable)
	ParentEventID string   // The parent event ID (if applicable)
}

// ValidateComment validates that an event is a proper NIP-22 comment
func ValidateComment(evt *event.Event) error {
	if evt.Kind != KindComment {
		return nil // Not a comment event, no validation needed
	}

	// Comments must have plaintext content (no HTML, Markdown, etc.)
	// This is more of a client-side concern, but we can validate it's not empty
	if strings.TrimSpace(evt.Content) == "" {
		return fmt.Errorf("comment (kind 1111) must have non-empty content")
	}

	// Validate required tags
	_, err := ExtractCommentThreadInfo(evt)
	if err != nil {
		return fmt.Errorf("invalid comment thread structure: %w", err)
	}

	// Validate that K and k tags are present
	hasKTag := false
	hasKTagLower := false
	for _, tag := range evt.Tags {
		if len(tag) >= 1 {
			if tag[0] == "K" {
				hasKTag = true
				if len(tag) < 2 || tag[1] == "" {
					return fmt.Errorf("K tag must have a kind value")
				}
			}
			if tag[0] == "k" {
				hasKTagLower = true
				if len(tag) < 2 || tag[1] == "" {
					return fmt.Errorf("k tag must have a kind value")
				}
			}
		}
	}

	if !hasKTag {
		return fmt.Errorf("comment must have K tag specifying root kind")
	}

	if !hasKTagLower {
		return fmt.Errorf("comment must have k tag specifying parent kind")
	}

	// Additional validation for specific tag combinations
	if err := validateTagRelationships(evt); err != nil {
		return err
	}

	// Check if this is a comment on a kind 1 event (which should use NIP-10 instead)
	rootKind, err := GetRootKind(evt)
	if err == nil {
		if parsedKind, parseErr := ParseKind(rootKind); parseErr == nil && parsedKind == 1 {
			return fmt.Errorf("comments must not be used to reply to kind 1 notes, use NIP-10 instead")
		}
	}

	return nil
}

// validateTagRelationships validates the relationships between different tags
func validateTagRelationships(evt *event.Event) error {
	rootTags := make(map[string]string)   // uppercase tag -> value
	parentTags := make(map[string]string) // lowercase tag -> value

	// Extract tag values
	for _, tag := range evt.Tags {
		if len(tag) >= 2 {
			tagName := tag[0]
			tagValue := tag[1]

			if len(tagName) == 1 && strings.ToUpper(tagName) == tagName {
				// Uppercase tag (root scope)
				rootTags[tagName] = tagValue
			} else if len(tagName) == 1 && strings.ToLower(tagName) == tagName {
				// Lowercase tag (parent scope)
				parentTags[tagName] = tagValue
			}
		}
	}

	// If we have both E and e tags, they should be consistent
	if rootE, hasRootE := rootTags["E"]; hasRootE {
		if parentE, hasParentE := parentTags["e"]; hasParentE {
			// For top-level comments, root and parent should be the same
			// For replies, they should be different
			if rootE == parentE {
				// This appears to be a top-level comment, which is fine
			}
		}
	}

	// Similar validation for A/a tags
	if rootA, hasRootA := rootTags["A"]; hasRootA {
		if parentA, hasParentA := parentTags["a"]; hasParentA {
			if rootA == parentA {
				// Top-level comment
			}
		}
	}

	return nil
}

// ExtractCommentThreadInfo extracts thread information from a comment event
func ExtractCommentThreadInfo(evt *event.Event) (*CommentThreadInfo, error) {
	if evt.Kind != KindComment {
		return nil, fmt.Errorf("event is not a comment (kind %d)", evt.Kind)
	}

	info := &CommentThreadInfo{
		RootTags:   make([]string, 0),
		ParentTags: make([]string, 0),
	}

	// Extract root scope tags (uppercase)
	for _, tag := range evt.Tags {
		if len(tag) >= 2 {
			tagName := tag[0]
			if len(tagName) == 1 && strings.ToUpper(tagName) == tagName && tagName != "K" && tagName != "P" {
				info.RootTags = append(info.RootTags, tagName)

				// Extract root event ID for common cases
				if tagName == "E" {
					info.RootEventID = tag[1]
				}
			}
		}
	}

	// Extract parent scope tags (lowercase)
	for _, tag := range evt.Tags {
		if len(tag) >= 2 {
			tagName := tag[0]
			if len(tagName) == 1 && strings.ToLower(tagName) == tagName && tagName != "k" && tagName != "p" {
				info.ParentTags = append(info.ParentTags, tagName)

				// Extract parent event ID for common cases
				if tagName == "e" {
					info.ParentEventID = tag[1]
				}
			}
		}
	}

	// Determine if this is a reply
	// A comment is a reply if it has a different parent event ID than root event ID
	// or if it has parent tags that are different from root tags
	hasDifferentParent := false
	if info.RootEventID != "" && info.ParentEventID != "" && info.RootEventID != info.ParentEventID {
		hasDifferentParent = true
	}

	// Also check if we have both root and parent tags with different values
	if !hasDifferentParent {
		rootTagValues := make(map[string]string)
		parentTagValues := make(map[string]string)

		for _, tag := range evt.Tags {
			if len(tag) >= 2 {
				tagName := tag[0]
				tagValue := tag[1]

				if len(tagName) == 1 && strings.ToUpper(tagName) == tagName && tagName != "K" && tagName != "P" {
					rootTagValues[tagName] = tagValue
				} else if len(tagName) == 1 && strings.ToLower(tagName) == tagName && tagName != "k" && tagName != "p" {
					parentTagValues[tagName] = tagValue
				}
			}
		}

		// Check if any parent tag value differs from corresponding root tag
		for pTag, pValue := range parentTagValues {
			rTag := strings.ToUpper(pTag)
			if rValue, exists := rootTagValues[rTag]; exists && rValue != pValue {
				hasDifferentParent = true
				break
			}
		}
	}

	info.IsReply = hasDifferentParent

	return info, nil
}

// IsCommentEvent checks if an event is a comment event
func IsCommentEvent(evt *event.Event) bool {
	return evt.Kind == KindComment
}

// GetRootKind extracts the root kind from K tag
func GetRootKind(evt *event.Event) (string, error) {
	if !IsCommentEvent(evt) {
		return "", fmt.Errorf("event is not a comment (kind %d)", evt.Kind)
	}

	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "K" {
			return tag[1], nil
		}
	}

	return "", fmt.Errorf("K tag not found")
}

// GetParentKind extracts the parent kind from k tag
func GetParentKind(evt *event.Event) (string, error) {
	if !IsCommentEvent(evt) {
		return "", fmt.Errorf("event is not a comment (kind %d)", evt.Kind)
	}

	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "k" {
			return tag[1], nil
		}
	}

	return "", fmt.Errorf("k tag not found")
}

// GetRootPubkey extracts the root author pubkey from P tag
func GetRootPubkey(evt *event.Event) (string, error) {
	if !IsCommentEvent(evt) {
		return "", fmt.Errorf("event is not a comment (kind %d)", evt.Kind)
	}

	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "P" {
			return tag[1], nil
		}
	}

	return "", fmt.Errorf("P tag not found")
}

// GetParentPubkey extracts the parent author pubkey from p tag
func GetParentPubkey(evt *event.Event) (string, error) {
	if !IsCommentEvent(evt) {
		return "", fmt.Errorf("event is not a comment (kind %d)", evt.Kind)
	}

	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			return tag[1], nil
		}
	}

	return "", fmt.Errorf("p tag not found")
}

// IsTopLevelComment checks if a comment is a top-level comment (not a reply to another comment)
func IsTopLevelComment(evt *event.Event) (bool, error) {
	threadInfo, err := ExtractCommentThreadInfo(evt)
	if err != nil {
		return false, err
	}

	// A top-level comment has the same root and parent event IDs
	// or has no parent event ID distinct from root
	return !threadInfo.IsReply, nil
}

// ValidateCommentForKind validates that a comment is appropriate for the given root kind
func ValidateCommentForKind(evt *event.Event, rootKind int) error {
	if !IsCommentEvent(evt) {
		return fmt.Errorf("event is not a comment (kind %d)", evt.Kind)
	}

	// Comments MUST NOT be used to reply to kind 1 notes (NIP-10 should be used instead)
	if rootKind == 1 {
		return fmt.Errorf("comments must not be used to reply to kind 1 notes, use NIP-10 instead")
	}

	return nil
}

// ParseKind parses a kind string to integer, handling special cases like "web"
func ParseKind(kindStr string) (int, error) {
	switch strings.ToLower(kindStr) {
	case "web":
		return -1, nil // Special case for web URLs
	default:
		return strconv.Atoi(kindStr)
	}
}
