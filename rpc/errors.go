package rpc

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/companyzero/bisonrelay/ratchet"
)

// ErrRMInvoicePayment is generated on servers when an RM push fails due
// to some payment check failure in the invoice.
//
// Do not change this message as it's used in plain text across the C2S RPC
// interface.
var ErrRMInvoicePayment = errors.New("invoice payment error on RM push")

// ErrUnableToGenerateInvoice is generated on clients when they are unable to
// generate an invoice for a remote peer.
//
// Do not change this message as it's used in plain text across the C2C RPC
// interface.
var ErrUnableToGenerateInvoice = errors.New("unable to generate payment invoice")

const errUnpaidSubscriptionRVMsg = "unpaid subscription to RV"

// ErrUnpaidSubscriptionRV is an error returned while attempting to subscribe
// to an RV that wasn't previously paid.
type ErrUnpaidSubscriptionRV ratchet.RVPoint

func (err ErrUnpaidSubscriptionRV) Error() string {
	return fmt.Sprintf("%s %s", errUnpaidSubscriptionRVMsg, ratchet.RVPoint(err).String())
}

func (err ErrUnpaidSubscriptionRV) Is(other error) bool {
	_, ok := other.(ErrUnpaidSubscriptionRV)
	return ok
}

// errUnpaidSubscriptionRVRegexp is a regexp that can parse messages generated
// by instances of ErrUnpaidSubscriptionRV.
var errUnpaidSubscriptionRVRegexp = regexp.MustCompile(fmt.Sprintf("^%s ([0-9a-fA-F]{64})$", errUnpaidSubscriptionRVMsg))

// ParseErrUnpaidSubscriptionRV attempts to parse a string as an
// ErrUnpaidSubscriptionRV. If this fails, it returns a nil error. If it succeeds,
// the return value is an instance of ErrUnpaidSubscriptionRV.
func ParseErrUnpaidSubscriptionRV(s string) error {
	matches := errUnpaidSubscriptionRVRegexp.FindStringSubmatch(s)
	if matches == nil {
		return nil
	}

	if len(matches) != 2 {
		return nil
	}

	var rv ratchet.RVPoint
	if err := rv.FromString(matches[1]); err != nil {
		return nil
	}

	return ErrUnpaidSubscriptionRV(rv)
}
