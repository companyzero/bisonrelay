package client

import (
	"errors"
	"fmt"
	"testing"

	"github.com/companyzero/bisonrelay/zkidentity"
)

func TestErrHasOngoingKX(t *testing.T) {
	otherRV := zkidentity.ShortID{31: 0x01}
	err1 := errHasOngoingKX{otherRV: otherRV}
	errWrapped := fmt.Errorf("wrapped %w", err1)
	if !errors.Is(errWrapped, errHasOngoingKX{}) {
		t.Fatalf("does not unwrap to errHasOngoingKX")
	}

	var err2 errHasOngoingKX
	if !errors.As(errWrapped, &err2) {
		t.Fatalf("does not unwrap as errHasOngoingKX")
	}

	if err2.otherRV != otherRV {
		t.Fatalf("unexpected other rv: got %s, want %s", err2.otherRV,
			otherRV)
	}
}
