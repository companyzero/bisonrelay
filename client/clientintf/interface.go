package clientintf

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// ID is a 32-byte global ID. This is used as an alias for all 32-byte arrays
// that are interpreted as unique IDs.
type ID = zkidentity.ShortID

func RandomID() ID {
	var id ID
	n, err := rand.Read(id[:])
	if err != nil {
		panic(err)
	}
	if n != len(id) {
		panic("insufficient entropy to generate ID")
	}
	return id
}

type UserID = ID
type PostID = ID
type RawRVID = ID

// Conn represents the required functions for a remote connection to a server.
type Conn interface {
	io.Reader
	io.Writer
	io.Closer
	RemoteAddr() net.Addr
}

// Dialer is a function that can generate new connections to a server.
type Dialer func(context.Context) (Conn, *tls.ConnectionState, error)

// CertConfirmer is a functiion that can be called to confirm whether a given
// server is safe.
type CertConfirmer func(context.Context, *tls.ConnectionState, *zkidentity.PublicIdentity) error

// ServerPolicy is the policy for a given server session.
type ServerPolicy struct {
	PushPaymentLifetime time.Duration
	MaxPushInvoices     int
}

// ServerSessionIntf is the interface available from serverSession to
// consumers.
type ServerSessionIntf interface {
	SendPRPC(msg rpc.Message, payload interface{}, reply chan<- interface{}) error
	RequestClose(err error)
	PayClient() PaymentClient
	PaymentRates() (uint64, uint64)
	ExpirationDays() int
	Policy() ServerPolicy

	// Context returns a context that gets cancelled once this session stops
	// running.
	Context() context.Context
}

// DecodedInvoice represents an invoice that was successfully decoded by the
// PaymentClient
type DecodedInvoice struct {
	ID         []byte
	MAtoms     int64
	ExpiryTime time.Time
}

// isExpired is similar to IsExpired, but with a parametrized nowFunc to allow
// unit testing.
func (inv *DecodedInvoice) isExpired(affordance time.Duration, nowFunc func() time.Time) bool {
	now := nowFunc()
	return inv.ExpiryTime.Before(now.Add(affordance))
}

// IsExpired returns whether this invoice has expired taking
// into account the specified affordance. In other words, it returns true if
// the expiration time of the invoice is before time.Now().Add(affordance).
func (inv *DecodedInvoice) IsExpired(affordance time.Duration) bool {
	return inv.isExpired(affordance, time.Now)
}

// PaymentClient is the interface for clients that can pay for invoices.
type PaymentClient interface {
	PayScheme() string
	PayInvoice(context.Context, string) (int64, error)
	PayInvoiceAmount(context.Context, string, int64) (int64, error)
	GetInvoice(context.Context, int64, func(int64)) (string, error)
	DecodeInvoice(context.Context, string) (DecodedInvoice, error)
	IsInvoicePaid(context.Context, int64, string) error
	IsPaymentCompleted(context.Context, string) (int64, error)
}

// FreePaymentClient implements the PaymentClient interface for servers that
// offer the "free" payment scheme: namely, invoices are requested but there is
// nothing to pay for.
type FreePaymentClient struct{}

func (pc FreePaymentClient) PayScheme() string                                 { return rpc.PaySchemeFree }
func (pc FreePaymentClient) PayInvoice(context.Context, string) (int64, error) { return 0, nil }
func (pc FreePaymentClient) PayInvoiceAmount(context.Context, string, int64) (int64, error) {
	return 0, nil
}
func (pc FreePaymentClient) GetInvoice(ctx context.Context, mat int64, cb func(int64)) (string, error) {
	return fmt.Sprintf("free invoice for %d milliatoms", mat), nil
}
func (pc FreePaymentClient) IsInvoicePaid(context.Context, int64, string) error {
	return nil
}
func (pc FreePaymentClient) IsPaymentCompleted(context.Context, string) (int64, error) {
	return 0, nil
}

// farFutureExpiryTime is a time far in the future for the expiration of free
// invoices.
var farFutureExpiryTime = time.Date(2200, 01, 01, 0, 0, 0, 0, time.UTC)

func (pc FreePaymentClient) DecodeInvoice(_ context.Context, invoice string) (DecodedInvoice, error) {
	var id [32]byte
	copy(id[:], invoice)
	return DecodedInvoice{ExpiryTime: farFutureExpiryTime, ID: id[:]}, nil
}

var (
	ErrSubsysExiting             = errors.New("subsys exiting")
	ErrInvoiceInsufficientlyPaid = errors.New("invoice insufficiently paid")
)
