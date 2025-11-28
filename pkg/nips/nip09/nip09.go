package nip09

import (
	"context"
	"log"

	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/storage"
)

// HandleDeletion handles a NIP-09 event deletion request.
func HandleDeletion(ctx context.Context, store storage.Store, evt *event.Event) error {
	if evt.Kind != 5 {
		return nil // Not a deletion event
	}

	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			eventID := tag[1]
			if err := store.DeleteEvent(ctx, eventID, evt.PubKey); err != nil {
				// Log the error but don't stop processing other deletions
				log.Printf("Failed to delete event %s: %v", eventID, err)
			}
		}
	}
	return nil
}
