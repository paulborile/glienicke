package nip17

import (
	"encoding/json"

	"github.com/paul/glienicke/pkg/event"
)

const (
	// PrivateDirectMessageKind represents NIP-17 private direct message rumors
	PrivateDirectMessageKind = 14
	// FileMessageKind represents NIP-17 file message rumors
	FileMessageKind = 15
)

// IsPrivateDirectMessage checks if an event is a NIP-17 private direct message
func IsPrivateDirectMessage(evt *event.Event) bool {
	return evt.Kind == PrivateDirectMessageKind
}

// IsFileMessage checks if an event is a NIP-17 file message
func IsFileMessage(evt *event.Event) bool {
	return evt.Kind == FileMessageKind
}

// GetRecipients extracts all recipient public keys from NIP-17 event tags
func GetRecipients(evt *event.Event) []string {
	var recipients []string
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			recipients = append(recipients, tag[1])
		}
	}
	if recipients == nil {
		return []string{}
	}
	return recipients
}

// GetReplyTo extracts the parent event ID if this is a reply
func GetReplyTo(evt *event.Event) (string, bool) {
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			return tag[1], true
		}
	}
	return "", false
}

// GetSubject extracts the subject from NIP-17 event tags
func GetSubject(evt *event.Event) (string, bool) {
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "subject" {
			return tag[1], true
		}
	}
	return "", false
}

// ValidateRumor validates a NIP-17 rumor event (should be unsigned)
func ValidateRumor(evt *event.Event) error {
	// Check kind
	if evt.Kind != PrivateDirectMessageKind && evt.Kind != FileMessageKind {
		return &ValidationError{Kind: "invalid_kind", Message: "invalid event kind for NIP-17"}
	}

	// Must have at least one recipient
	recipients := GetRecipients(evt)
	if len(recipients) == 0 {
		return &ValidationError{Kind: "missing_recipient", Message: "NIP-17 event must have at least one 'p' tag"}
	}

	// Content should not be empty for private messages
	if evt.Kind == PrivateDirectMessageKind && evt.Content == "" {
		return &ValidationError{Kind: "empty_content", Message: "NIP-17 private message content cannot be empty"}
	}

	// Event should not be signed (rumors are unsigned)
	if evt.Sig != "" {
		return &ValidationError{Kind: "signed_rumor", Message: "NIP-17 rumors must not be signed"}
	}

	return nil
}

// CreatePrivateDirectMessage creates a new NIP-17 private message rumor
func CreatePrivateDirectMessage(senderPubKey, content string, recipients []string, replyTo *string) *event.Event {
	evt := &event.Event{
		PubKey:  senderPubKey,
		Kind:    PrivateDirectMessageKind,
		Content: content,
		Tags:    make([][]string, 0),
	}

	// Add recipient tags
	for _, recipient := range recipients {
		evt.Tags = append(evt.Tags, []string{"p", recipient})
	}

	// Add reply tag if specified
	if replyTo != nil {
		evt.Tags = append(evt.Tags, []string{"e", *replyTo})
	}

	return evt
}

// CreateFileMessage creates a new NIP-17 file message rumor
func CreateFileMessage(senderPubKey, content, fileName, mimeType string, recipients []string) *event.Event {
	evt := &event.Event{
		PubKey:  senderPubKey,
		Kind:    FileMessageKind,
		Content: content,
		Tags:    make([][]string, 0),
	}

	// Add recipient tags
	for _, recipient := range recipients {
		evt.Tags = append(evt.Tags, []string{"p", recipient})
	}

	// Add file metadata tags
	if fileName != "" {
		evt.Tags = append(evt.Tags, []string{"filename", fileName})
	}
	if mimeType != "" {
		evt.Tags = append(evt.Tags, []string{"mimetype", mimeType})
	}

	return evt
}

// ValidationError represents a NIP-17 validation error
type ValidationError struct {
	Kind    string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// GiftWrapData represents the structure for NIP-59 gift wrapping
type GiftWrapData struct {
	Rumor     *event.Event `json:"rumor"`
	Recipient string       `json:"recipient"`
}

// SealData represents the structure for NIP-59 sealing
type SealData struct {
	EncryptedRumor string `json:"encrypted_rumor"`
}

// ExportRumor exports a rumor to JSON for gift wrapping
func ExportRumor(rumor *event.Event) ([]byte, error) {
	// Ensure rumor is properly validated before export
	if err := ValidateRumor(rumor); err != nil {
		return nil, err
	}

	return json.Marshal(rumor)
}

// ImportRumor imports a rumor from JSON
func ImportRumor(data []byte) (*event.Event, error) {
	var rumor event.Event
	if err := json.Unmarshal(data, &rumor); err != nil {
		return nil, &ValidationError{Kind: "invalid_json", Message: "failed to unmarshal rumor: " + err.Error()}
	}

	if err := ValidateRumor(&rumor); err != nil {
		return nil, err
	}

	return &rumor, nil
}
