package jsonrpc

import (
	"errors"
	"fmt"
	"io"
)

// ErrorCode is a JSON-RPC error code.
type ErrorCode int

func (err ErrorCode) Error() string {
	switch err {
	case ErrEOF:
		return "EOF"
	case ErrParseError:
		return "JSON parsing error"
	case ErrInvalidRequest:
		return "invalid JSON-RPC request"
	case ErrMethodNotFound:
		return "method not found"
	case ErrInvalidParams:
		return "invalid parameters"
	case ErrInternal:
		return "internal error"
	default:
		return fmt.Sprintf("error code %d", int(err))
	}
}

const (
	// Application defined error codes.
	ErrEOF = 10000

	// JSON-RPC defined error codes.
	ErrParseError     = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternal       = -32603
)

// Error is a JSON-RPC error response.
type Error struct {
	Code    ErrorCode   `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (err Error) Error() string {
	if err.Message == "" {
		return err.Code.Error()
	}
	return err.Message
}

// MakeError creates a JSON-RPC Error with the given code and message.
func MakeError(code ErrorCode, msg string) Error {
	if msg == "" {
		msg = code.Error()
	}
	return Error{Code: code, Message: msg}
}

func newError(code ErrorCode, msg string) *Error {
	err := MakeError(code, msg)
	return &err
}

func outboundFromError(id interface{}, err error) outboundMsg {
	res := outboundMsg{
		ID:      id,
		Version: version,
	}

	switch err := err.(type) {
	case ErrorCode:
		res.Error = newError(err, "")
	case Error:
		res.Error = &err
	case *Error:
		res.Error = err
	default:
		switch {
		case errors.Is(err, io.EOF):
			res.Error = newError(ErrEOF, "EOF")
		default:
			res.Error = newError(ErrInternal, err.Error())
		}
	}

	return res
}
