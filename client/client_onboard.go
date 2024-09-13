package client

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/decred/dcrlnd/lnrpc/chainrpc"
	"github.com/decred/dcrlnd/lnrpc/walletrpc"
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
		if err != nil {
			return err
		}

		switch ostate.Stage {
		case clientintf.StageInviteUnpaid, clientintf.StageInviteFetchTimeout:
			// Switch back to the fetching invite stage.
			ostate.Stage = clientintf.StageFetchingInvite
		}

		return c.db.UpdateOnboardState(tx, &ostate)
	})
	if err != nil {
		return err
	}
	c.log.Infof("Retrying onboarding attempt from stage %s", ostate.Stage)
	c.ntfns.notifyOnOnboardStateChanged(ostate, nil) // Initial state
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
func (c *Client) onboardOpenOutboundChan(ctx context.Context, onchainAmount dcrutil.Amount) (string, bool, uint32, error) {
	pc, ok := c.pc.(*DcrlnPaymentClient)
	if !ok {
		return "", false, 0, fmt.Errorf("payment client is not a dcrlnd payment client")
	}

	lnRPC := pc.LNRPC()
	info, err := lnRPC.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		return "", false, 0, err
	}

	var key, server string
	if len(info.Chains) == 0 || info.Chains[0].Chain != "decred" {
		return "", false, 0, fmt.Errorf("not connected to decred LN")
	}
	switch info.Chains[0].Network {
	case "mainnet":
		key = "03bd03386d7b2efe80ae46d6c8cfcfdfcf9c9297a465ac0d48c110d11ae58ed509"
		server = "hub0.bisonrelay.org:9735"
	case "simnet":
		key = "029398ddb14e4b3cb92fc64d61fcaaa2f3b590951b0b05ba1ecc04a7504d333213"
		server = "127.0.0.1:20202"
		if runtime.GOOS == "android" {
			server = "10.0.2.2:20202" // Proxy from emulator to local machine.
		}
	default:
		return "", false, 0, fmt.Errorf("network %q does not have default hub", info.Chains[0].Network)
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
		return "", false, 0, err
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
		return "", false, 0, fmt.Errorf("unable to decode pubkey: %w", err)
	}

	ocr := lnrpc.OpenChannelRequest{
		NodePubkey:         npk,
		LocalFundingAmount: int64(fundingAmt),
		PushAtoms:          0,
	}
	openStream, err := lnRPC.OpenChannel(ctx, &ocr)
	if err != nil {
		return "", false, 0, err
	}

	res, err := openStream.Recv()
	if err != nil {
		return "", false, 0, err
	}

	var cp string
	var opened bool
	switch updt := res.Update.(type) {
	case *lnrpc.OpenStatusUpdate_ChanPending:
		tx, err := chainhash.NewHash(updt.ChanPending.Txid)
		if err != nil {
			return "", false, 0, err
		}
		cp = fmt.Sprintf("%s:%d", tx, updt.ChanPending.OutputIndex)
		c.log.Infof("Outbound channel pending in output %s", cp)

	case *lnrpc.OpenStatusUpdate_ChanOpen:
		cp = chanPointToStr(updt.ChanOpen.ChannelPoint)
		opened = true
		c.log.Infof("Outbound channel opened in output %s", cp)

	default:
		return "", false, 0, fmt.Errorf("unknown OpenStatusUpdate type %T", res.Update)
	}

	// Height where to start looking for the channel to be mined.
	heightHint := info.BlockHeight - 1
	return cp, opened, heightHint, nil
}

