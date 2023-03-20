package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/slog"
)

// Client tip payment flow is:
//
//          Alice                                    Bob
//         -------                                  -----
//
//   TipUser()
//       \-------- RMGetInvoice -->
//
//                                            handleGetInvoice()
//                               <-- RMInvoice --------/
//
//   handleInvoice()
//     (out-of-band payment)

// requestTipInvoice sends a request to the remote user to send back an invoice.
func (c *Client) requestTipInvoice(ru *RemoteUser, milliAmt uint64, tag int32) error {
	var ta clientdb.TipUserAttempt
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		ta, err = c.db.ReadTipAttempt(tx, ru.ID(), tag)
		if err != nil {
			return err
		}

		// Double check the invoice is still needed.
		if ta.LastInvoice != "" {
			return fmt.Errorf("already have invoice")
		}
		if ta.Attempts > ta.MaxAttempts {
			return fmt.Errorf("max attempts at tipping user reached")
		}
		if ta.Completed != nil {
			return fmt.Errorf("user already tipped")
		}

		ta.Attempts += 1
		ta.InvoiceRequested = time.Now()
		return c.db.StoreTipUserAttempt(tx, ta)
	})
	if err != nil {
		return err
	}

	if ta.Attempts > ta.MaxAttempts {
		// Notify giving up on attempting to tip user.
		err := fmt.Errorf("attempt %d > max attempts %d to fetch invoice",
			ta.Attempts, ta.MaxAttempts)
		ru.log.Infof("Tip attempt (tag %d) failed to request invoice due to %v. Giving up.",
			ta.Tag, err)
		c.ntfns.notifyTipAttemptProgress(ru, int64(ta.MilliAtoms), false,
			int(ta.Attempts), err, false)

		// Consider this as handled, so don't return an error.
		return nil
	}

	getInvoice := rpc.RMGetInvoice{
		PayScheme:  c.pc.PayScheme(),
		MilliAtoms: milliAmt,
		Tag:        uint32(tag),
	}

	ru.log.Debugf("Attempt %d/%d at requesting invoice for tip payment of "+
		"%.8f DCR (tag %d)", ta.Attempts, ta.MaxAttempts,
		float64(milliAmt)/1e11, tag)

	payEvent := "gettipinvoice"
	return ru.sendRM(getInvoice, payEvent)
}

// TipUser starts an attempt to tip the user some amount of dcr. This dispatches
// a request for an invoice to the remote user, which once received will be
// paid.
//
// By the time the invoice is received, the local client may or may not have
// enough funds to pay for it, so multiple attempts will be made to fetch and
// pay for an invoice.
func (c *Client) TipUser(uid UserID, dcrAmount float64, maxAttempts int32) error {
	if dcrAmount <= 0 {
		return fmt.Errorf("cannot pay user %f <= 0", dcrAmount)
	}

	if maxAttempts <= 0 {
		return fmt.Errorf("maxAttempts %d <= 0", maxAttempts)
	}

	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}

	amt, err := dcrutil.NewAmount(dcrAmount)
	if err != nil {
		return err
	}
	milliAmt := uint64(amt) * 1e3

	var tag int32
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		tag = c.db.UnusedTipUserTag(tx, uid)
		ta := clientdb.TipUserAttempt{
			UID:              uid,
			Tag:              tag,
			MilliAtoms:       milliAmt,
			Created:          time.Now(),
			Attempts:         0,
			MaxAttempts:      maxAttempts,
			InvoiceRequested: time.Now(),
		}
		return c.db.StoreTipUserAttempt(tx, ta)
	})
	if err != nil {
		return err
	}

	if ru.log.Level() <= slog.LevelDebug {
		ru.log.Infof("Requesting invoice to tip user %d MAtoms (tag %d)", milliAmt,
			tag)
	} else {
		ru.log.Infof("Requesting invoice to tip user %.8f DCR (tag %d)", dcrAmount,
			tag)
	}

	return c.requestTipInvoice(ru, milliAmt, tag)
}

