package zkidentity

import (
	"bytes"
	"testing"
)

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

func TestShortIDLess(t *testing.T) {
	tests := []struct {
		name string
		id1  ShortID
		id2  ShortID
		want bool
	}{{
		name: "zero ids",
		want: false,
	}, {
		name: "first byte higher",
		id1:  ShortID{0: 0x02},
		id2:  ShortID{0: 0x01},
		want: false,
	}, {
		name: "first byte equal ",
		id1:  ShortID{0: 0x01},
		id2:  ShortID{0: 0x01},
		want: false,
	}, {
		name: "first byte lower",
		id1:  ShortID{0: 0x01},
		id2:  ShortID{0: 0x02},
		want: true,
	}, {
		name: "second byte higher",
		id1:  ShortID{1: 0x02},
		id2:  ShortID{1: 0x01},
		want: false,
	}, {
		name: "second byte equal ",
		id1:  ShortID{1: 0x01},
		id2:  ShortID{1: 0x01},
		want: false,
	}, {
		name: "second byte lower",
		id1:  ShortID{1: 0x01},
		id2:  ShortID{1: 0x02},
		want: true,
	}, {
		name: "last byte higher",
		id1:  ShortID{31: 0x02},
		id2:  ShortID{31: 0x01},
		want: false,
	}, {
		name: "last byte equal ",
		id1:  ShortID{31: 0x01},
		id2:  ShortID{31: 0x01},
		want: false,
	}, {
		name: "last byte lower",
		id1:  ShortID{31: 0x01},
		id2:  ShortID{31: 0x02},
		want: true,
	}, {
		name: "earlier byte higher",
		id1:  ShortID{16: 0x02},
		id2:  ShortID{17: 0x01},
		want: false,
	}, {
		name: "earlier byte higher and equal bytes",
		id1:  ShortID{16: 0x02, 17: 0x01},
		id2:  ShortID{17: 0x01},
		want: false,
	}, {
		name: "earlier byte equal and then lower",
		id1:  ShortID{16: 0x02, 17: 0x02},
		id2:  ShortID{16: 0x02, 17: 0x01},
		want: false,
	}, {
		name: "earlier byte higher and then higher",
		id1:  ShortID{16: 0x02, 17: 0x01},
		id2:  ShortID{16: 0x02, 17: 0x02},
		want: true,
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.id1.Less(&tc.id2)
			if got != tc.want {
				t.Fatalf("unexpected result: got %v, want %v",
					got, tc.want)
			}
			got2 := bytes.Compare(tc.id1[:], tc.id2[:]) < 0
			if got2 != got {
				t.Fatalf("unexpected result: got2 %v, got %v",
					got2, got)
			}
		})
	}

}