// onboardWaitOutChannelMined waits until the outbound channel has at least one
// confirmation.
func (c *Client) onboardWaitOutChannelMined(ctx context.Context, cp string, heightHint uint32) (uint32, int32, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pc, ok := c.pc.(*DcrlnPaymentClient)
	if !ok {
		return 0, 0, fmt.Errorf("payment client is not a dcrlnd payment client")
	}

	txid, err := chainhash.NewHashFromStr(cp[:64])
	if err != nil {
		return 0, 0, err
	}
	outputIdx, err := strconv.Atoi(cp[65:])
	if err != nil {
		return 0, 0, err
	}

	walletTx, err := pc.LNWallet().GetWalletTx(ctx, &walletrpc.GetWalletTxRequest{Txid: txid[:]})
	if err != nil {
		return 0, 0, fmt.Errorf("Error on getWalletTx: %w", err)
	}
	tx := wire.NewMsgTx()
	if err := tx.FromBytes(walletTx.RawTx); err != nil {
		return 0, 0, err
	}
	if outputIdx >= len(tx.TxOut) {
		return 0, 0, fmt.Errorf("outputIdx >= len(tx.TxOut)")
	}

	outScript := tx.TxOut[outputIdx].PkScript
	chanCapacity := dcrutil.Amount(tx.TxOut[outputIdx].Value)
	lnChain := pc.LNChain()
	confReq := &chainrpc.ConfRequest{
		Txid:       txid[:],
		NumConfs:   1,
		Script:     outScript,
		HeightHint: heightHint,
	}
	stream, err := lnChain.RegisterConfirmationsNtfn(ctx, confReq)
	if err != nil {
		return 0, 0, fmt.Errorf("error waiting for confs: %w", err)
	}

	for {
		confEvent, err := stream.Recv()
		if err != nil {
			return 0, 0, err
		}
		if confDet := confEvent.GetConf(); confDet != nil {
			confsNeeded := int32(confsNeededForChanSize(chanCapacity))
			confsGotten := int32(1)
			confsLeft := confsNeeded - confsGotten
			c.log.Infof("Oubound channel %s confirmed at height %d "+
				"with %d confs left", cp, confDet.BlockHeight,
				confsLeft)
			return confDet.BlockHeight, confsLeft, nil
		}
	}
}

