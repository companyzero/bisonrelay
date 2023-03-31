package client

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/timestats"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/decred/dcrlnd/lnrpc/invoicesrpc"
	"github.com/decred/dcrlnd/lnrpc/routerrpc"
	"github.com/decred/dcrlnd/lnrpc/walletrpc"
	"github.com/decred/dcrlnd/macaroons"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

type DcrlnPaymentClientCfg struct {
	TLSCertPath  string
	MacaroonPath string
	Address      string
	Log          slog.Logger
}

// DcrlnPaymentClient implements the PaymentClient interface for servers that
// offer the "dcrln" payment scheme.
type DcrlnPaymentClient struct {
	lnRpc      lnrpc.LightningClient
	lnInvoices invoicesrpc.InvoicesClient
	lnUnlocker lnrpc.WalletUnlockerClient
	lnRouter   routerrpc.RouterClient
	lnWallet   walletrpc.WalletKitClient
	log        slog.Logger
	payTiming  *timestats.Tracker
}

// NewDcrlndPaymentClient creates a new payment client that can send payments
// through dcrlnd.
func NewDcrlndPaymentClient(ctx context.Context, cfg DcrlnPaymentClientCfg) (*DcrlnPaymentClient, error) {
	// First attempt to establish a connection to lnd's RPC sever.
	creds, err := credentials.NewClientTLSFromFile(cfg.TLSCertPath, "")
	if err != nil {
		return nil, fmt.Errorf("unable to read cert file: %v", err)
	}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}

	// Load the specified macaroon file.
	macBytes, err := os.ReadFile(cfg.MacaroonPath)
	if err != nil {
		return nil, err
	}
	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macBytes); err != nil {
		return nil, err
	}

	// Now we append the macaroon credentials to the dial options.
	opts = append(
		opts,
		grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
	)

	conn, err := grpc.Dial(cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to dial to dcrlnd's gRPC server: %v", err)
	}

	// Start RPCs.
	lnRpc := lnrpc.NewLightningClient(conn)
	lnInvoices := invoicesrpc.NewInvoicesClient(conn)
	lnUnlocker := lnrpc.NewWalletUnlockerClient(conn)
	lnRouter := routerrpc.NewRouterClient(conn)
	lnWallet := walletrpc.NewWalletKitClient(conn)

	log := slog.Disabled
	if cfg.Log != nil {
		log = cfg.Log
	}

	return &DcrlnPaymentClient{
		lnRpc:      lnRpc,
		lnInvoices: lnInvoices,
		lnUnlocker: lnUnlocker,
		lnRouter:   lnRouter,
		lnWallet:   lnWallet,
		log:        log,
		payTiming:  timestats.NewTracker(250),
	}, nil
}

func (pc *DcrlnPaymentClient) LNRPC() lnrpc.LightningClient {
	return pc.lnRpc
}

func (pc *DcrlnPaymentClient) LNWallet() walletrpc.WalletKitClient {
	return pc.lnWallet
}

func (pc *DcrlnPaymentClient) PayScheme() string {
	return rpc.PaySchemeDCRLN
}

func (pc *DcrlnPaymentClient) UnlockWallet(ctx context.Context, pass string) error {
	if pc == nil {
		return fmt.Errorf("not connected / not running")
	}
	uwr := lnrpc.UnlockWalletRequest{
		WalletPassword: []byte(pass),
	}
	_, err := pc.lnUnlocker.UnlockWallet(ctx, &uwr)
	if err != nil {
		return err
	}

	pc.log.Info("Unlocked wallet")
	return nil
}

func (pc *DcrlnPaymentClient) PayInvoice(ctx context.Context, invoice string) (int64, error) {
	payReq, err := pc.lnRpc.DecodePayReq(ctx, &lnrpc.PayReqString{PayReq: invoice})
	if err != nil {
		return 0, fmt.Errorf("unable to decode pay req")
	}

	// TODO: Verify limits (max amount to pay, CLTV, etc).

	pc.log.Debugf("Attempting to pay %d MAtoms, hash %s req %s", payReq.NumMAtoms,
		payReq.PaymentHash, invoice)

	sendPayReq := &lnrpc.SendRequest{
		PaymentRequest: invoice,
		FeeLimit:       PaymentFeeLimit(uint64(payReq.NumMAtoms)),
	}

	start := time.Now()
	sendPayRes, err := pc.lnRpc.SendPaymentSync(ctx, sendPayReq)
	if err != nil {
		pc.log.Warnf("SendPayment error (%v) when attempting to pay "+
			"invoice. hash=%s, target=%s numMAtoms=%d",
			err, payReq.PaymentHash,
			payReq.Destination, payReq.NumMAtoms)
		return 0, fmt.Errorf("unable to complete LN payment: %v", err)
	}

	if sendPayRes.PaymentError != "" {
		pc.log.Warnf("Payment error (%s) when attempting to pay "+
			"invoice. hash=%s, target=%s numMAtoms=%d",
			sendPayRes.PaymentError, payReq.PaymentHash,
			payReq.Destination, payReq.NumMAtoms)
		return 0, fmt.Errorf("LN payment error: %s", sendPayRes.PaymentError)
	}
	pc.payTiming.Add(time.Since(start))

	fees := sendPayRes.PaymentRoute.TotalFeesMAtoms
	nbHops := len(sendPayRes.PaymentRoute.Hops)

	pc.log.Debugf("completed LN payment of hash %x preimage %x fees %d hops %d",
		sendPayRes.PaymentHash, sendPayRes.PaymentPreimage, fees, nbHops)

	return fees, nil
}

