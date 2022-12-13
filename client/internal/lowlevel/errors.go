package lowlevel

import (
	"errors"
	"fmt"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
)

var (
	errRMQExiting     = fmt.Errorf("rmq: %w", clientintf.ErrSubsysExiting)
	errRdvzMgrExiting = fmt.Errorf("rendezvous manager: %w", clientintf.ErrSubsysExiting)

	errNoPeerTLSCert = errors.New("server did not send any TLS certs")

	errPongTimeout = errors.New("pong timeout")

	// errSessRequestedClose is a guard error to signal the session was
	// closed on request.
	errSessRequestedClose = errors.New("requested session close")
	errORMTooLarge        = errors.New("outbound RM encrypted len greater than max allowed msg size")
)

// kxError is returned when the server KX stage fails.
type kxError struct {
	err error
}

func (err kxError) Error() string {
	return fmt.Sprintf("unable to complete kx: %v", err.err)
}

func (err kxError) Unwrap() error {
	return err.err
}

func (err kxError) Is(target error) bool {
	if _, ok := target.(kxError); ok {
		return true
	}

	return false
}

func makeKxError(err error) kxError {
	return kxError{err: err}
}

// AckError is an error generated when the server sends an Acknowledge message
// with an embedded Error message.
//
// Is is also used in client code to signal a given pushed message was
// processed with an error.
type AckError struct {
	ErrorStr  string
	ErrorCode int
	NonFatal  bool
}

func (err AckError) Error() string {
	return fmt.Sprintf("server returned ack error %d: %s",
		err.ErrorCode, err.ErrorStr)
}

func (err AckError) Is(target error) bool {
	if _, ok := target.(AckError); ok {
		return true
	}

	return false
}

func makeAckError(ack *rpc.Acknowledge) AckError {
	return AckError{ack.Error, ack.ErrorCode, false}
}

// UnwelcomeError is an error generated when the server responds with an
// Unwelcome message during the welcome stage of connection setup.
type UnwelcomeError struct {
	Reason string
}

func (err UnwelcomeError) Error() string {
	return fmt.Sprintf("server un-welcomed connection: %s", err.Reason)
}

func (err UnwelcomeError) Is(target error) bool {
	if _, ok := target.(UnwelcomeError); ok {
		return true
	}

	return false
}

func makeUnwelcomeError(reason string) UnwelcomeError {
	return UnwelcomeError{Reason: reason}
}

type invalidRecvTagError struct {
	cmd string
	tag uint32
}

func (err invalidRecvTagError) Error() string {
	return fmt.Sprintf("server sent invalid tag %d on msg %s", err.tag, err.cmd)
}

func (err invalidRecvTagError) Is(target error) bool {
	if _, ok := target.(invalidRecvTagError); ok {
		return true
	}

	return false
}

func makeInvalidRecvTagError(cmd string, tag uint32) invalidRecvTagError {
	return invalidRecvTagError{cmd: cmd, tag: tag}
}

// ToAck copies this error to the given Acknowledge msg.
func (err AckError) ToAck(ack *rpc.Acknowledge) {
	ack.Error = err.ErrorStr
	ack.ErrorCode = err.ErrorCode
}

type unmarshalError struct {
	what string
	err  error
}

func (err unmarshalError) Error() string {
	return fmt.Sprintf("unable to unmarshal %s: %v", err.what, err.err)
}

func (err unmarshalError) Unwrap() error {
	return err.err
}

func (err unmarshalError) Is(target error) bool {
	if _, ok := target.(unmarshalError); ok {
		return true
	}

	return false
}

func makeUnmarshalError(what string, err error) unmarshalError {
	return unmarshalError{what: what, err: err}
}

type routeMessageReplyError struct {
	errorStr string
}

func (err routeMessageReplyError) Error() string {
	return fmt.Sprintf("server replied error on rm: %s", err.errorStr)
}

func (err routeMessageReplyError) Is(target error) bool {
	_, ok := target.(routeMessageReplyError)
	return ok
}

type ErrRVAlreadySubscribed struct {
	rv string
}

func (err ErrRVAlreadySubscribed) Error() string {
	return fmt.Sprintf("already subscribed to rendezvous '%s'", err.rv)
}

func (err ErrRVAlreadySubscribed) Is(target error) bool {
	_, ok := target.(ErrRVAlreadySubscribed)
	return ok
}

func makeErrRVAlreadySubscribed(rv string) ErrRVAlreadySubscribed {
	return ErrRVAlreadySubscribed{rv: rv}
}

type ErrRVAlreadyUnsubscribed struct {
	rv string
}

func (err ErrRVAlreadyUnsubscribed) Error() string {
	return fmt.Sprintf("already unsubscribed to rendezvous '%s'", err.rv)
}

func (err ErrRVAlreadyUnsubscribed) Is(target error) bool {
	_, ok := target.(ErrRVAlreadyUnsubscribed)
	return ok
}

func makeRdvzAlreadyUnsubscribedError(rv string) ErrRVAlreadyUnsubscribed {
	return ErrRVAlreadyUnsubscribed{rv: rv}
}
