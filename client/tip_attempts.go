package client

import (
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
)

type tipUserAttemptAction string

const (
	actionCancel         tipUserAttemptAction = "cancel"
	actionExpire         tipUserAttemptAction = "expire"
	actionComplete       tipUserAttemptAction = "complete"
	actionRequestInvoice tipUserAttemptAction = "request_invoice"
	actionCheckPayment   tipUserAttemptAction = "check_payment"
	actionAttemptPayment tipUserAttemptAction = "attempt_payment"
)

// runningTipAttempt tracks the next action and the time when it should be taken
// for a given TipUser attempt.
type runningTipAttempt struct {
	tag            int32
	uid            clientintf.UserID
	nextAction     tipUserAttemptAction
	nextActionTime time.Time
}

// tipAttemptsList maintains a list of per-user running attempts at tipping.
type tipAttemptsList struct {
	requestInvoiceDeadline time.Duration
	maxLifetimeDuration    time.Duration
	payRetryDelayFactor    time.Duration

	m map[clientintf.UserID]runningTipAttempt
}

func newTipAttemptsList(requestInvoiceDeadline, maxLifetimeDuration, payRetryDelayFactor time.Duration) *tipAttemptsList {
	return &tipAttemptsList{
		requestInvoiceDeadline: requestInvoiceDeadline,
		maxLifetimeDuration:    maxLifetimeDuration,
		payRetryDelayFactor:    payRetryDelayFactor,
		m:                      map[clientintf.UserID]runningTipAttempt{},
	}
}

// addTipAttempt starts to track ta in the list. Returns the tip attempt that
// is actually being tracked.
func (tal *tipAttemptsList) addTipAttempt(ta *clientdb.TipUserAttempt) runningTipAttempt {
	if rta, ok := tal.m[ta.UID]; ok {
		// Overwrite only if the tag is the same
		if ta.Tag != rta.tag {
			panic("trying to add another tip attempt for the same user")
		}
	}
	na, nat := tal.determineTipAttemptAction(ta, false)
	rta := runningTipAttempt{
		tag:            ta.Tag,
		nextActionTime: nat,
		nextAction:     na,
		uid:            ta.UID,
	}
	tal.m[ta.UID] = rta
	return rta
}

// modifyTipAttempt updates an existing running tip attempt. It MUST already have
// been added to the list.
func (tal *tipAttemptsList) modifyTipAttempt(ta *clientdb.TipUserAttempt, paying bool) runningTipAttempt {
	if rta, ok := tal.m[ta.UID]; ok {
		if ta.Tag != rta.tag {
			panic("cannot modify tip attempt of different tag")
		}
	} else {
		panic("cannot modify tip attempt of inexistent user")
	}
	na, nat := tal.determineTipAttemptAction(ta, paying)
	rta := runningTipAttempt{
		tag:            ta.Tag,
		nextActionTime: nat,
		nextAction:     na,
		uid:            ta.UID,
	}
	tal.m[ta.UID] = rta
	return rta
}

// currentAttemptForUserIs returns true if the running attempt for the user is
// the given tag.
func (tal *tipAttemptsList) currentAttemptForUserIs(uid UserID, tag int32) bool {
	rta, ok := tal.m[uid]
	if !ok {
		return false
	}
	return rta.tag == tag
}

// hasAttemptForUser returns true if there is an attempt at tipping for the
// given user.
func (tal *tipAttemptsList) hasAttemptForUser(uid UserID) bool {
	_, ok := tal.m[uid]
	return ok
}

// delTipAttempt removes the ta from the list of tracked attempts.
func (tal *tipAttemptsList) delTipAttempt(uid clientintf.UserID, tag int32) {
	rta, ok := tal.m[uid]
	if !ok {
		return
	}

	if rta.tag == tag {
		delete(tal.m, uid)
	}
}

