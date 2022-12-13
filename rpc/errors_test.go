package rpc

import (
	"errors"
	"testing"

	"github.com/companyzero/bisonrelay/ratchet"
)

func TestErrUnpaidSubscriptionRV(t *testing.T) {
	r1 := ratchet.RVPoint{0: 0x01, 31: 0xff}
	err1 := ErrUnpaidSubscriptionRV(r1)

	if !errors.Is(err1, ErrUnpaidSubscriptionRV{}) {
		t.Fatalf("unexpected errors.Is result: got false, want true")
	}

	// Test against a hard-coded error string because this is decoded in
	// the client. Changing this string breaks existing clients.
	gotStr := err1.Error()
	wantStr := "unpaid subscription to RV 01000000000000000000000000000000000000000000000000000000000000ff"
	if gotStr != wantStr {
		t.Fatalf("unexpected error string: got %s, want %s", gotStr, wantStr)
	}

	// Test the parsing function.
	err2 := ParseErrUnpaidSubscriptionRV(gotStr)
	if err2 == nil {
		t.Fatalf("unexpected nil result while parsing error")
	}

	if !errors.Is(err2, ErrUnpaidSubscriptionRV{}) {
		t.Fatalf("unexpected errors.Is result: got false, want true")
	}

	r2 := ratchet.RVPoint(err1)
	if r2 != r1 {
		t.Fatalf("unexpected RVs: got %s, want %s", r2, r1)
	}

	var err3 ErrUnpaidSubscriptionRV
	if !errors.As(err1, &err3) {
		t.Fatalf("unexpected errors.As result: got false, want true")
	}
	r3 := ratchet.RVPoint(err3)
	if r1 != r3 {
		t.Fatalf("unexpected RVs: got %s, want %s", r3, r1)
	}
}
