package rtdtserver

import (
	"errors"
	"fmt"
	"testing"
)

// TestErrorCodeFromError tests that the error code can be extracted from
// various error types.
func TestErrorCodeFromError(t *testing.T) {

	const errTestCode errorCode = 0x12345678
	const noErrorCode errorCode = 0

	tests := []struct {
		name string
		err  error
		want errorCode
	}{{
		name: "error code",
		err:  errTestCode,
		want: errTestCode,
	}, {
		name: "coded error",
		err:  makeCodedError(errTestCode, nil),
		want: errTestCode,
	}, {
		name: "coded error with inner",
		err:  makeCodedError(errTestCode, errors.New("some inner error")),
		want: errTestCode,
	}, {
		name: "wrapped error code",
		err:  fmt.Errorf("wrapped error %w", errTestCode),
		want: errTestCode,
	}, {
		name: "wrapped coded error",
		err:  fmt.Errorf("wrapped coded error %w", makeCodedError(errTestCode, nil)),
		want: errTestCode,
	}, {
		name: "nil error",
		err:  nil,
		want: noErrorCode,
	}, {
		name: "misc error",
		err:  errors.New("some misc error"),
		want: noErrorCode,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := errorCodeFromError(tc.err)
			if got != tc.want {
				t.Fatalf("unexpected error code: got %d, want %d",
					got, tc.want)
			}
		})
	}
}