func (pc *DcrlnPaymentClient) PayInvoiceAmount(ctx context.Context, invoice string, amount int64) (int64, error) {
	payReq, err := pc.lnRpc.DecodePayReq(ctx, &lnrpc.PayReqString{PayReq: invoice})
	if err != nil {
		return 0, fmt.Errorf("unable to decode pay req")
	}

	// TODO: Verify limits (max amount to pay, CLTV, etc).

	pc.log.Debugf("Attempting to pay %d MAtoms, hash %s req %s", amount,
		payReq.PaymentHash, invoice)

	sendPayReq := &lnrpc.SendRequest{
		PaymentRequest: invoice,
		AmtMAtoms:      amount,
		FeeLimit:       PaymentFeeLimit(uint64(amount)),
	}

	start := time.Now()
	sendPayRes, err := pc.lnRpc.SendPaymentSync(ctx, sendPayReq)
	if err != nil {
		return 0, fmt.Errorf("unable to complete LN payment: %v", err)
	}

	if sendPayRes.PaymentError != "" {
		return 0, fmt.Errorf("LN payment error: %s", sendPayRes.PaymentError)
	}

	pc.payTiming.Add(time.Since(start))

	fees := sendPayRes.PaymentRoute.TotalFeesMAtoms
	hops := len(sendPayRes.PaymentRoute.Hops)
	pc.log.Debugf("completed LN payment of hash %x preimage %x fees %d hops %d",
		sendPayRes.PaymentHash, sendPayRes.PaymentPreimage, fees, hops)

	return fees, nil
}

func (pc *DcrlnPaymentClient) watchInvoice(ctx context.Context, rhash []byte,
	cb func(int64)) {

	subsReq := &invoicesrpc.SubscribeSingleInvoiceRequest{
		RHash: rhash,
	}
	invStream, err := pc.lnInvoices.SubscribeSingleInvoice(context.Background(), subsReq)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			pc.log.Errorf("Unable to keep watching invoice %x: %v",
				rhash, err)
		}
		return
	}

	for {
		invUpdate, err := invStream.Recv()
		if err != nil {
			return
		}

		if invUpdate.State == lnrpc.Invoice_SETTLED {
			pc.log.Debugf("Invoice %x settled with %f DCR",
				rhash, float64(invUpdate.AmtPaidMAtoms)/1e11)
			if cb != nil {
				cb(invUpdate.AmtPaidMAtoms)
			}
			return
		} else if invUpdate.State == lnrpc.Invoice_CANCELED {
			return
		}
	}
}

func (pc *DcrlnPaymentClient) GetInvoice(ctx context.Context, mat int64, cb func(int64)) (string, error) {
	addInvoiceReq := &lnrpc.Invoice{}
	if mat < 1000 {
		addInvoiceReq.ValueMAtoms = mat
	} else {
		// Use Value to get warnings about missing capacity.
		addInvoiceReq.Value = mat / 1000
		if mat%1000 > 500 {
			addInvoiceReq.Value += 1
		}
	}
	addInvoiceRes, err := pc.lnRpc.AddInvoice(ctx, addInvoiceReq)
	if err != nil {
		return "", err
	}

	go pc.watchInvoice(ctx, addInvoiceRes.RHash, cb)

	return addInvoiceRes.PaymentRequest, nil
}

