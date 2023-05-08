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

	// Wait until the main tip processing goroutine has performed its
	// startup.
	select {
	case <-c.ctx.Done():
		return fmt.Errorf("client is quitting")
	case <-c.tipAttemptsRunning:
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
	var ta clientdb.TipUserAttempt
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		tag = c.db.UnusedTipUserTag(tx, uid)
		ta = clientdb.TipUserAttempt{
			UID:         uid,
			Tag:         tag,
			MilliAtoms:  milliAmt,
			Created:     time.Now(),
			Attempts:    0,
			MaxAttempts: maxAttempts,
		}
		return c.db.StoreTipUserAttempt(tx, ta)
	})
	if err != nil {
		return err
	}

	if ru.log.Level() <= slog.LevelDebug {
		ru.log.Infof("Starting tip attempt of %d MAtoms (tag %d)", milliAmt,
			tag)
	} else {
		ru.log.Infof("Starting tip attempt of %.8f DCR (tag %d)", dcrAmount,
			tag)
	}

	// Send for the main tip attempts run() goroutine for scheduling.
	select {
	case c.tipAttemptsChan <- &ta:
	case <-c.ctx.Done():
		return fmt.Errorf("cannot send tip when client is shutting down")
	}
	return nil
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

	c.ntfns.notifyTipUserInvoiceGenerated(ru, getInvoice.Tag, inv)

	// Send reply.
	reply := rpc.RMInvoice{
		Invoice: inv,
		Tag:     getInvoice.Tag,
	}
	return ru.sendRM(reply, "getinvoicereply")
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
		return
	}

	// When there's an error and it's not yet the last attempt, notify
	// the UI.
	if ta.LastInvoiceError != nil && ta.Attempts < ta.MaxAttempts {
		invoiceErr := errors.New(*ta.LastInvoiceError)
		ru.log.Debugf("Attempt %d/%d at tip tag %d failed payment "+
			"due to %v", ta.Attempts, ta.MaxAttempts, ta.Tag, invoiceErr)
		c.ntfns.notifyTipAttemptProgress(ru, int64(ta.MilliAtoms), false,
			int(ta.Attempts), invoiceErr, true)
	}

	// Send to main tip goroutine for handling.
	select {
	case c.tipAttemptsChan <- &ta:
	case <-c.ctx.Done():
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
		if ta.LastInvoice != "" {
			return fmt.Errorf("already have previous invoice for this same tip attempt%w",
				errIgnore)
		}

		// Determine if invoice is good to attempt payment.
		if invoice.Error != nil {
			invoiceErr = errors.New(*invoice.Error)
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
		if invoiceErr == nil && ta.Created.Before(now.Add(-c.cfg.TipUserMaxLifetime)) {
			invoiceErr = fmt.Errorf("invoice created %s ago "+
				"which is greater than max lifetime %s",
				now.Sub(ta.Created).Truncate(time.Second),
				c.cfg.TipUserMaxLifetime)
		}

		if invoiceErr == nil {
			if ta.LastInvoice != "" {
				ta.PrevInvoices = append(ta.PrevInvoices, ta.LastInvoice)
			}

			ta.LastInvoice = invoice.Invoice
			ta.PaymentAttempt = nil
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

	// When there's an error and it's not yet the last attempt, notify
	// the UI.
	if ta.LastInvoiceError != nil && ta.Attempts < ta.MaxAttempts {
		invoiceErr := errors.New(*ta.LastInvoiceError)
		ru.log.Debugf("Attempt %d/%d at tip tag %d failed to fetch invoice "+
			"due to %v", ta.Attempts, ta.MaxAttempts, ta.Tag, invoiceErr)
		c.ntfns.notifyTipAttemptProgress(ru, int64(ta.MilliAtoms), false,
			int(ta.Attempts), invoiceErr, true)
	}

	// Send to main tip payment run() goroutine.
	select {
	case c.tipAttemptsChan <- &ta:
	case <-c.ctx.Done():
		return c.ctx.Err()
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

// RunningTipUserAttempt tracks information about a running attempt at tipping
// a user.
type RunningTipUserAttempt struct {
	Tag            int32
	UID            clientintf.UserID
	NextAction     string
	NextActionTime time.Time
}

// ListRunningTipUserAttempts lists the currently running attempts at tipping
// remote users.
func (c *Client) ListRunningTipUserAttempts() []RunningTipUserAttempt {
	ch := make(chan []RunningTipUserAttempt, 1)
	select {
	case c.listRunningTipAttemptsChan <- ch:
	case <-c.ctx.Done():
		return nil
	}

	select {
	case res := <-ch:
		return res
	case <-c.ctx.Done():
		return nil
	}
}

// restartTipUserPayment picks up the payment status for a payment that was
// initiated on a prior client run.
func (c *Client) restartTipUserPayment(ctx context.Context, ru *RemoteUser, ta clientdb.TipUserAttempt) {
	// This may block for a _long_ time.
	fees, payErr := c.pc.IsPaymentCompleted(ctx, ta.LastInvoice)
	c.handleTipUserPaymentResult(ru, ta.Tag, payErr, fees)
}

// takeTipAttemptAction executes the TipUser action specified in rta. This is
// the main dispatcher for the individual actions needed to tip an user.
func (c *Client) takeTipAttemptAction(ctx context.Context, attempts *tipAttemptsList,
	rta runningTipAttempt) error {

	ru, err := c.UserByID(rta.uid)
	if err != nil {
		return err
	}

	// Update the DB with the status of the action being in progress.
	var ta clientdb.TipUserAttempt
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) (runErr error) {

		// Helper function that will load the next tip user attempt
		// stored in the db for the user of the passed rta.
		loadNextTipUserAttempt := func() {
			// The action to take is final (cancel, complete, expire). So
			// look for the next tip attempt for this user.
			nextTA, err := c.db.NextTipAttemptToRetryForUser(tx, rta.uid,
				c.cfg.TipUserMaxLifetime)
			if errors.Is(err, clientdb.ErrNotFound) {
				// No more attempts to add for this user.
				ru.log.Trace("No more tip attempts for user to progress")
				return
			}
			if err != nil {
				ru.log.Errorf("Unable to load next user tip attempt: %v", err)
				return
			}

			nextRTA := attempts.addTipAttempt(&nextTA)
			ru.log.Debugf("Switching to tip attempt %d action %s", nextRTA.tag,
				nextRTA.nextAction)
		}

		// Load this tip attempt.
		var err error
		ta, err = c.db.ReadTipAttempt(tx, rta.uid, rta.tag)
		if errors.Is(err, clientdb.ErrNotFound) {
			// Attempt was removed. Try next one.
			attempts.delTipAttempt(rta.uid, rta.tag)
			loadNextTipUserAttempt()
			return nil
		}
		if err != nil {
			return err
		}

		// Double check the action to take is still the same one.
		checkAction, checkTime := attempts.determineTipAttemptAction(&ta, false)
		if checkAction != rta.nextAction || !checkTime.Before(time.Now()) {
			// Something happened that changed the action to take.
			// Update in the list of attempts and rely on the run()
			// loop to take the appropriate action.
			attempts.modifyTipAttempt(&ta, false)
			ru.log.Debugf("Skipping tip attempt %d action %s due to having "+
				"changed to %s", rta.tag, rta.nextAction, checkAction)
			rta.nextAction = "" // Prevent action after dbUpdate() returns.
			return nil
		}

		ru.log.Debugf("Taking tip attempt %d action %s", rta.tag, rta.nextAction)

		switch rta.nextAction {
		case actionCancel, actionComplete, actionExpire:
			// Final action, load next tip user attempt.
			attempts.delTipAttempt(rta.uid, rta.tag)
			loadNextTipUserAttempt()

		case actionRequestInvoice:
			// Record the attempt to request invoice.
			ta.Attempts += 1
			now := time.Now()
			ta.InvoiceRequested = &now
			ta.LastInvoiceError = nil
			if err := c.db.StoreTipUserAttempt(tx, ta); err != nil {
				return err
			}
			nextRTA := attempts.modifyTipAttempt(&ta, false)
			ru.log.Tracef("Next tip attempt %d action %s at time %s",
				ta.Tag, nextRTA.nextAction, nextRTA.nextActionTime)
			return nil

		case actionAttemptPayment:
			// Record the attempt to pay the invoice.
			now := time.Now()
			ta.PaymentAttempt = &now
			ta.LastInvoiceError = nil
			if err := c.db.StoreTipUserAttempt(tx, ta); err != nil {
				return err
			}
			nextRTA := attempts.modifyTipAttempt(&ta, true)
			ru.log.Tracef("Next tip attempt %d action %s at time %s",
				ta.Tag, nextRTA.nextAction, nextRTA.nextActionTime)
			return nil

		case actionCheckPayment:
			// Will check for payment after dbUpdate() returns.
			nextRTA := attempts.modifyTipAttempt(&ta, true)
			ru.log.Tracef("Next tip attempt %d action %s at time %s",
				ta.Tag, nextRTA.nextAction, nextRTA.nextActionTime)
			return nil

		default:
			return fmt.Errorf("unknown next tip attempt action %s", rta.nextAction)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Take the action.
	switch rta.nextAction {
	case "":
		// Skip doing any actions.

	case actionCancel:
		// Notify giving up on attempting to tip user.
		var err error
		if ta.LastInvoiceError != nil {
			err = errors.New(*ta.LastInvoiceError)
		} else {
			err = fmt.Errorf("attempt %d >= max attempts %d to fetch invoice",
				ta.Attempts, ta.MaxAttempts)
		}
		ru.log.Infof("Tip attempt (tag %d) failed after %d attempts "+
			"to request invoice due to %v. Giving up.",
			ta.Tag, ta.Attempts, err)
		c.ntfns.notifyTipAttemptProgress(ru, int64(ta.MilliAtoms), false,
			int(ta.Attempts), err, false)

	case actionExpire:
		// Notify tip attempt expired.
		lifetime := time.Since(ta.Created)
		err := fmt.Errorf("expired %s after creation", lifetime)
		ru.log.Infof("Tip attempt (tag %d) expired %s after creation "+
			"with %d/%d attempts", ta.Tag, lifetime, ta.Attempts,
			ta.MaxAttempts)
		c.ntfns.notifyTipAttemptProgress(ru, int64(ta.MilliAtoms), false,
			int(ta.Attempts), err, false)

	case actionComplete:
		// Notify tip completed successfully.
		ru.log.Infof("Completed tip user attempt (tag %d) for %.8f DCR",
			ta.Tag, float64(ta.MilliAtoms)/1e11)
		c.ntfns.notifyTipAttemptProgress(ru, int64(ta.MilliAtoms), true,
			int(ta.Attempts), nil, false)

	case actionRequestInvoice:
		// Request a new invoice from remote user.
		getInvoice := rpc.RMGetInvoice{
			PayScheme:  c.pc.PayScheme(),
			MilliAtoms: ta.MilliAtoms,
			Tag:        uint32(ta.Tag),
		}

		ru.log.Debugf("Attempt %d/%d at requesting invoice for tip payment of "+
			"%.8f DCR (tag %d)", ta.Attempts, ta.MaxAttempts,
			float64(ta.MilliAtoms)/1e11, ta.Tag)

		payEvent := "gettipinvoice"
		return c.sendWithSendQ(payEvent, getInvoice, ru.ID())

	case actionAttemptPayment:
		// Attempt to pay fetched invoice.
		ru.log.Debugf("Attempt %d/%d at paying tip user invoice of "+
			"%.8f DCR (tag %d)", ta.Attempts, ta.MaxAttempts,
			float64(ta.MilliAtoms)/1e11, ta.Tag)
		go c.payTipInvoice(ru, ta.LastInvoice, int64(ta.MilliAtoms), ta.Tag)

	case actionCheckPayment:
		// Check if payment was completed.
		ru.log.Debugf("Verifying tip user pay attempt %d/%d of invoice of "+
			"%.8f DCR (tag %d) completed or failed", ta.Attempts,
			ta.MaxAttempts, float64(ta.MilliAtoms)/1e11, ta.Tag)
		go c.restartTipUserPayment(ctx, ru, ta)

	default:
		return fmt.Errorf("unknown action to take %s", rta.nextAction)
	}

	return nil
}

// runTipAttempts is the main goroutine that coordinates TipUser attempts.
func (c *Client) runTipAttempts(ctx context.Context) error {
	<-c.abLoaded

	attempts := newTipAttemptsList(c.cfg.TipUserReRequestInvoiceDelay,
		c.cfg.TipUserMaxLifetime)

	// This is called on client start, so wait until the first set of RV
	// subscriptions is done, then sleep for a bit to allow any invoices
	// sent while the client was offline to be fetched.
	//
	// Ignore any tip attempts sent to the channel during this wait, because
	// they will be loaded from the db directly.
	var timeCh <-chan time.Time
	restartDelayDone := false
	firstSubDone := c.firstSubDone
	for !restartDelayDone {
		select {
		case <-firstSubDone:
			timeCh = time.After(c.cfg.TipUserRestartDelay)
			firstSubDone = nil
		case <-timeCh:
			restartDelayDone = true
		case <-c.tipAttemptsChan:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Reload existing attempts
	var oldestTAs []clientdb.TipUserAttempt
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		oldestTAs, err = c.db.ListOldestValidTipUserAttempts(tx, c.cfg.TipUserMaxLifetime)
		return err
	})
	if err != nil {
		return err
	}
	for _, ta := range oldestTAs {
		attempts.addTipAttempt(&ta)
	}
	delayToAction, _ := attempts.timeToNextAction()
	actionTimer := time.NewTimer(delayToAction)

	if len(oldestTAs) > 0 && c.log.Level() >= slog.LevelInfo {
		c.log.Infof("Restarting %d tip attempts", len(oldestTAs))
	} else {
		c.log.Debugf("Restarting %d tip attempts. Delay to next action: %s",
			len(oldestTAs), delayToAction)
	}

	// Signal that TipUser() calls may start proceeding.
	close(c.tipAttemptsRunning)

	for {
		select {
		case ch := <-c.listRunningTipAttemptsChan:
			ch <- attempts.currentAttempts()

		case ta := <-c.tipAttemptsChan:
			switch {

			case attempts.currentAttemptForUserIs(ta.UID, ta.Tag):
				rta := attempts.modifyTipAttempt(ta, false)
				c.log.Tracef("Modifying running tip attempt for user %s tag %d "+
					"action %s action time %s", ta.UID, ta.Tag, rta.nextAction,
					rta.nextActionTime)

			case attempts.hasAttemptForUser(ta.UID):
				c.log.Tracef("Skipping tip attempt for user %s tag %d "+
					"due to already running attempt for same user",
					ta.UID, ta.Tag)
			default:
				rta := attempts.addTipAttempt(ta)
				c.log.Tracef("Received new tip attempt for user %s tag %d "+
					"action %s action time %s", ta.UID, ta.Tag, rta.nextAction,
					rta.nextActionTime)
			}

		case <-actionTimer.C:
			// Time to take actions.
			actions := attempts.actionsForNow()
			for _, rta := range actions {
				err := c.takeTipAttemptAction(ctx, attempts, rta)
				if err != nil {
					attempts.delTipAttempt(rta.uid, rta.tag)
					c.log.Errorf("Unable to take tip action for user %s"+
						" tag %d: %v", rta.uid, rta.tag, err)
				}
			}

		case <-ctx.Done():
			return ctx.Err()
		}

		delayToAction, hasActions := attempts.timeToNextAction()
		if hasActions {
			if !actionTimer.Stop() {
				select {
				case <-actionTimer.C:
				default:
				}
			}
			actionTimer.Reset(delayToAction)
			c.log.Tracef("Next tip user action in %s", delayToAction)
		} else {
			c.log.Trace("Idling tip user actions")
		}
	}
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
