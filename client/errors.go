package client

import (
	"errors"
	"fmt"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/zkidentity"
)

var (
	errRemoteUserExiting = fmt.Errorf("remote user: %w", clientintf.ErrSubsysExiting)
	errClientExiting     = fmt.Errorf("client: %w", clientintf.ErrSubsysExiting)
	errAlreadyExists     = fmt.Errorf("already exists")
	errUserBlocked       = fmt.Errorf("user is blocked")
	errRMTooLarge        = errors.New("RM is too large")
)

type userNotFoundError struct {
	id string
}

func (err userNotFoundError) Error() string {
	return fmt.Sprintf("user %s not found", err.id)
}

func (err userNotFoundError) Is(target error) bool {
	_, ok := target.(userNotFoundError)
	return ok
}

type alreadyHaveUserError struct {
	id UserID
}

func (err alreadyHaveUserError) Error() string {
	return fmt.Sprintf("already have user %s", err.id)
}

func (err alreadyHaveUserError) Is(target error) bool {
	_, ok := target.(alreadyHaveUserError)
	return ok
}

// WalletUsableErrorKind holds the types of errors that may happen when checking
// if an LN wallet is usable for payments to the server.
type WalletUsableErrorKind string

func (err WalletUsableErrorKind) Error() string {
	return string(err)
}

// WalletUsableError is a complex error type that wraps both one the type of
// error and an underlying error (if it exists)
type WalletUsableError struct {
	descr string
	err   error
}

func (err WalletUsableError) Error() string {
	return err.descr
}

func (err WalletUsableError) Unwrap() error {
	return err.err
}

func makeWalletUsableErr(kind WalletUsableErrorKind, descr string) WalletUsableError {
	return WalletUsableError{descr: descr, err: kind}
}

// ErrKXSearchNeeded is returned when an action cannot be completed and a KX
// search must be performed.
type ErrKXSearchNeeded struct {
	Author UserID
}

func (err ErrKXSearchNeeded) Error() string {
	return fmt.Sprintf("KX search needed to find post author %s", err.Author)
}

func (err ErrKXSearchNeeded) Is(target error) bool {
	if _, ok := target.(ErrKXSearchNeeded); ok {
		return true
	}
	return false
}

type errHasOngoingKX struct {
	otherRV zkidentity.ShortID
}

func (err errHasOngoingKX) Error() string {
	return fmt.Sprintf("already has ongoing KX %s", err.otherRV)
}

func (err errHasOngoingKX) Is(target error) bool {
	_, ok := target.(errHasOngoingKX)
	return ok
}

func (err errHasOngoingKX) As(target interface{}) bool {
	other, ok := target.(*errHasOngoingKX)
	if !ok {
		return false
	}
	other.otherRV = err.otherRV
	return true
}
