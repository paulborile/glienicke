package nip40

import (
	"strconv"
	"time"

	"github.com/paul/glienicke/pkg/event"
)

// GetExpiration returns the expiration timestamp from an event's tags.
// Returns 0 if no expiration tag is found or if the tag is invalid.
func GetExpiration(evt *event.Event) time.Time {
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "expiration" {
			timestamp, err := strconv.ParseInt(tag[1], 10, 64)
			if err != nil {
				return time.Time{} // Invalid timestamp
			}
			return time.Unix(timestamp, 0)
		}
	}
	return time.Time{} // No expiration tag
}

// IsExpired checks if an event has expired based on its expiration tag.
func IsExpired(evt *event.Event) bool {
	expiration := GetExpiration(evt)
	if expiration.IsZero() {
		return false // No expiration tag
	}
	return time.Now().After(expiration)
}

// ShouldRejectEvent checks if an event should be rejected because it's already expired.
func ShouldRejectEvent(evt *event.Event) bool {
	return IsExpired(evt)
}

// ShouldFilterEvent checks if an event should be filtered from query results.
func ShouldFilterEvent(evt *event.Event) bool {
	return IsExpired(evt)
}
