package server

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/decred/dcrlnd/lnrpc/invoicesrpc"
	"github.com/decred/dcrlnd/macaroons"
	"github.com/decred/slog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	macaroon "gopkg.in/macaroon.v2"
)

func (z *ZKS) initPayments() error {
	switch z.settings.PayScheme {
	case rpc.PaySchemeFree:
		// Free payment scheme doesn't require any setup.
		return nil

	case rpc.PaySchemeDCRLN:
		// First attempt to establish a connection to lnd's RPC sever.
		creds, err := credentials.NewClientTLSFromFile(z.settings.LNTLSCert, "")
		if err != nil {
			return fmt.Errorf("unable to read cert file: %v", err)
		}
		opts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}

		// Load the specified macaroon file.
		macBytes, err := os.ReadFile(z.settings.LNMacaroonPath)
		if err != nil {
			return err
		}
		mac := &macaroon.Macaroon{}
		if err = mac.UnmarshalBinary(macBytes); err != nil {
			return err
		}

		macOpt, err := macaroons.NewMacaroonCredential(mac)
		if err != nil {
			return err
		}
		// Now we append the macaroon credentials to the dial options.
		opts = append(
			opts,
			grpc.WithPerRPCCredentials(macOpt),
		)

		conn, err := grpc.Dial(z.settings.LNRPCHost, opts...)
		if err != nil {
			return fmt.Errorf("unable to dial to dcrlnd's gRPC server: %v", err)
		}

		// Start RPCs.
		z.lnRpc = lnrpc.NewLightningClient(conn)
		z.lnInvoices = invoicesrpc.NewInvoicesClient(conn)

		// Check chain and network (mainnet, testnet, etc)?
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		lnInfo, err := z.lnRpc.GetInfo(ctx, &lnrpc.GetInfoRequest{})
		if err != nil {
			return fmt.Errorf("unable to get dcrlnd node info: %v", err)
		}

		z.lnNode = lnInfo.IdentityPubkey
		z.log.Infof("Initialized dcrlnd payment subsystem using node %s", z.lnNode)

		matomsPerGb, _ := z.calcPushCostMAtoms(1e9)
		dcrPerGb := float64(matomsPerGb) / 1e11
		z.log.Infof("Push data rate: %.8f DCR/GB", dcrPerGb)

		return nil
	default:
		return fmt.Errorf("unknown payment scheme %s",
			z.settings.PayScheme)
	}
}

// checkLNInvoiceOutstanding returns an error if the invoice if the given ln
// invoice is still outstanding or nil if the invoice is expired/cancelled.
func (z *ZKS) checkLNInvoiceOutstanding(ctx context.Context, hash []byte) error {
	lookupReq := &lnrpc.PaymentHash{
		RHash: hash,
	}
	var lookupRes *lnrpc.Invoice
	lookupRes, err := z.lnRpc.LookupInvoice(ctx, lookupReq)
	if err != nil && strings.HasSuffix(err.Error(), "unable to locate invoice") {
		// Invoice expired.
		err = nil
	} else if err == nil && lookupRes != nil {
		unsettledInvoice := (lookupRes.State != lnrpc.Invoice_CANCELED) &&
			(lookupRes.State != lnrpc.Invoice_SETTLED)
		if unsettledInvoice {
			expireTS := time.Unix(lookupRes.CreationDate+lookupRes.Expiry, 0)
			minExpiryTS := time.Now().Add(rpc.InvoiceExpiryAffordance)
			if expireTS.After(minExpiryTS) {
				err = fmt.Errorf("already have outstanding "+
					"ln payment request that expires only "+
					"in %s", expireTS.Sub(minExpiryTS))
			}
		}
	}

	return err
}

