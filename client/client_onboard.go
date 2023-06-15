package client

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrlnd/lnrpc"
	lpclient "github.com/decred/dcrlnlpd/client"
)

// ReadOnboard returns the existing onboard state.
func (c *Client) ReadOnboard() (*clientintf.OnboardState, error) {
	var ostate clientintf.OnboardState
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		ostate, err = c.db.ReadOnboardState(tx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &ostate, err
}

// RetryOnboarding retries the onboarding at the current stage. The onboarding
// must have errored before this can be called.
func (c *Client) RetryOnboarding() error {
	c.onboardMtx.Lock()
	defer c.onboardMtx.Unlock()
	if c.onboardRunning {
		return fmt.Errorf("cannot retry while onboarding is running")
	}

	var ostate clientintf.OnboardState
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		ostate, err = c.db.ReadOnboardState(tx)
		return err
	})
	if err != nil {
		return err
	}
	c.log.Infof("Retrying onboarding attempt")
	go c.runOnboarding(c.ctx, ostate)
	return nil
}

// SkipOnboardingStage skips the current onboarding stage to the next one. Not
// every stage is skippable.
func (c *Client) SkipOnboardingStage() error {
	c.onboardMtx.Lock()
	defer c.onboardMtx.Unlock()
	if c.onboardRunning {
		return fmt.Errorf("cannot skip while onboarding is running")
	}

	var ostate clientintf.OnboardState
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		ostate, err = c.db.ReadOnboardState(tx)
		if err != nil {
			return err
		}

		switch ostate.Stage {
		case clientintf.StageWaitingOutConfirm:
			ostate.Stage = clientintf.StageOpeningInbound
		case clientintf.StageOpeningInbound:
			ostate.Stage = clientintf.StageInitialKX
		case clientintf.StageInitialKX:
			ostate.Stage = clientintf.StageOnboardDone
		}

		return c.db.UpdateOnboardState(tx, &ostate)
	})
	if err != nil {
		return err
	}
	c.log.Infof("Skipping onboarding stage to %s", ostate.Stage)
	c.ntfns.notifyOnOnboardStateChanged(ostate, nil) // Initial state
	go c.runOnboarding(c.ctx, ostate)
	return nil
}

// StartOnboarding starts a new onboarding procedure with the given key.
func (c *Client) StartOnboarding(key clientintf.PaidInviteKey) error {
	c.onboardMtx.Lock()
	defer c.onboardMtx.Unlock()
	if c.onboardRunning {
		return fmt.Errorf("onboarding already running")
	}

	var ostate clientintf.OnboardState
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		if c.db.HasOnboardState(tx) {
			return fmt.Errorf("already have existing onboarding procedure")
		}
		ostate.Key = &key
		ostate.Stage = clientintf.StageFetchingInvite
		return c.db.UpdateOnboardState(tx, &ostate)
	})
	if err != nil {
		return err
	}
	c.log.Infof("Starting onboarding with key %s", key)
	c.ntfns.notifyOnOnboardStateChanged(ostate, nil) // Initial state
	go c.runOnboarding(c.ctx, ostate)
	return nil
}

// CancelOnboarding stops the currently running onboarding and removes it from
// the client.
func (c *Client) CancelOnboarding() error {
	c.onboardMtx.Lock()
	defer c.onboardMtx.Unlock()

	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.RemoveOnboardState(tx)
	})
	if err != nil {
		return err
	}

	if c.onboardRunning {
		go func() { c.onboardCancelChan <- struct{}{} }()
	}
	return nil
}

// onboardRedeemOnchainFunds redeems the on-chain funds included in the invite.
func (c *Client) onboardRedeemOnchainFunds(ctx context.Context, funds *rpc.InviteFunds) (dcrutil.Amount, chainhash.Hash, error) {
	var amount dcrutil.Amount
	var tx chainhash.Hash
	pc, ok := c.pc.(*DcrlnPaymentClient)
	if !ok {
		return amount, tx, fmt.Errorf("payment client is not a dcrlnd payment client")
	}

	for {
		var err error
		amount, tx, err = pc.RedeemInviteFunds(ctx, funds)
		if err == nil {
			return amount, tx, nil
		}
		if !strings.HasSuffix(err.Error(), "not found during chain scan") {
			return amount, tx, err
		}

		// The "not found during chain scan" error is likely because
		// the tx is still in the mempool (but the LN node doesn't know
		// about it). Wait until a new block comes in and try again.
		c.log.Infof("Invite funds not found on-chain. Waiting for next block to try again")
		bh, height, err := pc.WaitNextBlock(ctx)
		if err != nil {
			return amount, tx, err
		}
		c.log.Debugf("Received block %d (%s) while waiting to try to "+
			"spend invite funds again", height, bh)
	}
}

