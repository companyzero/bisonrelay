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

		// Now we append the macaroon credentials to the dial options.
		opts = append(
			opts,
			grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
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

		return nil
	default:
		return fmt.Errorf("unknown payment scheme %s",
			z.settings.PayScheme)
	}
}

func (z *ZKS) generateNextLNInvoice(ctx context.Context, sc *sessionContext, action rpc.GetInvoiceAction) (string, string, error) {
	// TODO: configurable timeout limit?
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	addInvoiceReq := &lnrpc.Invoice{
		Memo: "FR server invoice",
	}
	addInvoiceRes, err := z.lnRpc.AddInvoice(ctx, addInvoiceReq)
	if err != nil {
		return "", "", err
	}

	sc.Lock()
	var hash *[]byte
	switch action {
	case rpc.InvoiceActionPush:
		hash = &sc.lnPayReqHashPush
	case rpc.InvoiceActionSub:
		hash = &sc.lnPayReqHashSub
	default:
		sc.Unlock()
		return "", "", fmt.Errorf("unknown action %q", action)
	}

	if *hash != nil {
		// Double check this invoice was not cancelled or expired.
		lookupReq := &lnrpc.PaymentHash{
			RHash: *hash,
		}
		var lookupRes *lnrpc.Invoice
		lookupRes, err = z.lnRpc.LookupInvoice(ctx, lookupReq)
		if err != nil && strings.HasSuffix(err.Error(), "unable to locate invoice") {
			// Invoice expired.
			err = nil
		} else if lookupRes != nil {
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

		// There was already an outstanding payment for this
		// session. Returning an error here ensures only a
		// single invoice can be requested at a time.
		if err != nil {
			sc.Unlock()
			return "", "", err
		}
	}
	*hash = addInvoiceRes.RHash
	id := hex.EncodeToString(addInvoiceRes.RHash)
	sc.Unlock()

	z.stats.invoicesSent.add(1)

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

// isRMPaid returns whether the received routed message was paid for. Returns
// nil if it is paid, or an error if not.
func (z *ZKS) isRMPaid(ctx context.Context, rm *rpc.RouteMessage, sc *sessionContext) error {
	switch z.settings.PayScheme {
	case rpc.PaySchemeFree:
		return nil

	case rpc.PaySchemeDCRLN:
		msgLen := int64(len(rm.Message))
		wantMAtoms := msgLen * int64(z.settings.MilliAtomsPerByte)

		// Enforce the minimum payment policy.
		if wantMAtoms < int64(rpc.MinRMPushPayment) {
			wantMAtoms = int64(rpc.MinRMPushPayment)
		}

		sc.Lock()
		var err error
		if wantMAtoms < 0 {
			// Sanity check. Should never happen.
			err = fmt.Errorf("wantMAtoms (%d) < 0", wantMAtoms)
		} else if sc.lnPayReqHashPush == nil {
			err = fmt.Errorf("ln invoice not generated for next routed message")
		} else {
			lookupReq := &lnrpc.PaymentHash{
				RHash: sc.lnPayReqHashPush,
			}

			// Use a 5-second timeout context to avoid stalling the
			var lookupRes *lnrpc.Invoice
			lookupRes, err = z.lnRpc.LookupInvoice(ctx, lookupReq)
			if lookupRes != nil {
				switch {
				case lookupRes.State == lnrpc.Invoice_CANCELED:
					err = fmt.Errorf("LN invoice canceled")

					// Clear canceled/timed out invoices so
					// a new one can be generated.
					sc.lnPayReqHashPush = nil

				case lookupRes.State != lnrpc.Invoice_SETTLED:
					err = fmt.Errorf("Unexpected LN state: %d",
						lookupRes.State)

				case lookupRes.AmtPaidMAtoms < wantMAtoms:
					// TODO: also have upper limit if
					// overpaid?
					err = fmt.Errorf("LN invoice not "+
						"sufficiently paid (got %d, want %d)",
						lookupRes.AmtPaidMAtoms, wantMAtoms)

				default:
					z.stats.invoicesRecv.add(1)
					z.stats.matomsRecv.add(lookupRes.AmtPaidMAtoms)

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
			// Clear the successful payment req so we can wait for
			// the next request to generate a new invoice.
			sc.lnPayReqHashPush = nil
		}

		sc.Unlock()

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
				switch {
				case lookupRes.State == lnrpc.Invoice_OPEN:
					// Could be that the request doesn't
					// have any new (unpaid) RVs, so keep
					// going until we determine a payment
					// was actually needed.

				case lookupRes.State == lnrpc.Invoice_CANCELED:
					// Clear canceled/timed out invoices so
					// a new one can be generated, but
					// otherwise don't error because we might
					// not need any new payments yet.
					sc.lnPayReqHashPush = nil

				case lookupRes.State == lnrpc.Invoice_SETTLED:
					// Invoice paid. Determine how many
					// new subscripts will be allowed based
					// on how much was paid.
					sc.lnPayReqHashSub = nil
					nbAllowed = lookupRes.AmtPaidMAtoms / int64(z.settings.MilliAtomsPerSub)
					z.stats.invoicesRecv.add(1)
					z.stats.matomsRecv.add(lookupRes.AmtPaidMAtoms)

					sc.log.Debugf("LN invoice %x settled "+
						"w/ %d MAtoms for %d new subscriptions",
						lookupRes.RHash,
						lookupRes.AmtPaidMAtoms,
						nbAllowed,
					)

				default:
					err = fmt.Errorf("Unexpected LN state: %d",
						lookupRes.State)
					sc.lnPayReqHashPush = nil
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
	for _, rv := range r.AddRendezvous {
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
