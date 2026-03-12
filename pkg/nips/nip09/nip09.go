package nip09

import (
	"context"
	"log"

	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/storage"
)

const (
	KindChannelCreate   = 40
	KindChannelMetadata = 41
)

// HandleDeletion handles a NIP-09 event deletion request.
func HandleDeletion(ctx context.Context, store storage.Store, evt *event.Event) error {
	if evt.Kind != 5 {
		return nil // Not a deletion event
	}

	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			eventID := tag[1]

			// First, try to get the event to check if it's a channel event
			// We need to check this BEFORE deleting because GetEvent won't work after deletion
			deletedEvt, getErr := store.GetEvent(ctx, eventID)
			if getErr == nil && deletedEvt != nil {
				// Check if this is a channel create or metadata event
				if deletedEvt.Kind == KindChannelCreate || deletedEvt.Kind == KindChannelMetadata {
					channelID := getChannelIDFromEvent(deletedEvt)
					if channelID != "" {
						// Delete all channel events for this channel
						deletedCount, err := store.DeleteChannelEvents(ctx, channelID)
						if err != nil {
							log.Printf("Failed to delete channel events for channel %s: %v", channelID, err)
						} else if deletedCount > 0 {
							log.Printf("Deleted %d channel events for channel %s", deletedCount, channelID)
						}
					}
				}
			}

			// Now delete the event from the main events table
			if err := store.DeleteEvent(ctx, eventID, evt.PubKey); err != nil {
				log.Printf("Failed to delete event %s: %v", eventID, err)
			}
		}
	}
	return nil
}

func getChannelIDFromEvent(evt *event.Event) string {
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "channel_id" {
			return tag[1]
		}
	}
	return ""
}
