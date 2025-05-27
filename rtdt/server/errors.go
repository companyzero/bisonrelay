package rtdtserver

import (
	"errors"
	"fmt"
)

type errorCode uint64

const (
	errCodeNoError errorCode = 0

	errCodeJoinCookieInvalid       errorCode = 0x100000001
	errCodeJoinCookieWrongPeerID   errorCode = 0x100000002
	errCodeJoinCookieExpired       errorCode = 0x100000003
	errCodeJoinCookiePaymentReused errorCode = 0x100000004
	errCodeSourcePeerNotInSession  errorCode = 0x100000005
	errCodeSourcePeerNotAdmin      errorCode = 0x100000006
	errCodeTargetPeerNotInSession  errorCode = 0x100000007
	errCodeBanned                  errorCode = 0x100000008
	errCodeInvalidRotCookie        errorCode = 0x100000009
	errCodeExpiredRotCookie        errorCode = 0x10000000a
	errCodeMismatchedOldSessId     errorCode = 0x10000000b
	errCodeAlreadyUsedRotCookie    errorCode = 0x10000000c
)

func (ec errorCode) Error() string {
	switch ec {
	case errCodeJoinCookieInvalid:
		return "invalid raw join cookie"
	case errCodeJoinCookieWrongPeerID:
		return "join cookie encodes wrong peer id"
	case errCodeJoinCookieExpired:
		return "join cookie already expired"
	case errCodeJoinCookiePaymentReused:
		return "payment tag already redeemed"
	case errCodeSourcePeerNotInSession:
		return "source peer is not in session"
	case errCodeSourcePeerNotAdmin:
		return "source peer is not an admin of session"
	case errCodeTargetPeerNotInSession:
		return "target peer is not in session"
	case errCodeBanned:
		return "peer is banned from joining this session"
	default:
		return fmt.Sprintf("unknown error code #%016x", uint64(ec))
	}
}

type codedError struct {
	code  errorCode
	msg   string
	inner error
}

func (ce codedError) Error() string {
	if ce.msg != "" {
		return fmt.Sprintf("error code #%08x: %s", uint64(ce.code), ce.msg)
	}

	if ce.inner != nil {
		return fmt.Sprintf("error code #%08x: %s", uint64(ce.code), ce.inner.Error())
	}

	return fmt.Sprintf("error code #%08x: %s", uint64(ce.code), ce.code.Error())
}

func (ce codedError) Unwrap() error {
	if ce.inner != nil {
		return ce.inner
	}
	return ce.code
}

func (ce codedError) As(target interface{}) bool {
	switch t := target.(type) {
	case *codedError:
		*t = ce
		return true

	case *errorCode:
		*t = ce.code
		return true
	}

	return false
}

func makeCodedError(code errorCode, inner error) codedError {
	return codedError{code: code, inner: inner}
}

func errorCodeFromError(err error) errorCode {
	var res errorCode
	if errors.As(err, &res) {
		return res
	}
	return 0
}

var (
	errPingTooRecent   = errors.New("last ping is too recent")
	errPingTooLarge    = errors.New("ping has too large payload")
	errConnTimedOut    = errors.New("connection timed out")
	errBanScoreReached = errors.New("ban score reached")
	errNotInSession    = errors.New("peer is not in session")
)