// onboardOpenOutboundChan opens the outbound LN channel.
func (c *Client) onboardOpenOutboundChan(ctx context.Context, onchainAmount dcrutil.Amount) (string, bool, error) {
	pc, ok := c.pc.(*DcrlnPaymentClient)
	if !ok {
		return "", false, fmt.Errorf("payment client is not a dcrlnd payment client")
	}

	lnRPC := pc.LNRPC()
	info, err := lnRPC.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		return "", false, err
	}

	var key, server string
	if len(info.Chains) == 0 || info.Chains[0].Chain != "decred" {
		return "", false, fmt.Errorf("not connected to decred LN")
	}
	switch info.Chains[0].Network {
	case "mainnet":
		key = "03bd03386d7b2efe80ae46d6c8cfcfdfcf9c9297a465ac0d48c110d11ae58ed509"
		server = "hub0.bisonrelay.org:9735"
	case "simnet":
		key = "029398ddb14e4b3cb92fc64d61fcaaa2f3b590951b0b05ba1ecc04a7504d333213"
		server = "127.0.0.1:20202"
	default:
		return "", false, fmt.Errorf("network %q does not have default hub", info.Chains[0].Network)
	}

	// Check if connecting to the node was successful.
	// We discard the peer id returned as it is not needed.
	req := &lnrpc.ConnectPeerRequest{
		Addr: &lnrpc.LightningAddress{
			Pubkey: key,
			Host:   server,
		},
		Perm: false,
	}
	_, err = lnRPC.ConnectPeer(ctx, req)
	if err != nil &&
		!strings.Contains(err.Error(), "already connected") {
		return "", false, err
	}

	// Determine how much to fund the outbound channel. This is a guess,
	// based on the total amount received onchain and the fees that will
	// be paid, capped at a maximum channel size of 5 DCR.
	fundingAmt := onchainAmount - 10800
	if fundingAmt > 5e8 {
		fundingAmt = 5e8
	}

	npk, err := hex.DecodeString(key)
	if err != nil {
		return "", false, fmt.Errorf("unable to decode pubkey: %w", err)
	}

	ocr := lnrpc.OpenChannelRequest{
		NodePubkey:         npk,
		LocalFundingAmount: int64(fundingAmt),
		PushAtoms:          0,
	}
	openStream, err := lnRPC.OpenChannel(ctx, &ocr)
	if err != nil {
		return "", false, err
	}

	res, err := openStream.Recv()
	if err != nil {
		return "", false, err
	}

	var cp string
	var opened bool
	switch updt := res.Update.(type) {
	case *lnrpc.OpenStatusUpdate_ChanPending:
		tx, err := chainhash.NewHash(updt.ChanPending.Txid)
		if err != nil {
			return "", false, err
		}
		cp = fmt.Sprintf("%s:%d", tx, updt.ChanPending.OutputIndex)
		c.log.Infof("Outbound channel pending in output %s", cp)

	case *lnrpc.OpenStatusUpdate_ChanOpen:
		cp = chanPointToStr(updt.ChanOpen.ChannelPoint)
		opened = true
		c.log.Infof("Outbound channel opened in output %s", cp)

	default:
		return "", false, fmt.Errorf("unknown OpenStatusUpdate type %T", res.Update)
	}

	return cp, opened, nil
}

