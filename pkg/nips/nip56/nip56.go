package nip56

import (
	"fmt"

	"github.com/paul/glienicke/pkg/event"
)

// Report types defined in NIP-56
const (
	ReportTypeNudity        = "nudity"
	ReportTypeMalware       = "malware"
	ReportTypeProfanity     = "profanity"
	ReportTypeIllegal       = "illegal"
	ReportTypeSpam          = "spam"
	ReportTypeImpersonation = "impersonation"
	ReportTypeOther         = "other"
)

// Valid report types map for validation
var validReportTypes = map[string]bool{
	ReportTypeNudity:        true,
	ReportTypeMalware:       true,
	ReportTypeProfanity:     true,
	ReportTypeIllegal:       true,
	ReportTypeSpam:          true,
	ReportTypeImpersonation: true,
	ReportTypeOther:         true,
}

// BlobReport represents a blob being reported
type BlobReport struct {
	Hash    string // info hash of the blob
	Type    string // report type
	Server  string // optional server where blob can be found
	EventID string // ID of event containing the blob
}

// IsReportEvent checks if an event is a report event (kind 1984)
func IsReportEvent(evt *event.Event) bool {
	return evt.Kind == 1984
}

// ValidateReportEvent validates a NIP-56 report event
func ValidateReportEvent(evt *event.Event) error {
	if !IsReportEvent(evt) {
		return fmt.Errorf("event kind %d is not a report (1984)", evt.Kind)
	}

	// Check for required p tag (reported pubkey)
	reportedPubkey := ""
	hasPTag := false

	for _, tag := range evt.Tags {
		if len(tag) < 2 {
			continue
		}

		switch tag[0] {
		case "p":
			if reportedPubkey == "" {
				reportedPubkey = tag[1]
			}
			hasPTag = true

			// Validate report type if present (3rd element)
			if len(tag) >= 3 && tag[2] != "" {
				if !validReportTypes[tag[2]] {
					return fmt.Errorf("invalid report type: %s", tag[2])
				}
			}

		case "e":
			// Validate report type if present (3rd element)
			if len(tag) >= 3 && tag[2] != "" {
				if !validReportTypes[tag[2]] {
					return fmt.Errorf("invalid report type: %s", tag[2])
				}
			}

		case "x":
			// Validate report type if present (3rd element)
			if len(tag) >= 3 && tag[2] != "" {
				if !validReportTypes[tag[2]] {
					return fmt.Errorf("invalid report type: %s", tag[2])
				}
			}
		}
	}

	if !hasPTag {
		return fmt.Errorf("missing required 'p' tag for reported pubkey")
	}

	if reportedPubkey == "" {
		return fmt.Errorf("p tag cannot have empty pubkey")
	}

	// If we have an e tag, we should also have a p tag (which we've already validated)
	// This ensures reports about notes also reference the note author

	return nil
}

// GetReportedPubKey returns the pubkey being reported
func GetReportedPubKey(evt *event.Event) string {
	if !IsReportEvent(evt) {
		return ""
	}

	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			return tag[1]
		}
	}

	return ""
}

// GetReportedEventIDs returns the event IDs being reported
func GetReportedEventIDs(evt *event.Event) []string {
	if !IsReportEvent(evt) {
		return nil
	}

	var eventIDs []string
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			eventIDs = append(eventIDs, tag[1])
		}
	}

	return eventIDs
}

// GetReportedBlobs returns information about blobs being reported
func GetReportedBlobs(evt *event.Event) []BlobReport {
	if !IsReportEvent(evt) {
		return nil
	}

	var blobs []BlobReport
	var currentBlob BlobReport

	for _, tag := range evt.Tags {
		switch {
		case len(tag) >= 2 && tag[0] == "x":
			// Start a new blob report
			if currentBlob.Hash != "" && currentBlob.Hash != tag[1] {
				// Save previous blob if it exists and is different
				blobs = append(blobs, currentBlob)
			}
			currentBlob = BlobReport{Hash: tag[1]}
			if len(tag) >= 3 {
				currentBlob.Type = tag[2]
			}

		case len(tag) >= 2 && tag[0] == "e" && currentBlob.Hash != "":
			// Associate this event with the current blob
			currentBlob.EventID = tag[1]
			if len(tag) >= 3 && currentBlob.Type == "" {
				currentBlob.Type = tag[2]
			}

		case len(tag) >= 2 && tag[0] == "server" && currentBlob.Hash != "":
			// Add server info to current blob
			currentBlob.Server = tag[1]
		}
	}

	// Add the last blob if it exists
	if currentBlob.Hash != "" {
		blobs = append(blobs, currentBlob)
	}

	return blobs
}

// IsValidReportType checks if a report type is valid according to NIP-56
func IsValidReportType(reportType string) bool {
	return validReportTypes[reportType]
}

// GetReportTypes returns all valid report types
func GetReportTypes() []string {
	return []string{
		ReportTypeNudity,
		ReportTypeMalware,
		ReportTypeProfanity,
		ReportTypeIllegal,
		ReportTypeSpam,
		ReportTypeImpersonation,
		ReportTypeOther,
	}
}