func (c *Client) handleGetInvoice(ru *RemoteUser, getInvoice rpc.RMGetInvoice) error {

	// Helper to reply with an error.
	replyWithErr := func(err error) {
		errStr := err.Error()
		reply := rpc.RMInvoice{
			Tag:   getInvoice.Tag,
			Error: &errStr,
		}
		errSend := ru.sendRM(reply, "getinvoicereply")
		if errSend != nil && !errors.Is(errSend, clientintf.ErrSubsysExiting) {
			ru.log.Warnf("Error sending FTPayForGetReply: %v", errSend)
		}
	}

	clientPS := c.pc.PayScheme()
	if getInvoice.PayScheme != clientPS {
		err := fmt.Errorf("requested pay scheme %q not same as %q",
			getInvoice.PayScheme, clientPS)
		replyWithErr(err)
		return err
	}

	cb := func(receivedMAtoms int64) {
		dcrAmt := float64(receivedMAtoms) / 1e11
		ru.log.Infof("Received %f DCR as tip", dcrAmt)
		err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
			return c.db.RecordUserPayEvent(tx, ru.ID(), "tip", receivedMAtoms, 0)
		})
		if err != nil {
			c.log.Warnf("Error while updating DB to store tip payment status: %v", err)
		}
		if c.cfg.TipReceived != nil {
			c.cfg.TipReceived(ru, dcrAmt)
		}
	}

	amountMAtoms := int64(getInvoice.MilliAtoms)
	dcrAmount := float64(amountMAtoms) / 1e11
	inv, err := c.pc.GetInvoice(c.ctx, amountMAtoms, cb)
	if err != nil {
		c.ntfns.notifyInvoiceGenFailed(ru, dcrAmount, err)
		replyWithErr(rpc.ErrUnableToGenerateInvoice)
		ru.log.Warnf("Unable to generate invoice for %.8f DCR: %v",
			dcrAmount, err)

		// The prior notification and logging will alert users about
		// the failure to generate the invoice, but otherwise this call
		// was handled, so return nil instead of the error.
		return nil
	}

	if ru.log.Level() <= slog.LevelDebug {
		ru.log.Debugf("Generated invoice for tip of %.8f DCR: %s",
			dcrAmount, inv)
	} else {
		ru.log.Infof("Generated invoice for tip of %.8f DCR",
			dcrAmount)
	}

	// Send reply.
	reply := rpc.RMInvoice{
		Invoice: inv,
		Tag:     getInvoice.Tag,
	}
	return ru.sendRM(reply, "getinvoicereply")
}

// scheduleRequestTipAttemptInvoice schedules a new attempt at requesting
// an invoice to complete the specified tip attempt.
func (c *Client) scheduleRequestTipAttemptInvoice(ta clientdb.TipUserAttempt) {
	// TODO: schedule this better, instead of using a goroutine per tip
	// attempt.
	go func() {
		now := time.Now()
		delay := c.cfg.TipUserReRequestInvoiceDelay - now.Sub(ta.InvoiceRequested)
		c.log.Debugf("Scheduling re-request for invoice to tip user "+
			"(tag %d) after %s", ta.Tag, delay)

		select {
		case <-time.After(delay):
		case <-c.ctx.Done():
			return
		}

		ru, err := c.UserByID(ta.UID)
		if err != nil {
			// Blocked user during this delay.
			c.log.Warnf("Unable to fetch user to request tip invoice: %v", err)
			return
		}

		err = c.requestTipInvoice(ru, ta.MilliAtoms, ta.Tag)
		if err != nil {
			ru.log.Errorf("Unable to request rescheduled tip invoice: %v", err)
		}
	}()
}