func (pc *DcrlnPaymentClient) IsInvoicePaid(ctx context.Context, minMatAmt int64, invoice string) error {
	payReq, err := pc.lnRpc.DecodePayReq(ctx, &lnrpc.PayReqString{PayReq: invoice})
	if err != nil {
		return fmt.Errorf("unable to decode pay req")
	}

	rhash, err := hex.DecodeString(payReq.PaymentHash)
	if err != nil {
		return err
	}

	lookupReq := &lnrpc.PaymentHash{RHash: rhash}
	var lookupRes *lnrpc.Invoice
	lookupRes, err = pc.lnRpc.LookupInvoice(ctx, lookupReq)
	if err != nil {
		return err
	}

	switch {
	case lookupRes.State == lnrpc.Invoice_CANCELED:
		return fmt.Errorf("LN invoice canceled")

	case lookupRes.State != lnrpc.Invoice_SETTLED:
		return fmt.Errorf("Unexpected LN state: %d",
			lookupRes.State)

	case lookupRes.AmtPaidMAtoms < minMatAmt:
		// TODO: also have upper limit if overpaid?
		return fmt.Errorf("paid %d < wanted %d: %w", lookupRes.AmtPaidMAtoms,
			minMatAmt, clientintf.ErrInvoiceInsufficientlyPaid)

	default:
		return nil
	}
}

func (pc *DcrlnPaymentClient) DecodeInvoice(ctx context.Context, invoice string) (clientintf.DecodedInvoice, error) {
	payReq, err := pc.lnRpc.DecodePayReq(ctx, &lnrpc.PayReqString{PayReq: invoice})
	if err != nil {
		return clientintf.DecodedInvoice{}, fmt.Errorf("unable to decode pay req")
	}

	expiryTS := (payReq.Timestamp + payReq.Expiry)

	id, err := hex.DecodeString(payReq.PaymentHash)
	if err != nil {
		return clientintf.DecodedInvoice{}, fmt.Errorf("unable to decode payment hash: %v", err)
	}

	return clientintf.DecodedInvoice{
		ID:         id,
		MAtoms:     payReq.NumMAtoms,
		ExpiryTime: time.Unix(expiryTS, 0),
	}, nil
}

func (pc *DcrlnPaymentClient) IsPaymentCompleted(ctx context.Context, invoice string) (int64, error) {
	payReq, err := pc.lnRpc.DecodePayReq(ctx, &lnrpc.PayReqString{PayReq: invoice})
	if err != nil {
		return 0, fmt.Errorf("unable to decode pay req")
	}

	payHash, err := hex.DecodeString(payReq.PaymentHash)
	if err != nil {
		return 0, fmt.Errorf("unable to decode payment hash: %v", err)
	}

	req := &routerrpc.TrackPaymentRequest{
		PaymentHash: payHash,
	}
	stream, err := pc.lnRouter.TrackPaymentV2(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("unable to create payment tracking stream: %v", err)
	}
	for {
		event, err := stream.Recv()
		if err != nil {
			return 0, fmt.Errorf("error reading from payment tracking stream: %v", err)
		}

		switch event.Status {
		case lnrpc.Payment_SUCCEEDED:
			return event.FeeMAtoms, nil
		case lnrpc.Payment_FAILED:
			return 0, fmt.Errorf("payment failed due to %s", event.FailureReason.String())
		case lnrpc.Payment_UNKNOWN:
			return 0, fmt.Errorf("payment status is unknown")
		case lnrpc.Payment_IN_FLIGHT:
			pc.log.Tracef("Payment %x is inflight", payHash)
		default:
			return 0, fmt.Errorf("unknown payment tracking status %s", event.Status)
		}
	}
}

// PaymentTimingStats returns timing information for payment stats.
func (pc *DcrlnPaymentClient) PaymentTimingStats() []timestats.Quantile {
	return pc.payTiming.Quantiles()
}

const (
	ErrUnableToQueryNode  = WalletUsableErrorKind("ErrUnableToQueryNode")
	ErrNoPeers            = WalletUsableErrorKind("ErrNoPeers")
	ErrLowOutboundBalance = WalletUsableErrorKind("ErrLowOutboundBalance")
	ErrTooOldBlockchain   = WalletUsableErrorKind("ErrTooOldBlockchain")
	ErrNoActiveChannels   = WalletUsableErrorKind("ErrNoActiveChannels")
	ErrNoRouteToServer    = WalletUsableErrorKind("ErrNoRouteToServer")
	ErrUnableToPingPeers  = WalletUsableErrorKind("ErrUnableToPingPeers")
)

