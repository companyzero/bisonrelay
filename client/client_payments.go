package client

import (
	"errors"
	"fmt"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
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

// TipUser sends a tip with the given dcr amount to the remote user.
func (c *Client) TipUser(uid UserID, dcrAmount float64) error {
	if dcrAmount <= 0 {
		return fmt.Errorf("cannot pay user %f <= 0", dcrAmount)
	}

	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}

	milliAmt := uint64(dcrAmount * 1e11)

	replyChan := make(chan interface{})
	tag := ru.tagForMsg(replyChan)

	getInvoice := rpc.RMGetInvoice{
		PayScheme:  rpc.PaySchemeDCRLN,
		MilliAtoms: milliAmt,
		Tag:        tag,
	}

	ru.log.Debugf("Requesting invoice to pay user %.8f DCR", dcrAmount)

	payEvent := "gettipinvoice"
	err = ru.sendRM(getInvoice, payEvent)
	if err != nil {
		return err
	}

	// Wait until a reply with the invoice is received.
	var ir rpc.RMInvoice
	select {
	case res := <-replyChan:
		switch res := res.(type) {
		case rpc.RMInvoice:
			ir = res
		case error:
			return res
		default:
			return fmt.Errorf("unknown result type %T", res)
		}
	case <-c.ctx.Done():
		return errClientExiting
	}

	ru.log.Debugf("Got invoice to pay user: %q", ir.Invoice)

	// Decode invoice and verify.
	if ir.Invoice == "" {
		return nil
	}
	inv, err := c.pc.DecodeInvoice(c.ctx, ir.Invoice)
	if err != nil {
		return err
	}
	if inv.MAtoms < 0 || uint64(inv.MAtoms) > milliAmt {
		return fmt.Errorf("client generated invoice for amount different "+
			"then requested (%d vs %d matoms)", inv.MAtoms, milliAmt)
	}

	// Pay for invoice.
	fees, err := c.pc.PayInvoice(c.ctx, ir.Invoice)
	if err == nil {
		ru.log.Infof("Paid user %.8f DCR", float64(inv.MAtoms)/1e11)
		err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
			// Amount is negative because we're paying an invoice.
			payEvent := "paytip"
			amount := -inv.MAtoms
			fees := -fees
			return c.db.RecordUserPayEvent(tx, uid, payEvent, amount, fees)
		})
	}
	return err
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
		replyWithErr(fmt.Errorf("unable to generate payment invoice"))
		ru.log.Warnf("Unable to generate invoice for %.8f DCR: %v",
			dcrAmount, err)

		// The prior notification and logging will alert users about
		// the failure to generate the invoice, but otherwise this call
		// was handled, so return nil instead of the error.
		return nil
	}

	if ru.log.Level() <= slog.LevelDebug {
		ru.log.Debugf("Generated invoice for user payment of %.8f DCR: %s",
			dcrAmount, inv)
	} else {
		ru.log.Infof("Generated invoice for user payment of %.8f DCR",
			dcrAmount)
	}

	// Send reply.
	reply := rpc.RMInvoice{
		Invoice: inv,
		Tag:     getInvoice.Tag,
	}
	return ru.sendRM(reply, "getinvoicereply")
}

func (c *Client) handleInvoice(ru *RemoteUser, invoice rpc.RMInvoice) error {
	var v interface{}
	if invoice.Error != nil {
		v = errors.New(*invoice.Error)
	} else {
		v = invoice
	}
	return ru.replyToTaggedMsg(invoice.Tag, v)
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
