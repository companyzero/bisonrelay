package lowlevel

import (
	"errors"
	"testing"

	"github.com/companyzero/bisonrelay/rpc"
)

// TestKxErrors asserts the kxError type works as intended.
func TestKxErrors(t *testing.T) {
	err1 := makeKxError(errors.New("err1"))
	err2 := makeKxError(errors.New("err2"))
	if !errors.Is(err1, err2) {
		t.Fatalf("unexpected errors.Is result")
	}

	if !errors.Is(err1, kxError{}) {
		t.Fatalf("unexpected errors.Is result")
	}
}

// TestUnwelcomeError asserts the UnwelcomeError type works as intended.
func TestUnwelcomeError(t *testing.T) {
	var err1 error = makeUnwelcomeError("reason 1")
	var err2 error = makeUnwelcomeError("reason 2")
	if !errors.Is(err1, err2) {
		t.Fatalf("unexpected errors.Is result")
	}

	if !errors.Is(err1, UnwelcomeError{}) {
		t.Fatalf("unexpected errors.Is result")
	}

	if errors.Is(err1, errors.New("other")) {
		t.Fatalf("unexpected errors.Is result")
	}
}

// TestAckError asserts the AckError type works as intended.
func TestAckError(t *testing.T) {
	var err1 error = makeAckError(&rpc.Acknowledge{Error: "zero"})
	var err2 error = makeAckError(&rpc.Acknowledge{Error: "two"})
	if !errors.Is(err1, err2) {
		t.Fatalf("unexpected errors.Is result")
	}

	if !errors.Is(err1, AckError{}) {
		t.Fatalf("unexpected errors.Is result")
	}

	if errors.Is(err1, errors.New("other")) {
		t.Fatalf("unexpected errors.Is result")
	}
}

// TestUnmarshalErrors asserts the unmarshalError type works as intended.
func TestUnmarshalErrors(t *testing.T) {
	err1 := makeUnmarshalError("1", errors.New("err1"))
	err2 := makeUnmarshalError("2", errors.New("err2"))
	if !errors.Is(err1, err2) {
		t.Fatalf("unexpected errors.Is result")
	}

	if !errors.Is(err1, unmarshalError{}) {
		t.Fatalf("unexpected errors.Is result")
	}
}

// TestInvalidRecvTagErrors asserts the invalidRecvTagError type works as
// intended.
func TestInvalidRecvTagErrors(t *testing.T) {
	err1 := makeInvalidRecvTagError("cmd", 0)
	err2 := makeInvalidRecvTagError("cmd2", 2)
	if !errors.Is(err1, err2) {
		t.Fatalf("unexpected errors.Is result")
	}

	if !errors.Is(err1, invalidRecvTagError{}) {
		t.Fatalf("unexpected errors.Is result")
	}
}
