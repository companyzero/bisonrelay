package zkidentity

import "testing"

func TestShortIDConstantTimeEq(t *testing.T) {
	tests := []struct {
		name string
		id1  ShortID
		id2  ShortID
		want bool
	}{{
		name: "equal zero short ids",
		id1:  ShortID{},
		id2:  ShortID{},
		want: true,
	}, {
		name: "equal non-zero short ids",
		id1:  ShortID{0: 0x5a, 31: 0xa5},
		id2:  ShortID{0: 0x5a, 31: 0xa5},
		want: true,
	}, {
		name: "unequal short ids",
		id1:  ShortID{0: 0x5a, 31: 0xa5},
		id2:  ShortID{0: 0x5a, 31: 0xa4},
		want: false,
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.id1.ConstantTimeEq(&tc.id2)
			if got != tc.want {
				t.Fatalf("unexpected result: got %v, want %v",
					got, tc.want)
			}
		})
	}
}