// handleTipUserPaymentResult takes the appropriate action after a payment
// attempt for a TipUser request.
func (c *Client) handleTipUserPaymentResult(ru *RemoteUser, tag int32, payErr error, fees int64) {
	if errors.Is(payErr, context.Canceled) {
		// Cancelation isn't a fatal error.
		return
	}

	var ta clientdb.TipUserAttempt
	dbErr := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		ta, err = c.db.ReadTipAttempt(tx, ru.ID(), tag)
		if err != nil {
			return err
		}

		if ta.Completed != nil {
			return fmt.Errorf("tip attempt already completed when " +
				"trying to handle payment result")
		}

		if payErr == nil {
			// Amount is negative because we're paying an invoice.
			payEvent := "paytip"
			amount := -int64(ta.MilliAtoms)
			fees := -fees
			if err := c.db.RecordUserPayEvent(tx, ru.ID(), payEvent, amount, fees); err != nil {
				return err
			}
			now := time.Now()
			ta.Completed = &now
			ta.LastInvoiceError = nil
		} else {
			errMsg := payErr.Error()
			ta.LastInvoiceError = &errMsg
			ta.LastInvoice = ""
			ta.PaymentAttempt = nil
		}

		return c.db.StoreTipUserAttempt(tx, ta)
	})

	if dbErr != nil {
		ru.log.Errorf("Unable to store tip payment update: %v", dbErr)
	}

	if ta.Completed != nil {
		// Notify tip completed successfully.
		ru.log.Infof("Completed tip user attempt (tag %d) for %.8f DCR",
			ta.Tag, float64(ta.MilliAtoms)/1e11)
		c.ntfns.notifyTipAttemptProgress(ru, int64(ta.MilliAtoms), true,
			int(ta.Attempts), nil, false)
	} else if ta.Attempts < ta.MaxAttempts {
		// Notify tip attempt failed and will be tried again.
		ru.log.Infof("Tip attempt (tag %d) %d/%d failed payment due to %q. Requesting new invoice.",
			ta.Tag, ta.Attempts, ta.MaxAttempts, *ta.LastInvoiceError)
		c.ntfns.notifyTipAttemptProgress(ru, int64(ta.MilliAtoms), false,
			int(ta.Attempts), payErr, true)

		// Schedule request for a new invoice.
		c.scheduleRequestTipAttemptInvoice(ta)
	} else {
		// Notify giving up on attempting to tip user.
		ru.log.Infof("Tip attempt (tag %d) %d/%d failed payment due to %q. Giving up.",
			ta.Tag, ta.Attempts, ta.MaxAttempts, *ta.LastInvoiceError)
		c.ntfns.notifyTipAttemptProgress(ru, int64(ta.MilliAtoms), false,
			int(ta.Attempts), payErr, false)
	}
}

// payTipInvoice starts the payment process for a received invoice.
func (c *Client) payTipInvoice(ru *RemoteUser, invoice string, amtMAtoms int64, tag int32) {
	fees, payErr := c.pc.PayInvoice(c.ctx, invoice)
	c.handleTipUserPaymentResult(ru, tag, payErr, fees)
}

