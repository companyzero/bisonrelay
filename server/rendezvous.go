// Copyright (c) 2016-2020 Company 0, LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/server/serverdb"
)

// maybePushRM pushes the given RM to the appropriate session if there is an
// online session that is expecting it.
func (z *ZKS) maybePushRM(r rpc.RouteMessage) {
	z.stats.rmsRecv.Add(1)

	z.Lock() // XXX LOOOL
	if sc, ok := z.subscribers[r.Rendezvous]; ok {
		sc.msgC <- r.Rendezvous
	}
	z.Unlock()
}

func (z *ZKS) handleRouteMessage(ctx context.Context, writer chan *RPCWrapper,
	msg rpc.Message, r rpc.RouteMessage, sc *sessionContext) error {

	sc.log.Tracef("handleRouteMessage tag %v", msg.Tag)

	// always reply from here on out (provided non fatal error)
	reply := RPCWrapper{
		Message: rpc.Message{
			Command: rpc.TaggedCmdRouteMessageReply,
			Tag:     msg.Tag,
		},
	}

	err := z.isRMPaid(ctx, &r, sc)
	if err != nil {
		// Reply with a generic invoice error.
		reply.Payload = rpc.RouteMessageReply{
			Error: rpc.ErrRMInvoicePayment.Error(),
		}
		writer <- &reply
		sc.log.Errorf("handleRouteMessage isRMPaid: %v", err)
		return nil
	}

	payload := rpc.RouteMessageReply{}
	var invoiceID string

	// Generate the next invoice that needs to be paid, if needed.
	switch z.settings.PayScheme {
	case rpc.PaySchemeFree:
		// Send a dummy invoice to avoid having the client re-request it.
		payload.NextInvoice = "free invoice"

	case rpc.PaySchemeDCRLN:
		var err error
		invoiceAction := rpc.InvoiceActionPush
		payload.NextInvoice, invoiceID, err = z.generateNextLNInvoice(ctx, sc, invoiceAction)
		if err != nil {
			sc.log.Errorf("handleRouteMessage generate invoice %v", err)
		} else {
			sc.log.Debugf("Generated invoice for action %q pay scheme %q: %s",
				invoiceAction, z.settings.PayScheme, invoiceID)
		}

	default:
		// Shouldn't happen unless it's in-development.
		return fmt.Errorf("unimplemented payment scheme %s", z.settings.PayScheme)
	}

	// Ensure RV is not empty.
	var emptyRV ratchet.RVPoint
	if r.Rendezvous == emptyRV {
		es := fmt.Sprintf("handleRouteMessage tag %v: invalid client",
			msg.Tag)
		payload.Error = es
		sc.log.Tracef(es)
		return nil
	}

	// Store on disk
	err = z.db.StorePayload(z.dbCtx, r.Rendezvous, r.Message, time.Now())
	if errors.Is(err, serverdb.ErrAlreadyStoredRV) {
		sc.log.Warnf("Attempt to store already stored RV %s", r.Rendezvous)
	} else if err != nil {
		payload.Error = err.Error()
		z.log.Warnf("handleRouteMessage tag %v: %v", msg.Tag, err)
	} else {
		sc.log.Debugf("Stored %d bytes at RV %s", len(r.Message), r.Rendezvous)

		// Deliver notification if there's an online session expecting
		// it.
		go z.maybePushRM(r)
	}

	// Send reply.
	reply.Payload = payload
	writer <- &reply
	return nil
}

func (z *ZKS) handleSubscribeRoutedMessages(ctx context.Context, msg rpc.Message,
	r rpc.SubscribeRoutedMessages, sc *sessionContext) error {

	var payload rpc.SubscribeRoutedMessagesReply

	if err := z.areSubsPaid(ctx, &r, sc); errors.Is(err, rpc.ErrUnpaidSubscriptionRV{}) {
		// This specific error (unpaid RV) is returned to the client and
		// then the client session is forcibly closed.
		payload.Error = err.Error()
		sc.writer <- &RPCWrapper{
			Message: rpc.Message{
				Command: rpc.TaggedCmdSubscribeRoutedMessagesReply,
				Tag:     msg.Tag,
			},
			Payload:              payload,
			CloseAfterWritingErr: err,
		}

		// Return nil instead of error because the session will be
		// automatically closed after the above reply message is sent.
		return nil
	} else if err != nil {
		return fmt.Errorf("areSubsPaid: %v", err)
	}

	// Create a subscription for messages in sessionSubscribe()
	sc.msgSetC <- r

	// Generate the next invoice that needs to be paid, if needed.
	switch z.settings.PayScheme {
	case rpc.PaySchemeFree:
		// Send a dummy invoice to avoid having the client re-request it.
		payload.NextInvoice = "free invoice"

	case rpc.PaySchemeDCRLN:
		// Only need to regenerate if it's empty, because the user might
		// have sent only already paid for subs.
		sc.Lock()
		needsNewInvoice := sc.lnPayReqHashSub == nil
		sc.Unlock()

		if needsNewInvoice {
			var err error
			var invoiceID string
			invoiceAction := rpc.InvoiceActionSub
			payload.NextInvoice, invoiceID, err = z.generateNextLNInvoice(ctx, sc, invoiceAction)
			if err != nil {
				sc.log.Errorf("handleSubscribeRoutedMessages generate invoice %v", err)
			} else {
				sc.log.Debugf("Generated invoice for action %q pay scheme %q: %s",
					invoiceAction, z.settings.PayScheme, invoiceID)
			}
		}

	default:
		// Shouldn't happen unless it's in-development.
		return fmt.Errorf("unimplemented payment scheme %s", z.settings.PayScheme)
	}

	// Reply.
	sc.writer <- &RPCWrapper{
		Message: rpc.Message{
			Command: rpc.TaggedCmdSubscribeRoutedMessagesReply,
			Tag:     msg.Tag,
		},
		Payload: payload,
	}

	return nil
}