// onboardWaitChannelOpened waits until a channel is opened to the local node.
func (c *Client) onboardWaitChannelOpened(ctx context.Context, cp string) error {
	pc, ok := c.pc.(*DcrlnPaymentClient)
	if !ok {
		return fmt.Errorf("payment client is not a dcrlnd payment client")
	}

	lnRPC := pc.LNRPC()

	// Subscribe to fetch channel events. This is done first to catch
	// channels opened between the other checks.
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	eventsStream, err := lnRPC.SubscribeChannelEvents(streamCtx, &lnrpc.ChannelEventSubscription{})
	if err != nil {
		return err
	}

	// See if the channel is already opened.
	listRes, err := lnRPC.ListChannels(ctx, &lnrpc.ListChannelsRequest{})
	if err != nil {
		return err
	}
	for _, c := range listRes.Channels {
		if c.ChannelPoint == cp {
			return nil
		}
	}

	// See if the channel is pending.
	isPending := false
	pendingRes, err := lnRPC.PendingChannels(ctx, &lnrpc.PendingChannelsRequest{})
	if err != nil {
		return err
	}
	for _, c := range pendingRes.PendingOpenChannels {
		if c.Channel.ChannelPoint == cp {
			isPending = true
			break
		}
	}
	if !isPending {
		return fmt.Errorf("channel %s is not pending", cp)
	}

	for {
		event, err := eventsStream.Recv()
		if err != nil {
			return err
		}

		opened := event.GetOpenChannel()
		if opened != nil && opened.ChannelPoint == cp {
			break
		}
	}

	// After opening, wait an additional time to ensure channel operations
	// are started.
	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// onboardOpenInboundChan requests to the LPD that the inbound channel be
// opened.
func (c *Client) onboardOpenInboundChan(ctx context.Context) (string, error) {
	pc, ok := c.pc.(*DcrlnPaymentClient)
	if !ok {
		return "", fmt.Errorf("payment client is not a dcrlnd payment client")
	}

	lnRPC := pc.LNRPC()
	info, err := lnRPC.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		return "", err
	}

	balance, err := lnRPC.ChannelBalance(ctx, &lnrpc.ChannelBalanceRequest{})
	if err != nil {
		return "", err
	}

	var server, cert string
	if len(info.Chains) == 0 || info.Chains[0].Chain != "decred" {
		return "", fmt.Errorf("not connected to decred LN")
	}
	switch info.Chains[0].Network {
	case "mainnet":
		server = "https://lp0.bisonrelay.org:9130"
		cert = `-----BEGIN CERTIFICATE-----
MIIBwjCCAWmgAwIBAgIQA78YKmDt+ffFJmAN5EZmejAKBggqhkjOPQQDAjAyMRMw
EQYDVQQKEwpiaXNvbnJlbGF5MRswGQYDVQQDExJscDAuYmlzb25yZWxheS5vcmcw
HhcNMjIwOTE4MTMzNjA4WhcNMzIwOTE2MTMzNjA4WjAyMRMwEQYDVQQKEwpiaXNv
bnJlbGF5MRswGQYDVQQDExJscDAuYmlzb25yZWxheS5vcmcwWTATBgcqhkjOPQIB
BggqhkjOPQMBBwNCAASF1StlsfdDUaCXMiZvDBhhMZMdvAUoD6wBdS0tMBN+9y91
UwCBu4klh+VmpN1kCzcR6HJHSx5Cctxn7Smw/w+6o2EwXzAOBgNVHQ8BAf8EBAMC
AoQwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUqqlcDx8e+XgXXU9cXAGQEhS8
59kwHQYDVR0RBBYwFIISbHAwLmJpc29ucmVsYXkub3JnMAoGCCqGSM49BAMCA0cA
MEQCIGtLFLIVMnU2EloN+gI+uuGqqqeBIDSNhP9+bznnZL/JAiABsLKKtaTllCSM
cNPr8Y+sSs2MHf6xMNBQzV4KuIlPIg==
-----END CERTIFICATE-----`

	case "simnet":
		server = "https://127.0.0.1:29130"

		// On simnet, load the cert from the default
		// ~/.dcrlnlpd/tls.cert location.
		dir := dcrutil.AppDataDir("dcrlnlpd", false)
		tlsCertFile := filepath.Join(dir, "tls.cert")
		certBytes, err := os.ReadFile(tlsCertFile)
		if err != nil {
			return "", err
		}
		cert = string(certBytes)

	default:
		return "", fmt.Errorf("network %q does not have default LPD", info.Chains[0].Network)
	}

	// Size the requested channel so that 66% of the outgoing channel
	// capacity is used to acquire the channel, or at most a 2 DCR channel
	// is requested. Assume the fee rate is 1%.
	const maxChanSize = 2e8
	maxToPay := balance.MaxOutboundAmount * 66 / 100
	chanAmt := dcrutil.Amount(maxToPay) * 100 // 1% fee rate
	if chanAmt > maxChanSize {
		chanAmt = maxChanSize
	}
	chanSize := uint64(chanAmt)
	pendingChan := make(chan string, 1)
	lpcfg := lpclient.Config{
		LC:           lnRPC,
		Address:      server,
		Certificates: []byte(cert),

		PolicyFetched: func(policy lpclient.ServerPolicy) error {
			estInvoice := lpclient.EstimatedInvoiceAmount(chanSize,
				policy.ChanInvoiceFeeRate)
			c.log.Infof("Fetched server policy for chan of size %d."+
				" Estimated Invoice amount: %s", chanAmt,
				dcrutil.Amount(estInvoice))
			return nil
		},

		PayingInvoice: func(payHash string) {
			c.log.Infof("Paying invoice %s for onboarding inbound channel", payHash)
		},

		InvoicePaid: func() {
			c.log.Infof("Invoice for onboarding inbound channel paid. Waiting for channel to be opened")
		},

		PendingChannel: func(channelPoint string, capacity uint64) {
			c.log.Infof("Detected new pending channel %s with LP node with capacity %s",
				channelPoint, dcrutil.Amount(capacity))
			pendingChan <- channelPoint
		},
	}
	lpc, err := lpclient.New(lpcfg)
	if err != nil {
		return "", err
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- lpc.RequestChannel(ctx, chanSize)
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case cp := <-pendingChan:
		return cp, nil
	case err := <-errChan:
		return "", err
	}
}