func (z *ZKS) generateNextLNInvoice(ctx context.Context, sc *sessionContext, action rpc.GetInvoiceAction) (string, string, error) {

	// Configurable timeout limit?
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	sc.Lock()
	defer sc.Unlock()

	// Check for limits of invoice generation. Depending on the action,
	// different limits are applied.
	switch action {
	case rpc.InvoiceActionPush:
		// When at the limit of max amount of concurrent invoices,
		// check if any have already expired.
		if len(sc.lnPushHashes) >= z.settings.MaxPushInvoices {
			now := time.Now()
			deleted := false
			for id, expires := range sc.lnPushHashes {
				if now.After(expires) {
					delete(sc.lnPushHashes, id)
					deleted = true
				}
			}
			if !deleted {
				return "", "", fmt.Errorf("max amount of unpaid invoices reached")
			}
		}
	case rpc.InvoiceActionSub:
		if sc.lnPayReqHashSub != nil {
			err := z.checkLNInvoiceOutstanding(ctx, sc.lnPayReqHashSub)
			if err != nil {
				return "", "", err
			}
		}

	case rpc.InvoiceActionCreateRTSess:
		if sc.lnCreateRTSessHash != nil {
			err := z.checkLNInvoiceOutstanding(ctx, sc.lnCreateRTSessHash)
			if err != nil {
				return "", "", err
			}
		}

	case rpc.InvoiceActionGetRTCookie:
		if sc.lnGetRTCookieHash != nil {
			err := z.checkLNInvoiceOutstanding(ctx, sc.lnGetRTCookieHash)
			if err != nil {
				return "", "", err
			}
		}

	case rpc.InvoiceActionPublishInRTSess:
		if sc.lnPublishInRTSessHash != nil {
			err := z.checkLNInvoiceOutstanding(ctx, sc.lnPublishInRTSessHash)
			if err != nil {
				return "", "", err
			}
		}

	default:
		return "", "", fmt.Errorf("unknown action %q", action)
	}

	expirySeconds := 3600
	addInvoiceReq := &lnrpc.Invoice{
		Memo:   "BR server invoice",
		Expiry: int64(expirySeconds),
	}
	addInvoiceRes, err := z.lnRpc.AddInvoice(ctx, addInvoiceReq)
	if err != nil {
		return "", "", err
	}

	// Store the generated invoice to count it towards the limits.
	switch action {
	case rpc.InvoiceActionPush:
		// Track when this invoice will expire.
		var hash [32]byte
		copy(hash[:], addInvoiceRes.RHash)
		expireTS := time.Now().Add(time.Second*time.Duration(expirySeconds) - rpc.InvoiceExpiryAffordance)
		sc.lnPushHashes[hash] = expireTS
	case rpc.InvoiceActionSub:
		sc.lnPayReqHashSub = addInvoiceRes.RHash
	case rpc.InvoiceActionCreateRTSess:
		sc.lnCreateRTSessHash = addInvoiceRes.RHash
	case rpc.InvoiceActionGetRTCookie:
		sc.lnGetRTCookieHash = addInvoiceRes.RHash
	case rpc.InvoiceActionPublishInRTSess:
		sc.lnPublishInRTSessHash = addInvoiceRes.RHash
	}

	z.stats.invoicesSent.Add(1)
	id := hex.EncodeToString(addInvoiceRes.RHash)
	return addInvoiceRes.PaymentRequest, id, nil
}

func (z *ZKS) handleGetInvoice(ctx context.Context, sc *sessionContext,
	msg rpc.Message, r rpc.GetInvoice) error {

	if r.PaymentScheme != z.settings.PayScheme {
		return fmt.Errorf("client requested unsuported pay scheme %s",
			r.PaymentScheme) // Sanitize PaymentScheme for log?
	}

	var invoice rpc.GetInvoiceReply
	var invoiceID string
	switch r.PaymentScheme {
	case rpc.PaySchemeFree:
		// Send a dummy invoice to avoid having the client re-request it.
		invoice.Invoice = "free invoice"

	case rpc.PaySchemeDCRLN:
		var err error
		invoice.Invoice, invoiceID, err = z.generateNextLNInvoice(ctx, sc, r.Action)
		if err != nil {
			return err
		}

	default:
		// Shouldn't happen unless it's in-development.
		return fmt.Errorf("unimplemented payment scheme %s", r.PaymentScheme)
	}

	if sc.log.Level() <= slog.LevelTrace {
		sc.log.Tracef("Generated invoice for action %q pay scheme %q: %s",
			r.Action, r.PaymentScheme, invoice)
	} else {
		sc.log.Debugf("Generated invoice for action %q pay scheme %q: %s",
			r.Action, r.PaymentScheme, invoiceID)
	}

	reply := RPCWrapper{
		Message: rpc.Message{
			Command: rpc.TaggedCmdGetInvoiceReply,
			Tag:     msg.Tag,
		},
		Payload: invoice,
	}
	sc.writer <- &reply

	return nil
}

