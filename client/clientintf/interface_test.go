package clientintf

import (
	"testing"
	"time"
)

// TestInvoiceIsExpired tests that the IsExpired function works as intended.
func TestInvoiceIsExpired(t *testing.T) {
	base := time.Date(2020, 01, 01, 0, 0, 0, 0, time.UTC)
	minute := time.Minute

	tests := []struct {
		name       string
		now        time.Time
		expiry     time.Time
		affordance time.Duration
		expired    bool
	}{{
		name:       "now < expiry - aff < expiry",
		affordance: minute,
		expiry:     base.Add(2 * minute),
		now:        base,
		expired:    false,
	}, {
		name:       "now = expiry - aff + 1 < expiry",
		affordance: minute,
		expiry:     base.Add(minute + 1),
		now:        base,
		expired:    false,
	}, {
		name:       "now = expiry - aff < expiry",
		affordance: minute,
		expiry:     base.Add(minute),
		now:        base,
		expired:    false,
	}, {
		name:       "expiry - aff - 1 = now < expiry",
		affordance: minute,
		expiry:     base.Add(minute - 1),
		now:        base,
		expired:    true,
	}, {
		name:       "expiry - aff < now = expiry",
		affordance: minute,
		expiry:     base,
		now:        base,
		expired:    true,
	}, {
		name:       "expiry - aff < expiry < now",
		affordance: minute,
		expiry:     base.Add(-1),
		now:        base,
		expired:    true,
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			inv := DecodedInvoice{ExpiryTime: tc.expiry}
			nowFunc := func() time.Time {
				return tc.now
			}
			got := inv.isExpired(tc.affordance, nowFunc)
			if got != tc.expired {
				t.Fatalf("unexpected result: got %v, want %v",
					got, tc.expired)
			}
		})
	}
}