// CheckLNWalletUsable checks whether the given ln wallet is synced and is
// usable for sending payments to the given server LN node.
func CheckLNWalletUsable(ctx context.Context, lc lnrpc.LightningClient, svrNode string) error {
	info, err := lc.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		errMsg := fmt.Sprintf("unable to query node info: %v", err)
		return makeWalletUsableErr(ErrUnableToQueryNode, errMsg)
	}

	// Check if all peers are online. We force-ping all peers with a
	// 15-second deadline and quit if they don't respond. This is useful
	// in scenarios where the network dropped and we wanna double check
	// the peers are online.
	peers, err := lc.ListPeers(ctx, &lnrpc.ListPeersRequest{})
	if err != nil {
		errMsg := fmt.Sprintf("unable to query node peers: %v", err)
		return makeWalletUsableErr(ErrUnableToQueryNode, errMsg)
	}
	if len(peers.Peers) == 0 {
		errMsg := "ln wallet does not have any peers"
		return makeWalletUsableErr(ErrNoPeers, errMsg)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	var g errgroup.Group
	for _, p := range peers.Peers {
		p := p
		g.Go(func() error {
			req := &lnrpc.EnforceNodePingRequest{
				PubKey: p.PubKey,
			}
			_, err := lc.EnforceNodePing(pingCtx, req)
			if err != nil {
				return fmt.Errorf("unable to enforce ping to "+
					"peer %s: %v", p.PubKey, err)
			}
			return nil
		})
	}
	err = g.Wait()
	cancel()
	if err != nil {
		errMsg := fmt.Sprintf("unable to ping all peers: %v", err)
		return makeWalletUsableErr(ErrUnableToPingPeers, errMsg)
	}

	// Check if wallet has channels.
	if info.NumActiveChannels == 0 {
		errMsg := "ln wallet does not have any active channels"
		return makeWalletUsableErr(ErrNoActiveChannels, errMsg)
	}

	// We want a timestamp that is at least more recent than an hour ago to
	// make sure we're not too far from the current tip.
	wantMinTimestamp := time.Now().Round(0).Add(-60 * time.Minute)
	headerTime := time.Unix(info.BestHeaderTimestamp, 0)
	if headerTime.Before(wantMinTimestamp) {
		format := "2006-01-02T15:04:05"
		errMsg := fmt.Sprintf("blockchain tip %s at height %d has timestamp "+
			"(%s) which is older than the minimum allowed (%s)", info.BlockHash,
			info.BlockHeight, headerTime.Format(format),
			wantMinTimestamp.Format(format))
		return makeWalletUsableErr(ErrTooOldBlockchain, errMsg)
	}

	// Check if wallet has outbound bandwidth.
	bal, err := lc.ChannelBalance(ctx, &lnrpc.ChannelBalanceRequest{})
	if err != nil {
		errMsg := fmt.Sprintf("unable to query node channel balance: %v", err)
		return makeWalletUsableErr(ErrUnableToQueryNode, errMsg)
	}
	lowBalanceLimit := int64(1000) // In Milliatoms
	if bal.MaxOutboundAmount < lowBalanceLimit {
		errMsg := fmt.Sprintf("wallet has low outbound bandwidth (%.8f)",
			dcrutil.Amount(bal.MaxOutboundAmount/1000).ToCoin())
		return makeWalletUsableErr(ErrLowOutboundBalance, errMsg)
	}

	// Check if we can find a route to pay the server node.
	reqQuerySvrNode := &lnrpc.QueryRoutesRequest{
		PubKey:    svrNode,
		AmtMAtoms: 1000,
		FeeLimit:  PaymentFeeLimit(1000),
	}
	queryRouteRes, err := lc.QueryRoutes(ctx, reqQuerySvrNode)
	if err != nil {
		errMsg := fmt.Sprintf("unable to query payment route to server: %v", err)
		return makeWalletUsableErr(ErrUnableToQueryNode, errMsg)
	}
	if len(queryRouteRes.Routes) == 0 {
		errMsg := fmt.Sprintf("no payment route found to server node %s",
			svrNode)
		return makeWalletUsableErr(ErrNoRouteToServer, errMsg)
	}

	return nil
}

// PaymentFeeLimit returns the fee limit to use when making a payment for BR
// nodes of the specified size.
func PaymentFeeLimit(amountMAtoms uint64) *lnrpc.FeeLimit {
	// minFeeLimit is the minimum fee limit to use for making payments. This
	// is set so that the minimum payment of BR messages (1 atom) can be
	// sent through multiple hops of the current Decred LN network (which
	// uses a 1 atom base fee per hop).
	const minFeeLimit int64 = 20 * 1000

	var feeLimit *lnrpc.FeeLimit
	if int64(amountMAtoms) < minFeeLimit {
		feeLimit = &lnrpc.FeeLimit{
			Limit: &lnrpc.FeeLimit_FixedMAtoms{
				FixedMAtoms: int64(amountMAtoms) + minFeeLimit,
			},
		}
	}

	return feeLimit
}