func (z *ZKS) calcPushCostMAtoms(msgLen int) (int64, error) {
	return rpc.CalcPushCostMAtoms(z.settings.PushPayRateMinMAtoms,
		z.settings.PushPayRateMAtoms, z.settings.PushPayRateBytes,
		uint64(msgLen))
}

// isRMPaid returns whether the received routed message was paid for. Returns
// nil if it is paid, or an error if not.
func (z *ZKS) isRMPaid(ctx context.Context, rm *rpc.RouteMessage, sc *sessionContext) error {
	switch z.settings.PayScheme {
	case rpc.PaySchemeFree:
		return nil

	case rpc.PaySchemeDCRLN:
		msgLen := len(rm.Message)
		wantMAtoms, err := z.calcPushCostMAtoms(msgLen)

		// Compat to old clients: if the PaidInvoiceID field is nil and
		// there is a single outstanding invoice, use that one.
		//
		// TODO: remove in the future once all clients have updated.
		paidInvoiceID := rm.PaidInvoiceID
		if paidInvoiceID == nil {
			sc.Lock()
			if len(sc.lnPushHashes) == 1 {
				for id := range sc.lnPushHashes {
					paidInvoiceID = id[:]
				}
			}
			sc.Unlock()
		}

		// Sanity check paid invoice id.
		if err == nil && len(paidInvoiceID) != 32 {
			err = fmt.Errorf("paid invoice ID was not specified")
		}

		// Verify the potentially paid invoice was not redeemed yet.
		if err == nil {
			var redeemed bool
			redeemed, err = z.db.IsPushPaymentRedeemed(ctx, paidInvoiceID)
			if err == nil && redeemed {
				err = fmt.Errorf("already redeemed invoice %x", paidInvoiceID)
			}
		}

		// Verify the invoice was settled.
		if err == nil {
			lookupReq := &lnrpc.PaymentHash{
				RHash: paidInvoiceID,
			}

			maxLifetimeDuration := time.Duration(z.settings.PushPaymentLifetime) * time.Second
			payTimeLimit := time.Now().Add(-maxLifetimeDuration)

			// Use a 5-second timeout context to avoid stalling the
			// server.
			var lookupRes *lnrpc.Invoice
			lookupRes, err = z.lnRpc.LookupInvoice(ctx, lookupReq)
			if lookupRes != nil {
				switch {
				case lookupRes.State == lnrpc.Invoice_CANCELED:
					err = fmt.Errorf("LN invoice canceled")

				case lookupRes.State != lnrpc.Invoice_SETTLED:
					err = fmt.Errorf("unexpected LN state: %d",
						lookupRes.State)

				case lookupRes.AmtPaidMAtoms < wantMAtoms:
					// Also have upper limit if
					// overpaid?
					err = fmt.Errorf("LN invoice not "+
						"sufficiently paid (got %d, want %d)",
						lookupRes.AmtPaidMAtoms, wantMAtoms)

				case time.Unix(lookupRes.SettleDate, 0).Before(payTimeLimit):
					err = fmt.Errorf("LN invoice settled at %s "+
						"while limit date for redemption "+
						"is %s", time.Unix(lookupRes.SettleDate, 0),
						payTimeLimit)

				default:
					z.stats.invoicesRecv.Add(1)
					z.stats.matomsRecv.Add(lookupRes.AmtPaidMAtoms)

					// Everything ok.
					sc.log.Debugf("LN invoice %x settled "+
						"w/ %d MAtoms for %d bytes",
						lookupRes.RHash,
						lookupRes.AmtPaidMAtoms,
						msgLen)
				}
			}
		}

		if err == nil {
			// Store that the invoice was redeemed.
			err = z.db.StorePushPaymentRedeemed(ctx, paidInvoiceID, time.Now())

			// And decrement from total amount of concurrent invoices.
			var hash [32]byte
			copy(hash[:], paidInvoiceID)
			sc.Lock()
			delete(sc.lnPushHashes, hash)
			sc.Unlock()
		}

		return err
	default:
		return fmt.Errorf("unimplemented isNextRMPaid for scheme %s",
			z.settings.PayScheme)
	}
}

