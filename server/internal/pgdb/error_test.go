package pgdb

import (
	"errors"
	"io"
	"testing"
)

// TestErrorKindStringer tests the stringized output for the ErrorKind type.
func TestErrorKindStringer(t *testing.T) {
	tests := []struct {
		in   ErrorKind
		want string
	}{
		{ErrMissingDatabase, "ErrMissingDatabase"},
		{ErrConnFailed, "ErrConnFailed"},
		{ErrBeginTx, "ErrBeginTx"},
		{ErrCommitTx, "ErrCommitTx"},
		{ErrQueryFailed, "ErrQueryFailed"},
		{ErrMissingRole, "ErrMissingRole"},
		{ErrMissingTablespace, "ErrMissingTablespace"},
		{ErrBadSetting, "ErrBadSetting"},
		{ErrMissingTable, "ErrMissingTable"},
		{ErrBadDataTablespace, "ErrBadDataTablespace"},
		{ErrBadIndexTablespace, "ErrBadIndexTablespace"},
		{ErrMissingProc, "ErrMissingProc"},
		{ErrMissingTrigger, "ErrMissingTrigger"},
		{ErrOldDatabase, "ErrOldDatabase"},
	}

	for i, test := range tests {
		result := test.in.Error()
		if result != test.want {
			t.Errorf("#%d: got: %s want: %s", i, result, test.want)
			continue
		}
	}
}

// TestContextError tests the error output for the ContextError type.
func TestContextError(t *testing.T) {
	tests := []struct {
		in   ContextError
		want string
	}{{
		ContextError{Description: "duplicate block"},
		"duplicate block",
	}, {
		ContextError{Description: "human-readable error"},
		"human-readable error",
	}}

	for i, test := range tests {
		result := test.in.Error()
		if result != test.want {
			t.Errorf("#%d: got: %s want: %s", i, result, test.want)
			continue
		}
	}
}

// TestErrorKindIsAs ensures both ErrorKind and Error can be identified as being
// a specific error kind via errors.Is and unwrapped via errors.As.
func TestErrorKindIsAs(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		target    error
		wantMatch bool
		wantAs    ErrorKind
	}{{
		name:      "ErrQueryFailed == ErrQueryFailed",
		err:       ErrQueryFailed,
		target:    ErrQueryFailed,
		wantMatch: true,
		wantAs:    ErrQueryFailed,
	}, {
		name:      "ContextError.ErrQueryFailed == ErrQueryFailed",
		err:       contextError(ErrQueryFailed, "", nil),
		target:    ErrQueryFailed,
		wantMatch: true,
		wantAs:    ErrQueryFailed,
	}, {
		name:      "ContextError.ErrQueryFailed == ContextError.ErrQueryFailed",
		err:       contextError(ErrQueryFailed, "", nil),
		target:    contextError(ErrQueryFailed, "", nil),
		wantMatch: true,
		wantAs:    ErrQueryFailed,
	}, {
		name:      "ErrQueryFailed != ErrMissingRole",
		err:       ErrQueryFailed,
		target:    ErrMissingRole,
		wantMatch: false,
		wantAs:    ErrQueryFailed,
	}, {
		name:      "ContextError.ErrQueryFailed != ErrMissingRole",
		err:       contextError(ErrQueryFailed, "", nil),
		target:    ErrMissingRole,
		wantMatch: false,
		wantAs:    ErrQueryFailed,
	}, {
		name:      "ErrQueryFailed != ContextError.ErrMissingRole",
		err:       ErrQueryFailed,
		target:    contextError(ErrMissingRole, "", nil),
		wantMatch: false,
		wantAs:    ErrQueryFailed,
	}, {
		name:      "ContextError.ErrQueryFailed != ContextError.ErrMissingRole",
		err:       contextError(ErrQueryFailed, "", nil),
		target:    contextError(ErrMissingRole, "", nil),
		wantMatch: false,
		wantAs:    ErrQueryFailed,
	}, {
		name:      "ContextError.ErrQueryFailed != io.EOF",
		err:       contextError(ErrQueryFailed, "", nil),
		target:    io.EOF,
		wantMatch: false,
		wantAs:    ErrQueryFailed,
	}}

	for _, test := range tests {
		// Ensure the error matches or not depending on the expected result.
		result := errors.Is(test.err, test.target)
		if result != test.wantMatch {
			t.Errorf("%s: incorrect error identification -- got %v, want %v",
				test.name, result, test.wantMatch)
			continue
		}

		// Ensure the underlying error kind can be unwrapped and is the expected
		// kind.
		var kind ErrorKind
		if !errors.As(test.err, &kind) {
			t.Errorf("%s: unable to unwrap to error kind", test.name)
			continue
		}
		if kind != test.wantAs {
			t.Errorf("%s: unexpected unwrapped error kind -- got %v, want %v",
				test.name, kind, test.wantAs)
			continue
		}
	}
}
