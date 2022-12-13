package rpc

import (
	"fmt"
	"regexp"

	"github.com/companyzero/bisonrelay/ratchet"
)

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
