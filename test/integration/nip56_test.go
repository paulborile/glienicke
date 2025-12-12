package integration

import (
	"context"
	"testing"
	"time"

	"github.com/paul/glienicke/internal/store/memory"
	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/nips/nip56"
	"github.com/stretchr/testify/require"
)

func TestNIP56_Reporting(t *testing.T) {
	ctx := context.Background()

	// Create test store
	store := memory.New()

	// Create test keys
	reporterKey := testutil.MustGenerateKeyPair()
	reportedKey := testutil.MustGenerateKeyPair()

	// Test 1: Create and validate a valid profile report
	t.Run("Valid Profile Report", func(t *testing.T) {
		reportEvent := &event.Event{
			Kind:      1984,
			CreatedAt: time.Now().Unix(),
			Tags: [][]string{
				{"p", reportedKey.PubKeyHex, "impersonation"},
				{"L", "social.nos.ontology"},
				{"l", "NS-imp", "social.nos.ontology"},
			},
			Content: "Profile is impersonating nostr:" + reportedKey.PubKeyHex,
		}

		// Sign the event
		require.NoError(t, reporterKey.SignEvent(reportEvent))

		// Validate it's a proper report event
		err := nip56.ValidateReportEvent(reportEvent)
		require.NoError(t, err)

		// Store the event
		require.NoError(t, store.SaveEvent(ctx, reportEvent))

		// Verify we can retrieve it
		filter := &event.Filter{
			Kinds:   []int{1984},
			Authors: []string{reporterKey.PubKeyHex},
		}

		events, err := store.QueryEvents(ctx, []*event.Filter{filter})
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, reportEvent.ID, events[0].ID)
	})

	// Test 2: Create and validate a valid note report
	t.Run("Valid Note Report", func(t *testing.T) {
		noteToReport := &event.Event{
			Kind:      1,
			CreatedAt: time.Now().Unix() - 3600,
			Tags:      [][]string{},
			Content:   "This is a note that will be reported",
		}
		require.NoError(t, reportedKey.SignEvent(noteToReport))

		reportEvent := &event.Event{
			Kind:      1984,
			CreatedAt: time.Now().Unix(),
			Tags: [][]string{
				{"e", noteToReport.ID, "spam"},
				{"p", reportedKey.PubKeyHex},
			},
			Content: "This note is spam",
		}

		require.NoError(t, reporterKey.SignEvent(reportEvent))

		// Validate report event
		err := nip56.ValidateReportEvent(reportEvent)
		require.NoError(t, err)

		// Store both events
		require.NoError(t, store.SaveEvent(ctx, noteToReport))
		require.NoError(t, store.SaveEvent(ctx, reportEvent))
	})

	// Test 3: Test invalid report events
	t.Run("Invalid Report Events", func(t *testing.T) {
		testCases := []struct {
			name    string
			event   *event.Event
			wantErr string
		}{
			{
				name: "Missing required p tag",
				event: &event.Event{
					Kind:      1984,
					CreatedAt: time.Now().Unix(),
					Tags:      [][]string{},
					Content:   "No p tag",
				},
				wantErr: "missing required 'p' tag",
			},
			{
				name: "Invalid report type",
				event: &event.Event{
					Kind:      1984,
					CreatedAt: time.Now().Unix(),
					Tags: [][]string{
						{"p", reportedKey.PubKeyHex, "invalid_type"},
					},
					Content: "Invalid report type",
				},
				wantErr: "invalid report type",
			},
			{
				name: "e tag without p tag",
				event: &event.Event{
					Kind:      1984,
					CreatedAt: time.Now().Unix(),
					Tags: [][]string{
						{"e", "event_id", "spam"},
					},
					Content: "e tag without p tag",
				},
				wantErr: "missing required 'p' tag",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				require.NoError(t, reporterKey.SignEvent(tc.event))

				err := nip56.ValidateReportEvent(tc.event)
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
			})
		}
	})

	// Test 4: Test blob reporting
	t.Run("Blob Report", func(t *testing.T) {
		blobHash := "sha256:abc123..."
		containingEvent := &event.Event{
			Kind:      1,
			CreatedAt: time.Now().Unix() - 1800,
			Tags: [][]string{
				{"x", blobHash},
			},
			Content: "Note containing a blob",
		}
		require.NoError(t, reportedKey.SignEvent(containingEvent))

		reportEvent := &event.Event{
			Kind:      1984,
			CreatedAt: time.Now().Unix(),
			Tags: [][]string{
				{"p", reportedKey.PubKeyHex},
				{"x", blobHash, "malware"},
				{"e", containingEvent.ID, "malware"},
				{"server", "https://example.com/blob"},
			},
			Content: "This blob contains malware",
		}

		require.NoError(t, reporterKey.SignEvent(reportEvent))

		// Validate report event
		err := nip56.ValidateReportEvent(reportEvent)
		require.NoError(t, err)

		// Store events
		require.NoError(t, store.SaveEvent(ctx, containingEvent))
		require.NoError(t, store.SaveEvent(ctx, reportEvent))
	})

	// Test 5: Test helper functions
	t.Run("Helper Functions", func(t *testing.T) {
		reportEvent := &event.Event{
			Kind:      1984,
			CreatedAt: time.Now().Unix(),
			Tags: [][]string{
				{"p", reportedKey.PubKeyHex, "impersonation"},
				{"e", "event123", "spam"},
				{"x", "blob456", "malware"},
			},
			Content: "Multiple report tags",
		}

		// Test IsReportEvent
		require.True(t, nip56.IsReportEvent(reportEvent))

		regularEvent := &event.Event{
			Kind:      1,
			CreatedAt: time.Now().Unix(),
			Tags:      [][]string{},
			Content:   "Regular note",
		}
		require.False(t, nip56.IsReportEvent(regularEvent))

		// Test GetReportedPubKey
		pubkey := nip56.GetReportedPubKey(reportEvent)
		require.Equal(t, reportedKey.PubKeyHex, pubkey)

		// Test GetReportedEventIDs
		eventIDs := nip56.GetReportedEventIDs(reportEvent)
		require.Len(t, eventIDs, 1)
		require.Equal(t, "event123", eventIDs[0])

		// Test GetReportedBlobs
		blobs := nip56.GetReportedBlobs(reportEvent)
		require.Len(t, blobs, 1)
		require.Equal(t, "blob456", blobs[0].Hash)
		require.Equal(t, "malware", blobs[0].Type)
	})
}