// areSubsPaid verifies whether all subscriptions in the given message were paid
// for, either previously or with the most recent payment.
func (z *ZKS) areSubsPaid(ctx context.Context, r *rpc.SubscribeRoutedMessages, sc *sessionContext) error {
	var err error
	var nbAllowed int64 // nb of max new entries allowed, based on paid invoice

	switch z.settings.PayScheme {
	case rpc.PaySchemeFree:
		// Always paid.
		return nil

	case rpc.PaySchemeDCRLN:
		sc.Lock()
		if sc.lnPayReqHashSub != nil {
			lookupReq := &lnrpc.PaymentHash{
				RHash: sc.lnPayReqHashSub,
			}
			var lookupRes *lnrpc.Invoice
			lookupRes, err = z.lnRpc.LookupInvoice(ctx, lookupReq)
			if lookupRes != nil {
				switch lookupRes.State {
				case lnrpc.Invoice_OPEN:
					// Could be that the request doesn't
					// have any new (unpaid) RVs, so keep
					// going until we determine a payment
					// was actually needed.

				case lnrpc.Invoice_CANCELED:
					// Clear canceled/timed out invoices so
					// a new one can be generated, but
					// otherwise don't error because we might
					// not need any new payments yet.
					sc.lnPayReqHashSub = nil

				case lnrpc.Invoice_SETTLED:
					// Invoice paid. Determine how many
					// new subscripts will be allowed based
					// on how much was paid.
					sc.lnPayReqHashSub = nil
					nbAllowed = lookupRes.AmtPaidMAtoms / int64(z.settings.MilliAtomsPerSub)
					z.stats.invoicesRecv.Add(1)
					z.stats.matomsRecv.Add(lookupRes.AmtPaidMAtoms)

					sc.log.Debugf("LN invoice %x settled "+
						"w/ %d MAtoms for %d new subscriptions",
						lookupRes.RHash,
						lookupRes.AmtPaidMAtoms,
						nbAllowed,
					)

				default:
					err = fmt.Errorf("unexpected LN state: %d",
						lookupRes.State)
					sc.lnPayReqHashSub = nil
				}
			}
		} else {
			sc.lnPayReqHashSub = nil
		}
		sc.Unlock()
	}

	if err != nil {
		return err
	}

	// Store in DB the new unpaid items.
	needsPay := append(r.AddRendezvous, r.MarkPaid...)
	for _, rv := range needsPay {
		if paid, err := z.db.IsSubscriptionPaid(ctx, rv); err != nil {
			return err
		} else if paid {
			continue
		}

		if nbAllowed <= 0 {
			return rpc.ErrUnpaidSubscriptionRV(rv)
		}
		nbAllowed -= 1
		if err := z.db.StoreSubscriptionPaid(z.dbCtx, rv, time.Now()); err != nil {
			return err
		}
		sc.log.Debugf("Stored RV %s as paid", rv)
	}

	if nbAllowed > 0 {
		sc.log.Warnf("Paid for more new subscriptions (%d) than "+
			"performed", nbAllowed)
	}

	return err
}

func (z *ZKS) cancelLNInvoice(ctx context.Context, hash []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req := &invoicesrpc.CancelInvoiceMsg{
		PaymentHash: hash,
	}

	_, err := z.lnInvoices.CancelInvoice(ctx, req)
	return err
}
