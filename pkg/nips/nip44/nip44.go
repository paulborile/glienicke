package nip44

import (
	"github.com/paul/glienicke/pkg/event"
)

// IsEncryptedDirectMessage checks if an event is a NIP-44 encrypted direct message.
func IsEncryptedDirectMessage(evt *event.Event) bool {
	return evt.Kind == 4
}

// GetRecipientPubKey extracts the recipient's public key from a NIP-44 event's tags.
func GetRecipientPubKey(evt *event.Event) (string, bool) {
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			return tag[1], true
		}
	}
	return "", false
}