// handleInvoice handles received RMInvoice calls.
func (c *Client) handleInvoice(ru *RemoteUser, invoice rpc.RMInvoice) error {

	// Decode invoice to determine if it's valid.
	decoded, decodedErr := c.pc.DecodeInvoice(c.ctx, invoice.Invoice)

	var ta clientdb.TipUserAttempt
	var invoiceErr error
	errIgnore := errors.New("") // guard for ignorable errors.
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		ta, err = c.db.ReadTipAttempt(tx, ru.ID(), int32(invoice.Tag))
		if err != nil {
			return err
		}

		// Determine if attempt should be recorded or discarded.
		if ta.Completed != nil {
			return fmt.Errorf("already completed payment%w", errIgnore)
		}
		if ta.PaymentAttempt != nil {
			return fmt.Errorf("payment attempt already in-flight%w", errIgnore)
		}
		if ta.Attempts > ta.MaxAttempts {
			return fmt.Errorf("attempts %d > max attempts %d%w",
				ta.Attempts, ta.MaxAttempts, errIgnore)
		}
		if ta.PaymentAttempt != nil {
			return fmt.Errorf("still waiting for payment attempt from %s "+
				"to complete%w", *ta.PaymentAttempt, errIgnore)
		}

		// Determine if invoice is good to attempt payment.
		if invoice.Error != nil {
			invoiceErr = errors.New(*invoice.Error)
		}
		if invoiceErr == nil && ta.LastInvoice != "" {
			invoiceErr = fmt.Errorf("already have previous invoice for this same tip attempt")
		}
		if invoiceErr == nil && decodedErr != nil {
			invoiceErr = decodedErr
		}
		if invoiceErr == nil && decoded.MAtoms != int64(ta.MilliAtoms) {
			invoiceErr = fmt.Errorf("milliatoms requested in invoice (%d) "+
				"different than milliatoms originally requested (%d)",
				decoded.MAtoms, ta.MilliAtoms)
		}
		if invoiceErr == nil && decoded.IsExpired(time.Second) {
			invoiceErr = fmt.Errorf("invoice received is already expired")
		}
		now := time.Now()
		if ta.Created.Before(now.Add(-c.cfg.TipUserMaxLifetime)) {
			// Modify so that no more attempts are made, as the
			// max lifetime for payment was reached.
			if invoiceErr == nil {
				invoiceErr = fmt.Errorf("invoice created %s ago "+
					"which is greater than max lifetime %s",
					now.Sub(ta.Created).Truncate(time.Second),
					c.cfg.TipUserMaxLifetime)
			}
			ta.Attempts = ta.MaxAttempts
		}

		if invoiceErr == nil {
			if ta.LastInvoice != "" {
				ta.PrevInvoices = append(ta.PrevInvoices, ta.LastInvoice)
			}

			ta.LastInvoice = invoice.Invoice
			ta.PaymentAttempt = &now
		} else {
			ta.LastInvoice = ""
			errMsg := invoiceErr.Error()
			ta.LastInvoiceError = &errMsg
		}

		return c.db.StoreTipUserAttempt(tx, ta)
	})
	if errors.Is(err, errIgnore) {
		// Could be the user was offline for a long time and we sent
		// multiple attempts to request invoice and received back
		// multiple invoices, so log at a lower level than ERR.
		ru.log.Warnf("Not paying received invoice for TipUser attempt (tag %d) "+
			"due to: %v", ta.Tag, err)
		return nil
	}
	if err != nil {
		return err
	}

	if invoiceErr != nil && ta.Attempts < ta.MaxAttempts {
		// Notify this attempt failed and will be tried again.
		ru.log.Infof("Tip attempt (tag %d) %d/%d failed to obtain "+
			"invoice due to %q. Requesting new invoice.",
			ta.Tag, ta.Attempts, ta.MaxAttempts, invoiceErr)
		c.ntfns.notifyTipAttemptProgress(ru, int64(ta.MilliAtoms), false,
			int(ta.Attempts), invoiceErr, true)

		c.scheduleRequestTipAttemptInvoice(ta)
	} else if invoiceErr != nil {
		// Notify giving up on attempting to tip user.
		ru.log.Warnf("Tip attempt (tag %d) %d/%d failed to obtain invoice "+
			"due to %q. Giving up.", ta.Tag, ta.Attempts,
			ta.MaxAttempts, *ta.LastInvoiceError)
		c.ntfns.notifyTipAttemptProgress(ru, int64(ta.MilliAtoms), false,
			int(ta.Attempts), invoiceErr, false)
	} else {
		// Pay for invoice.
		go c.payTipInvoice(ru, invoice.Invoice, decoded.MAtoms, int32(invoice.Tag))
	}

	return nil
}

