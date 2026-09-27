package relay

import (
	"testing"

	"github.com/paulborile/glienicke/pkg/event"
)

func intp(n int) *int { return &n }

// TestClampFilterLimits verifies the per-filter limit is capped at
// maxEventsPerREQ: unset and over-cap limits are pulled down, under-cap limits
// are left alone. This is what bounds how many rows a query materializes.
func TestClampFilterLimits(t *testing.T) {
	r := &Relay{maxEventsPerREQ: defaultMaxEventsPerREQ} // 100

	cases := []struct {
		name string
		in   *int
		want int
	}{
		{"no limit -> capped", nil, defaultMaxEventsPerREQ},
		{"over cap -> capped", intp(5000), defaultMaxEventsPerREQ},
		{"under cap -> unchanged", intp(10), 10},
		{"exactly cap -> unchanged", intp(defaultMaxEventsPerREQ), defaultMaxEventsPerREQ},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &event.Filter{Limit: tc.in}
			r.clampFilterLimits([]*event.Filter{f})
			if f.Limit == nil {
				t.Fatalf("limit must always be set after clamp, got nil")
			}
			if *f.Limit != tc.want {
				t.Fatalf("clamped limit = %d, want %d", *f.Limit, tc.want)
			}
		})
	}
}

// TestClampFilterLimitsAllFilters verifies every filter in a multi-filter REQ is
// clamped, not just the first — otherwise an OR'd filter set could still
// materialize an unbounded result.
func TestClampFilterLimitsAllFilters(t *testing.T) {
	r := &Relay{maxEventsPerREQ: defaultMaxEventsPerREQ}

	filters := []*event.Filter{
		{Limit: nil},
		{Limit: intp(99999)},
		{Limit: intp(5)},
	}
	r.clampFilterLimits(filters)

	for i, f := range filters {
		if f.Limit == nil || *f.Limit > defaultMaxEventsPerREQ {
			t.Fatalf("filter %d not clamped: %v", i, f.Limit)
		}
	}
	if *filters[2].Limit != 5 {
		t.Fatalf("under-cap filter should be unchanged, got %d", *filters[2].Limit)
	}
}

// TestClampFilterLimitsDisabled verifies a non-positive cap leaves filters
// untouched (cap disabled).
func TestClampFilterLimitsDisabled(t *testing.T) {
	r := &Relay{maxEventsPerREQ: 0}
	f := &event.Filter{Limit: nil}
	r.clampFilterLimits([]*event.Filter{f})
	if f.Limit != nil {
		t.Fatalf("with cap disabled, an unset limit must stay nil, got %d", *f.Limit)
	}
}
