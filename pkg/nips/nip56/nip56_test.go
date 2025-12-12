package nip56

import (
	"testing"

	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/require"
)

func TestIsReportEvent(t *testing.T) {
	testCases := []struct {
		name     string
		kind     int
		expected bool
	}{
		{"Report event kind 1984", 1984, true},
		{"Regular text note", 1, false},
		{"Metadata event", 0, false},
		{"Contact list", 3, false},
		{"Delete request", 5, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evt := &event.Event{Kind: tc.kind}
			result := IsReportEvent(evt)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestValidateReportEvent(t *testing.T) {
	t.Run("Valid profile report", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "abcdef1234567890", "impersonation"},
			},
		}
		err := ValidateReportEvent(evt)
		require.NoError(t, err)
	})

	t.Run("Valid note report", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"e", "event123", "spam"},
				{"p", "abcdef1234567890"},
			},
		}
		err := ValidateReportEvent(evt)
		require.NoError(t, err)
	})

	t.Run("Missing p tag", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"e", "event123", "spam"},
			},
		}
		err := ValidateReportEvent(evt)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing required 'p' tag")
	})

	t.Run("Empty p tag pubkey", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "", "spam"},
			},
		}
		err := ValidateReportEvent(evt)
		require.Error(t, err)
		require.Contains(t, err.Error(), "p tag cannot have empty pubkey")
	})

	t.Run("Invalid report type in p tag", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "abcdef1234567890", "invalid_type"},
			},
		}
		err := ValidateReportEvent(evt)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid report type")
	})

	t.Run("Invalid report type in e tag", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "abcdef1234567890"},
				{"e", "event123", "invalid_type"},
			},
		}
		err := ValidateReportEvent(evt)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid report type")
	})

	t.Run("Invalid report type in x tag", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "abcdef1234567890"},
				{"x", "blob123", "invalid_type"},
			},
		}
		err := ValidateReportEvent(evt)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid report type")
	})

	t.Run("Wrong kind", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1,
			Tags: [][]string{
				{"p", "abcdef1234567890", "spam"},
			},
		}
		err := ValidateReportEvent(evt)
		require.Error(t, err)
		require.Contains(t, err.Error(), "event kind 1 is not a report (1984)")
	})

	t.Run("Valid report with L and l tags", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "abcdef1234567890", "nudity"},
				{"L", "social.nos.ontology"},
				{"l", "NS-nud", "social.nos.ontology"},
			},
		}
		err := ValidateReportEvent(evt)
		require.NoError(t, err)
	})

	t.Run("Complex valid report with multiple targets", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "user123", "impersonation"},
				{"e", "note1", "spam"},
				{"x", "blob1", "malware"},
				{"e", "note2", "illegal"},
				{"server", "https://example.com/blob1"},
			},
		}
		err := ValidateReportEvent(evt)
		require.NoError(t, err)
	})
}

func TestGetReportedPubKey(t *testing.T) {
	t.Run("Returns pubkey from p tag", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "pubkey123", "spam"},
				{"e", "event123"},
			},
		}
		result := GetReportedPubKey(evt)
		require.Equal(t, "pubkey123", result)
	})

	t.Run("Returns empty string if not a report event", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1,
			Tags: [][]string{
				{"p", "pubkey123"},
			},
		}
		result := GetReportedPubKey(evt)
		require.Empty(t, result)
	})

	t.Run("Returns empty string if no p tag", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"e", "event123"},
			},
		}
		result := GetReportedPubKey(evt)
		require.Empty(t, result)
	})

	t.Run("Uses first p tag if multiple", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "first_pubkey", "spam"},
				{"p", "second_pubkey", "profanity"},
			},
		}
		result := GetReportedPubKey(evt)
		require.Equal(t, "first_pubkey", result)
	})
}