// onboardInitialKX performs the initial KX included in the invite.
func (c *Client) onboardInitialKX(ctx context.Context, ostate clientintf.OnboardState) error {
	// Listen for any events.
	kxCompletedChan := make(chan struct{}, 1)
	reg := c.ntfns.Register(OnKXCompleted(func(_ *clientintf.RawRVID, ru *RemoteUser, _ bool) {
		if ru.ID() == ostate.Invite.Public.Identity {
			kxCompletedChan <- struct{}{}
		}
	}))
	defer reg.Unregister()

	if _, err := c.UserByID(ostate.Invite.Public.Identity); err == nil {
		// Already KXd!
		return nil
	}

	// Check if already attempting to kx.
	var kxing bool
	err := c.dbView(func(tx clientdb.ReadTx) error {
		kxing = c.db.KXExists(tx, ostate.Invite.InitialRendezvous)
		return nil
	})
	if err != nil {
		return err
	}

	if !kxing {
		// Not yet. Accept.
		err := c.AcceptInvite(*ostate.Invite)
		if err != nil {
			return err
		}
	}

	// Wait until KX is done.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-kxCompletedChan:
		return nil
	}
}

// runOnboarding runs the onboarding procedure.
func (c *Client) runOnboarding(ctx context.Context, ostate clientintf.OnboardState) error {
	c.onboardMtx.Lock()
	if c.onboardRunning {
		c.onboardMtx.Unlock()
		return fmt.Errorf("already running onboarding")
	}
	c.onboardRunning = true
	c.onboardMtx.Unlock()

	// Cancel running the onboard if it's canceled by CancelOnboarding().
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
		case <-c.onboardCancelChan:
			cancel()
		}
	}()

	var runErr error
	for runErr == nil && ostate.Stage != clientintf.StageOnboardDone {
		switch {
		case canceled(ctx):
			runErr = ctx.Err()

		case ostate.Stage == clientintf.StageFetchingInvite && ostate.Key == nil:
			runErr = fmt.Errorf("empty paid invite key")

		case ostate.Stage == clientintf.StageFetchingInvite:
			var invite rpc.OOBPublicIdentityInvite
			invite, runErr = c.FetchPrepaidInvite(ctx, *ostate.Key, io.Discard)
			if runErr == nil {
				ostate.Invite = &invite
				ostate.Stage = clientintf.StageRedeemingFunds
			}

		case ostate.Stage == clientintf.StageRedeemingFunds && ostate.Invite.Funds == nil:
			ostate.Stage = clientintf.StageInviteNoFunds

		case ostate.Stage == clientintf.StageInviteNoFunds:
			runErr = clientintf.ErrOnboardNoFunds

		case ostate.Stage == clientintf.StageRedeemingFunds:
			// Haven't redeemed the invite funds yet, attempt to do
			// so.
			amount, tx, err := c.onboardRedeemOnchainFunds(ctx, ostate.Invite.Funds)
			runErr = err
			if runErr == nil {
				ostate.Stage = clientintf.StageWaitingFundsConfirm
				ostate.RedeemAmount = amount
				ostate.RedeemTx = &tx
			}

		case ostate.Stage == clientintf.StageWaitingFundsConfirm:
			// Verify that the invite funds were confirmed onchain.
			pc, ok := c.pc.(*DcrlnPaymentClient)
			if !ok {
				runErr = fmt.Errorf("payment client is not a dcrlnd payment client")
			} else {
				runErr = pc.WaitTxConfirmed(ctx, *ostate.RedeemTx)
				if runErr == nil {
					ostate.Stage = clientintf.StageOpeningOutbound
				}
			}

		case ostate.Stage == clientintf.StageOpeningOutbound:
			// Open channel to one of predefined hubs.
			var opened bool
			ostate.OutChannelID, opened, runErr = c.onboardOpenOutboundChan(ctx, ostate.RedeemAmount)
			if opened {
				ostate.Stage = clientintf.StageOpeningInbound
			} else {
				ostate.Stage = clientintf.StageWaitingOutConfirm
			}

		case ostate.Stage == clientintf.StageWaitingOutConfirm:
			runErr = c.onboardWaitChannelOpened(ctx, ostate.OutChannelID)
			if runErr == nil {
				ostate.Stage = clientintf.StageOpeningInbound
			}

		case ostate.Stage == clientintf.StageOpeningInbound:
			ostate.InChannelID, runErr = c.onboardOpenInboundChan(ctx)
			if runErr == nil {
				ostate.Stage = clientintf.StageInitialKX
			}

		case ostate.Stage == clientintf.StageInitialKX:
			runErr = c.onboardInitialKX(ctx, ostate)
			if runErr == nil {
				ostate.Stage = clientintf.StageOnboardDone
				c.log.Infof("Onboarding is done")
			}

		default:
			runErr = fmt.Errorf("unknown onboarding stage %q", ostate.Stage)
		}

		c.log.Debugf("Onboarding at stage %s with err %v", ostate.Stage, runErr)

		// If this stage was completed without errors, update the db.
		if runErr == nil {
			runErr = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
				if ostate.Stage == clientintf.StageOnboardDone {
					return c.db.RemoveOnboardState(tx)
				}
				return c.db.UpdateOnboardState(tx, &ostate)
			})
		}

		// Notify the user of any state changes, except context.Canceled
		// which is triggered by the user.
		if !errors.Is(runErr, context.Canceled) {
			c.ntfns.notifyOnOnboardStateChanged(ostate, runErr)
		}
	}

	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		c.log.Errorf("Onboarding errored: %v", runErr)
	}

	c.onboardMtx.Lock()
	c.onboardRunning = false
	c.onboardMtx.Unlock()

	return runErr
}

// restartOnboarding restarts the client's onboarding procedure (if there is
// one).
func (c *Client) restartOnboarding(ctx context.Context) error {
	<-c.abLoaded
	var ostate clientintf.OnboardState
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		ostate, err = c.db.ReadOnboardState(tx)
		return err
	})
	if errors.Is(err, clientdb.ErrNotFound) {
		// No onboard to restart.
		return nil
	}
	if err != nil {
		return err
	}
	c.log.Infof("Restarting onboarding procedure at stage %s", ostate.Stage)

	// Error is ignored because it's logged inside runOnboarding().
	_ = c.runOnboarding(ctx, ostate)
	return nil
}
