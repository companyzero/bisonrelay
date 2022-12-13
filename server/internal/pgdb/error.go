package pgdb

import (
	"errors"
)

// ErrorKind identifies a kind of error.  It has full support for errors.Is and
// errors.As, so the caller can directly check against an error kind when
// determining the reason for an error.
type ErrorKind string

// These constants are used to identify a specific ErrorKind.
const (
	// ErrMissingDatabase indicates the required backend database does not
	// exist.
	ErrMissingDatabase = ErrorKind("ErrMissingDatabase")

	// ErrConnFailed indicates an error when attempting to connect to the
	// database that is not covered by a more specific connection failure error
	// such as a missing database.
	ErrConnFailed = ErrorKind("ErrConnFailed")

	// ErrBeginTx indicates an error when attempting to start a database
	// transaction.
	ErrBeginTx = ErrorKind("ErrBeginTx")

	// ErrCommitTx indicates an error when attempting to commit a database
	// transaction.
	ErrCommitTx = ErrorKind("ErrCommitTx")

	// ErrQueryFailed indicates an unexpected error happened when executing a
	// SQL query on the database.
	ErrQueryFailed = ErrorKind("ErrQueryFailed")

	// ErrMissingRole indicates the required role does not exist.
	ErrMissingRole = ErrorKind("ErrMissingRole")

	// ErrMissingTablespace indicates a required tablespace does not exist.
	ErrMissingTablespace = ErrorKind("ErrMissingTablespace")

	// ErrBadSetting indicates the database does not have a configuration option
	// set to a required value.
	ErrBadSetting = ErrorKind("ErrBadSetting")

	// ErrMissingTable indicates a required table does not exist.
	ErrMissingTable = ErrorKind("ErrMissingTable")

	// ErrBadDataTablespace indicates a table does not have the expected data
	// tablespace.
	ErrBadDataTablespace = ErrorKind("ErrBadDataTablespace")

	// ErrBadIndexTablespace indicates a table does not have the expected index
	// tablespace.
	ErrBadIndexTablespace = ErrorKind("ErrBadIndexTablespace")

	// ErrMissingProc indicates a required stored procedure does not exist.
	ErrMissingProc = ErrorKind("ErrMissingProc")

	// ErrMissingTrigger indicates a required table constraint does not
	// exist.
	ErrMissingTrigger = ErrorKind("ErrMissingTrigger")

	// ErrOldDatabase indicates a database has been upgraded to a newer version
	// that is no longer compatible with the current version of the software.
	ErrOldDatabase = ErrorKind("ErrOldDatabase")

	// ErrUpgradeV2 indicates an error that happened during the upgrade to
	// the version 2 database.
	ErrUpgradeV2 = ErrorKind("ErrUpgradeV2")
)

// Error satisfies the error interface and prints human-readable errors.
func (e ErrorKind) Error() string {
	return string(e)
}

// ContextError wraps an error with additional context.  It has full support for
// errors.Is and errors.As, so the caller can ascertain the specific wrapped
// error.
//
// RawErr contains the original error in the case where an error has been
// converted.
type ContextError struct {
	Err         error
	Description string
	RawErr      error
}

// Error satisfies the error interface and prints human-readable errors.
func (e ContextError) Error() string {
	return e.Description
}

// Is implements the interface to work with the standard library's errors.Is.
//
// It calls errors.Is on both the Err and RawErr fields, in that order, and
// returns true when one of them matches the target.  Otherwise, it returns
// false.
//
// This means it keeps all of the same semantics typically provided by Is in
// terms of unwrapping error chains.
func (e ContextError) Is(err error) bool {
	if errors.Is(e.Err, err) {
		return true
	}
	return errors.Is(e.RawErr, err)
}

// As implements the interface to work with the standard library's errors.As.
//
// It calls errors.As on both the Err and RawErr fields, in that order, and
// returns true when one of them matches the target.  Otherwise, it returns
// false.
//
// This means it keeps all of the same semantics typically provided by As in
// terms of unwrapping error chains and setting the target to the matched error.
func (e ContextError) As(target interface{}) bool {
	if errors.As(e.Err, target) {
		return true
	}
	return errors.As(e.RawErr, target)
}

// contextError creates a ContextError given a set of arguments.
func contextError(kind ErrorKind, desc string, rawErr error) ContextError {
	return ContextError{Err: kind, Description: desc, RawErr: rawErr}
}