func TestGetReportedEventIDs(t *testing.T) {
	t.Run("Returns single event ID", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "pubkey123"},
				{"e", "event123", "spam"},
			},
		}
		result := GetReportedEventIDs(evt)
		require.Len(t, result, 1)
		require.Equal(t, "event123", result[0])
	})

	t.Run("Returns multiple event IDs", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "pubkey123"},
				{"e", "event1", "spam"},
				{"e", "event2", "illegal"},
			},
		}
		result := GetReportedEventIDs(evt)
		require.Len(t, result, 2)
		require.Contains(t, result, "event1")
		require.Contains(t, result, "event2")
	})

	t.Run("Returns nil if not a report event", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1,
			Tags: [][]string{
				{"e", "event123"},
			},
		}
		result := GetReportedEventIDs(evt)
		require.Nil(t, result)
	})

	t.Run("Returns nil if no e tags", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "pubkey123"},
			},
		}
		result := GetReportedEventIDs(evt)
		require.Len(t, result, 0)
	})
}

func TestGetReportedBlobs(t *testing.T) {
	t.Run("Returns single blob report", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "pubkey123"},
				{"x", "blob123", "malware"},
				{"e", "event123", "malware"},
				{"server", "https://example.com/blob123"},
			},
		}
		result := GetReportedBlobs(evt)
		require.Len(t, result, 1)
		require.Equal(t, "blob123", result[0].Hash)
		require.Equal(t, "malware", result[0].Type)
		require.Equal(t, "event123", result[0].EventID)
		require.Equal(t, "https://example.com/blob123", result[0].Server)
	})

	t.Run("Returns multiple blob reports", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "pubkey123"},
				{"x", "blob1", "malware"},
				{"e", "event1", "malware"},
				{"server", "https://example.com/blob1"},
				{"x", "blob2", "nudity"},
				{"e", "event2", "nudity"},
				{"server", "https://example.com/blob2"},
			},
		}
		result := GetReportedBlobs(evt)
		require.Len(t, result, 2)

		// Check first blob
		require.Equal(t, "blob1", result[0].Hash)
		require.Equal(t, "malware", result[0].Type)
		require.Equal(t, "event1", result[0].EventID)
		require.Equal(t, "https://example.com/blob1", result[0].Server)

		// Check second blob
		require.Equal(t, "blob2", result[1].Hash)
		require.Equal(t, "nudity", result[1].Type)
		require.Equal(t, "event2", result[1].EventID)
		require.Equal(t, "https://example.com/blob2", result[1].Server)
	})

	t.Run("Returns nil if not a report event", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1,
			Tags: [][]string{
				{"x", "blob123", "malware"},
			},
		}
		result := GetReportedBlobs(evt)
		require.Nil(t, result)
	})

	t.Run("Returns nil if no x tags", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "pubkey123"},
				{"e", "event123", "spam"},
			},
		}
		result := GetReportedBlobs(evt)
		require.Len(t, result, 0)
	})

	t.Run("Handles blob with minimal tags", func(t *testing.T) {
		evt := &event.Event{
			Kind: 1984,
			Tags: [][]string{
				{"p", "pubkey123"},
				{"x", "blob123"},
			},
		}
		result := GetReportedBlobs(evt)
		require.Len(t, result, 1)
		require.Equal(t, "blob123", result[0].Hash)
		require.Empty(t, result[0].Type)
		require.Empty(t, result[0].EventID)
		require.Empty(t, result[0].Server)
	})
}

func TestIsValidReportType(t *testing.T) {
	testCases := []struct {
		reportType string
		expected   bool
	}{
		{ReportTypeNudity, true},
		{ReportTypeMalware, true},
		{ReportTypeProfanity, true},
		{ReportTypeIllegal, true},
		{ReportTypeSpam, true},
		{ReportTypeImpersonation, true},
		{ReportTypeOther, true},
		{"invalid_type", false},
		{"", false},
		{"SPAM", false}, // case sensitive
	}

	for _, tc := range testCases {
		t.Run(tc.reportType, func(t *testing.T) {
			result := IsValidReportType(tc.reportType)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestGetReportTypes(t *testing.T) {
	types := GetReportTypes()
	expectedTypes := []string{
		ReportTypeNudity,
		ReportTypeMalware,
		ReportTypeProfanity,
		ReportTypeIllegal,
		ReportTypeSpam,
		ReportTypeImpersonation,
		ReportTypeOther,
	}

	require.Equal(t, len(expectedTypes), len(types))

	// Check that all expected types are present
	for _, expectedType := range expectedTypes {
		require.Contains(t, types, expectedType)
	}
}