func (c *Client) ListTipUserAttempts(uid UserID) ([]clientdb.TipUserAttempt, error) {
	var res []clientdb.TipUserAttempt
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.ListTipUserAttempts(tx, uid)
		return err
	})
	return res, err
}

// restartTipUserPayment picks up the payment status for a payment that was
// initiated on a prior client run.
func (c *Client) restartTipUserPayment(ctx context.Context, ru *RemoteUser, ta clientdb.TipUserAttempt) {
	// This may block for a _long_ time.
	fees, payErr := c.pc.IsPaymentCompleted(ctx, ta.LastInvoice)
	c.handleTipUserPaymentResult(ru, ta.Tag, payErr, fees)
}

// restartTipUserAttempts restarts all existing TipUser requests, checking on
// their progress.
func (c *Client) restartTipUserAttempts(ctx context.Context) error {

	// Any tip attempts already restarted between now and the loop below
	// were done so in response to an RMInvoice received, so we should
	// ignore those for the purposes of restarting.
	startTime := time.Now()
	c.log.Tracef("Will restart TipUser attempts with start time %s", startTime)
	<-c.abLoaded

	// This is called on client start, so wait until the first set of RV
	// subscriptions is done, then sleep for a bit to allow any invoices
	// sent while the client was offline to be fetched.
	select {
	case <-c.firstSubDone:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-time.After(c.cfg.TipUserRestartDelay):
	case <-ctx.Done():
		return ctx.Err()
	}

	c.log.Debugf("Restarting TipUser attempts with start time %s", startTime)

	requestInvoiceDeadline := time.Now().Add(-c.cfg.TipUserReRequestInvoiceDelay)

	// Restart any lingering attempts.
	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		oldAttempts, err := c.db.ListTipUserAttemptsToRetry(tx, startTime,
			c.cfg.TipUserMaxLifetime)
		if err != nil {
			return err
		}

		for _, ta := range oldAttempts {
			ta := ta
			ru, err := c.UserByID(ta.UID)
			if err != nil {
				continue
			}

			if ta.LastInvoice != "" {
				// Pickup the payment.
				go c.restartTipUserPayment(ctx, ru, ta)
			} else if ta.Attempts >= ta.MaxAttempts {
				// No payment and already sent all invoice
				// requests. Nothing else to do but wait if
				// a valid invoice will still arrive.
				ru.log.Debugf("Already sent %d/%d attempts to "+
					"request invoice for tipping user (tag %d)",
					ta.Attempts, ta.MaxAttempts, ta.Tag)
			} else if ta.InvoiceRequested.Before(requestInvoiceDeadline) {
				// Enough time passed to request a new invoice.
				go func() {
					err := c.requestTipInvoice(ru, ta.MilliAtoms, ta.Tag)
					if err != nil {
						ru.log.Errorf("Unable to send request tip RM: %v", err)
					}
				}()
			} else {
				// Not enough time passed to re-request this
				// invoice, so schedule it for the future.
				c.scheduleRequestTipAttemptInvoice(ta)
			}
		}

		return nil
	})
}

// ListPaymentStats returns the general payment stats for all users.
func (c *Client) ListPaymentStats() (map[UserID]clientdb.UserPayStats, error) {
	var res map[UserID]clientdb.UserPayStats
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.ListPayStats(tx)
		return err
	})
	return res, err
}

// SummarizeUserPayStats reads the payment stats file of the given user and
// returns a summary of what the payments were made and received in relation to
// the specified user.
func (c *Client) SummarizeUserPayStats(uid UserID) ([]clientdb.PayStatsSummary, error) {
	var res []clientdb.PayStatsSummary
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.SummarizeUserPayStats(tx, uid)
		return err
	})
	return res, err

}

// ClearPayStats removes the payment stats associated with the given user. If
// nil is passed, then the payment stats for all users are cleared.
func (c *Client) ClearPayStats(uid *UserID) error {
	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.ClearPayStats(tx, uid)
	})
}