// timeToNextAction returns the time until the next action needs to be taken.
// Returns false if no action is needed.
func (tal *tipAttemptsList) timeToNextAction() (time.Duration, bool) {
	var earliest time.Time
	if len(tal.m) == 0 {
		return 0, false
	}

	for _, rta := range tal.m {
		if earliest.IsZero() || rta.nextActionTime.Before(earliest) {
			earliest = rta.nextActionTime
		}
	}
	return time.Until(earliest), true //earliest.Sub(time.Now()), true
}

// determineTipAttemptAction determines what is the next action and the time
// to take it for a given TipUserAttempt.
//
//nolint:durationcheck
func (tal *tipAttemptsList) determineTipAttemptAction(ta *clientdb.TipUserAttempt,
	paying bool) (tipUserAttemptAction, time.Time) {

	// Helper to return a time that triggers an action right now. This returns
	// a time in the past, based on the time the tip attempt was created so
	// that older attempts have a lower (i.e. earlier) action time for
	// sorting purposes.
	actNow := func() time.Time {
		return ta.Created.Add(-time.Minute).In(time.Local)
	}

	if ta.Attempts > ta.MaxAttempts {
		// Max attempts made.
		return actionCancel, actNow()
	}

	if ta.Completed != nil {
		// Already completed.
		return actionComplete, actNow()
	}

	// expireDeadline is when the entire tip attempt expires.
	expireDeadline := ta.Created.Add(tal.maxLifetimeDuration).In(time.Local)
	if expireDeadline.Before(time.Now()) {
		// Expired.
		return actionExpire, expireDeadline
	}

	if paying {
		// Paying (or waiting payment check to complete). Only thing to
		// do is wait to expire.
		return actionExpire, expireDeadline
	}

	if ta.LastInvoice != "" {
		if ta.PaymentAttempt == nil {
			if ta.PaymentAttemptFailed == nil {
				// First payment attempt, act immediately.
				return actionAttemptPayment, actNow()
			}

			// constant delay increase for repeated retriable payment attempts.
			delay := time.Duration(ta.PaymentAttemptCount) * tal.payRetryDelayFactor
			nextAttemptTime := (*ta.PaymentAttemptFailed).Add(delay).In(time.Local)
			return actionAttemptPayment, nextAttemptTime
		}

		// Check payment attempt.
		return actionCheckPayment, actNow()
	}

	if ta.LastInvoiceError != nil && ta.Attempts == ta.MaxAttempts {
		// Last attempt at requesting invoice errored. Cancel
		// tipping attempt.
		return actionCancel, actNow()
	}

	if ta.LastInvoiceError != nil {
		// Had an error paying or requesting an invoice. Wait until
		// it's time to try and request a new invoice.
		return actionRequestInvoice, ta.InvoiceRequested.Add(tal.requestInvoiceDeadline).In(time.Local)
	}

	if ta.InvoiceRequested == nil {
		// No record of when an invoice was requested, request one now.
		return actionRequestInvoice, actNow()
	}

	// Invoice requested but not received yet. Expire after the lifetime of
	// the tip attempt elapses.
	return actionExpire, expireDeadline
}

// actionsForNow returns all actions that need to be taken now (i.e. all actions
// with nextActionTime < now()). At most one action per user is returned.
func (tal *tipAttemptsList) actionsForNow() []runningTipAttempt {
	now := time.Now()
	var res []runningTipAttempt
	for _, rta := range tal.m {
		if rta.nextActionTime.Before(now) {
			res = append(res, rta)
		}
	}
	return res
}

// currentAttempts returns a list with the currently running tip user attempts.
func (tal *tipAttemptsList) currentAttempts() []RunningTipUserAttempt {
	res := make([]RunningTipUserAttempt, len(tal.m))
	i := 0
	for _, rta := range tal.m {
		res[i] = RunningTipUserAttempt{
			UID:            rta.uid,
			Tag:            rta.tag,
			NextAction:     string(rta.nextAction),
			NextActionTime: rta.nextActionTime,
		}
	}
	return res
}