// onboardWaitChannelOpened waits until a channel is opened to the local node.
func (c *Client) onboardWaitChannelOpened(ctx context.Context, cp string, minedHeight uint32) (bool, int32, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pc, ok := c.pc.(*DcrlnPaymentClient)
	if !ok {
		return false, 0, fmt.Errorf("payment client is not a dcrlnd payment client")
	}

	lnRPC := pc.LNRPC()

	// Subscribe to fetch channel events. This is done first to catch
	// channels opened between the other checks.
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	eventsStream, err := lnRPC.SubscribeChannelEvents(streamCtx, &lnrpc.ChannelEventSubscription{})
	if err != nil {
		return false, 0, err
	}

	// See if the channel is already opened.
	listRes, err := lnRPC.ListChannels(ctx, &lnrpc.ListChannelsRequest{})
	if err != nil {
		return false, 0, err
	}
	for _, c := range listRes.Channels {
		if c.ChannelPoint == cp {
			// Already opened.
			return true, 0, nil
		}
	}

	// See if the channel is pending.
	isPending := false
	var chanCapacity dcrutil.Amount
	pendingRes, err := lnRPC.PendingChannels(ctx, &lnrpc.PendingChannelsRequest{})
	if err != nil {
		return false, 0, err
	}
	for _, ch := range pendingRes.PendingOpenChannels {
		if ch.Channel.ChannelPoint == cp {
			chanCapacity = dcrutil.Amount(ch.Channel.Capacity)
			isPending = true
			break
		}
	}
	if !isPending {
		return false, 0, fmt.Errorf("channel %s is not pending", cp)
	}

	// Wait for one of either a channel event or a new block.
	chanBlock, chanEvent := make(chan interface{}, 1), make(chan interface{}, 1)
	go func() {
		_, height, err := pc.WaitNextBlock(ctx)
		if err != nil {
			chanBlock <- err
		} else {
			// Delay sending this to give a chance for the channel
			// to complete first.
			time.Sleep(250 * time.Millisecond)
			chanBlock <- height
		}
	}()
	go func() {
		for {
			event, err := eventsStream.Recv()
			if err != nil {
				chanEvent <- err
				return
			}

			opened := event.GetOpenChannel()
			if opened != nil && opened.ChannelPoint == cp {
				chanEvent <- event
				return
			}
		}
	}()

	select {
	case blockEvent := <-chanBlock:
		if err, ok := blockEvent.(error); ok {
			return false, 0, err
		} else {
			// New block, increasing confirmation. Calc how many
			// confs are left.
			blockHeight := blockEvent.(uint32)
			confsNeeded := int32(confsNeededForChanSize(chanCapacity))
			confsGotten := int32(blockHeight - minedHeight + 1)
			if minedHeight == 0 {
				// Only onboarding started in old versions have
				// minedHeight == 0.
				confsGotten = 1
			}
			confsLeft := confsNeeded - confsGotten
			c.log.Infof("Onboarding outbound channel %s mined at "+
				"%d current height %d confs needed %d confs "+
				"left %d", cp, minedHeight, blockHeight,
				confsNeeded, confsLeft)
			return false, confsLeft, nil
		}
	case event := <-chanEvent:
		if err, ok := event.(error); ok {
			return false, 0, err
		}
	case <-ctx.Done():
		return false, 0, ctx.Err()
	}

	// After opening, wait an additional time to ensure channel operations
	// are started.
	select {
	case <-time.After(5 * time.Second):
		return true, 0, nil
	case <-ctx.Done():
		return false, 0, ctx.Err()
	}
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
		server = "https://hub0.bisonrelay.org:9130"
		cert = `-----BEGIN CERTIFICATE-----
MIIBwzCCAWigAwIBAgIQJNKWfgRSQnnMdBwKsVshhTAKBggqhkjOPQQDAjAxMREw
DwYDVQQKEwhkY3JsbmxwZDEcMBoGA1UEAxMTaHViMC5iaXNvbnJlbGF5Lm9yZzAe
Fw0yNDA5MTIxNTMyNTVaFw0zNDA5MTExNTMyNTVaMDExETAPBgNVBAoTCGRjcmxu
bHBkMRwwGgYDVQQDExNodWIwLmJpc29ucmVsYXkub3JnMFkwEwYHKoZIzj0CAQYI
KoZIzj0DAQcDQgAE8BvBcDlzJs+DLRHa08bLVx1ya9S+PX+b7obfhq45VdkenSNt
xk9OJZUGnpTkDbt1CBLjQg6RRqYkADYviCuDfaNiMGAwDgYDVR0PAQH/BAQDAgKE
MA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFBkc97rEXLNm3S/166Q7OqOoBuwd
MB4GA1UdEQQXMBWCE2h1YjAuYmlzb25yZWxheS5vcmcwCgYIKoZIzj0EAwIDSQAw
RgIhAKW0WpOpb0HyXofI1ML0Yu29NqU+WNwyOVzD9IlOluerAiEA84ltFlil8D1i
L6izsBzTqk6GKYSfl095BKOGyIrT+1c=
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
		forceUpdateDB := false

		switch {
		case canceled(ctx):
			runErr = ctx.Err()

		case ostate.Stage == clientintf.StageFetchingInvite && ostate.Key == nil:
			runErr = fmt.Errorf("empty paid invite key")

		case ostate.Stage == clientintf.StageInviteUnpaid:
			// When the invite was unpaid, try to fetch it again.
			fallthrough

		case ostate.Stage == clientintf.StageInviteFetchTimeout:
			// When the invite was not sent, try to fetch it again.
			fallthrough

		case ostate.Stage == clientintf.StageFetchingInvite:
			// Use a 1 minute timeout to account for cases when
			// the invite was already fetched (this is not
			// communicated by the server).
			ctx, cancel := context.WithTimeoutCause(ctx, time.Minute, errTimeoutWaitingPrepaidInvite)
			c.log.Infof("Fetching paid invite from server at RV point %s",
				ostate.Key.RVPoint())
			var invite rpc.OOBPublicIdentityInvite
			invite, runErr = c.FetchPrepaidInvite(ctx, *ostate.Key, io.Discard)
			if runErr == nil {
				ostate.Invite = &invite
				ostate.Stage = clientintf.StageRedeemingFunds
			} else if errors.Is(runErr, rpc.ErrUnpaidSubscriptionRV{}) {
				ostate.Stage = clientintf.StageInviteUnpaid

				// Save this state for restart.
				forceUpdateDB = true
			} else if errors.Is(context.Cause(ctx), errTimeoutWaitingPrepaidInvite) {
				ostate.Stage = clientintf.StageInviteFetchTimeout
				runErr = errTimeoutWaitingPrepaidInvite

				// Save this state for restart.
				forceUpdateDB = true
			}
			cancel()
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
			c.log.Infof("Waiting for tx %s to confirm to proceed with onboarding",
				ostate.RedeemTx)
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
			c.log.Infof("Attempting to open outbound channel from "+
				"%s of on-chain funds", ostate.RedeemAmount)
			var opened bool
			ostate.OutChannelID, opened, ostate.OutChannelHeightHint, runErr = c.onboardOpenOutboundChan(ctx, ostate.RedeemAmount)
			switch {
			case runErr != nil:
			case opened:
				ostate.Stage = clientintf.StageOpeningInbound
			default:
				ostate.Stage = clientintf.StageWaitingOutMined
			}

		case ostate.Stage == clientintf.StageWaitingOutMined && ostate.OutChannelHeightHint == 0:
			// Some old onboards don't have the height hint set, so
			// we can't listen on this event. Skip to next stage.
			ostate.Stage = clientintf.StageWaitingOutConfirm

		case ostate.Stage == clientintf.StageWaitingOutMined:
			// Wait until the outbound channel is mined on-chain.
			c.log.Infof("Onboarding waiting until channel %s is mined",
				ostate.OutChannelID)
			ostate.OutChannelMinedHeight, ostate.OutChannelConfsLeft, runErr = c.onboardWaitOutChannelMined(ctx,
				ostate.OutChannelID, ostate.OutChannelHeightHint)
			if runErr == nil {
				ostate.Stage = clientintf.StageWaitingOutConfirm
			}

		case ostate.Stage == clientintf.StageWaitingOutConfirm:
			// Wait until the outbound channel is considered open
			// by the local node.
			c.log.Infof("Onboarding waiting until channel %s is confirmed open",
				ostate.OutChannelID)
			var opened bool
			opened, ostate.OutChannelConfsLeft, runErr = c.onboardWaitChannelOpened(ctx, ostate.OutChannelID,
				ostate.OutChannelMinedHeight)
			if opened && runErr == nil {
				ostate.Stage = clientintf.StageOpeningInbound
			}

		case ostate.Stage == clientintf.StageOpeningInbound:
			c.log.Infof("Onboarding attepmting to open inbound channel")
			ostate.InChannelID, runErr = c.onboardOpenInboundChan(ctx)
			if runErr == nil {
				ostate.Stage = clientintf.StageInitialKX
			}

		case ostate.Stage == clientintf.StageInitialKX:
			c.log.Infof("Onboarding attempting initial KX with %s (%q)",
				ostate.Invite.Public.Identity, ostate.Invite.Public.Nick)
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
		if runErr == nil || forceUpdateDB {
			dbErr := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
				if ostate.Stage == clientintf.StageOnboardDone {
					return c.db.RemoveOnboardState(tx)
				}
				return c.db.UpdateOnboardState(tx, &ostate)
			})
			if runErr == nil && dbErr != nil {
				runErr = dbErr
			}
		}

		// Notify the user of any state changes, except context.Canceled
		// which is triggered by the user.
		if !errors.Is(runErr, context.Canceled) {
			if runErr != nil {
				c.log.Errorf("Onboarding errored: %v", runErr)
			}
			c.ntfns.notifyOnOnboardStateChanged(ostate, runErr)
		}
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
