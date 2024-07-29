package golib

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/resources"
	"github.com/companyzero/bisonrelay/client/resources/simplestore"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/companyzero/bisonrelay/embeddeddcrlnd"
	"github.com/companyzero/bisonrelay/lockfile"
	"github.com/companyzero/bisonrelay/rates"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/davecgh/go-spew/spew"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrlnd/build"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/decred/dcrlnd/lnrpc/initchainsyncrpc"
	"github.com/decred/dcrlnd/lnrpc/walletrpc"
	lpclient "github.com/decred/dcrlnlpd/client"
	"github.com/decred/go-socks/socks"
	"github.com/decred/slog"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

type clientCtx struct {
	c      *client.Client
	lnpc   *client.DcrlnPaymentClient
	ctx    context.Context
	cancel func()
	runMtx sync.Mutex
	runErr error

	log     slog.Logger
	logBknd *logBackend

	// skipWalletCheckChan is called if we should skip the next wallet
	// check.
	skipWalletCheckChan chan struct{}

	initIDChan   chan iDInit
	certConfChan chan bool

	// confirmPayReqRecvChan is written to by the user to confirm or deny
	// paying to open a chan.
	confirmPayReqRecvChan chan bool

	httpClient *http.Client
	rates      *rates.Rates

	// downloadConfChans tracks confirmation channels about downloads that
	// are about to be initiated.
	downloadConfMtx   sync.Mutex
	downloadConfChans map[zkidentity.ShortID]chan bool

	// expirationDays are the expirtation days provided by the server when
	// connected
	expirationDays uint64

	serverState atomic.Value
}

var (
	cmtx sync.Mutex
	cs   map[uint32]*clientCtx
	lfs  map[string]*lockfile.LockFile = map[string]*lockfile.LockFile{}

	// The following are debug vars.
	sigUrgCount       atomic.Uint64
	isServerConnected atomic.Bool
)

func handleHello(name string) (string, error) {
	if name == "*bug" {
		return "", fmt.Errorf("name '%s' is an error", name)
	}
	return "hello " + name, nil
}

func isClientRunning(handle uint32) bool {
	cmtx.Lock()
	var res bool
	if cs != nil {
		res = cs[handle] != nil
	}
	cmtx.Unlock()
	return res
}

func handleInitClient(handle uint32, args initClient) error {
	cmtx.Lock()
	defer cmtx.Unlock()
	if cs == nil {
		cs = make(map[uint32]*clientCtx)
	}
	if cs[handle] != nil {
		return errors.New("client already initialized")
	}

	// Initialize logging.
	logBknd, err := newLogBackend(args.LogFile, args.DebugLevel)
	if err != nil {
		return err
	}
	logBknd.notify = args.WantsLogNtfns

	ctx := context.Background()

	// Initialize DB.
	db, err := clientdb.New(clientdb.Config{
		Root:          args.DBRoot,
		MsgsRoot:      args.MsgsRoot,
		DownloadsRoot: args.DownloadsDir,
		EmbedsRoot:    args.EmbedsDir,
		Logger:        logBknd.logger("FDDB"),
		ChunkSize:     rpc.MaxChunkSize,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize DB: %v", err)
	}
	// Prune embedded file cache.
	if err = db.PruneEmbeds(0); err != nil {
		return fmt.Errorf("unable to prune cache: %v", err)
	}

	// Initialize pay client.
	var pc clientintf.PaymentClient = clientintf.FreePaymentClient{}
	var lnpc *client.DcrlnPaymentClient
	if args.LNRPCHost != "" && args.LNTLSCertPath != "" && args.LNMacaroonPath != "" {
		pcCfg := client.DcrlnPaymentClientCfg{
			TLSCertPath:  args.LNTLSCertPath,
			MacaroonPath: args.LNMacaroonPath,
			Address:      args.LNRPCHost,
			Log:          logBknd.logger("LNPY"),
		}
		lnpc, err = client.NewDcrlndPaymentClient(context.Background(), pcCfg)
		if err != nil {
			return fmt.Errorf("unable to initialize dcrln pay client: %v", err)
		}
		pc = lnpc
	}

	initIDChan := make(chan iDInit)
	certConfChan := make(chan bool)

	var c *client.Client
	var cctx *clientCtx

	ntfns := client.NewNotificationManager()
	ntfns.Register(client.OnPMNtfn(func(user *client.RemoteUser, msg rpc.RMPrivateMessage, ts time.Time) {
		// TODO: replace PM{} for types.ReceivedPM{}.
		pm := pm{
			UID:       user.ID(),
			Msg:       msg.Message,
			TimeStamp: ts.Unix(),
			Nick:      user.Nick(),
		}
		notify(NTPM, pm, nil)
	},
	))

	// GCM must be sync to order correctly on startup.
	ntfns.RegisterSync(client.OnGCMNtfn(func(user *client.RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		// TODO: replace GCMessage{} for types.ReceivedGCMsg{}.
		gcm := gcMessage{
			SenderUID: user.ID(),
			ID:        msg.ID.String(),
			Msg:       msg.Message,
			TimeStamp: ts.Unix(),
		}
		notify(NTGCMessage, gcm, nil)
	}))

	ntfns.Register(client.OnPostSubscriberUpdated(func(ru *client.RemoteUser, subscribed bool) {
		v := postSubscriberUpdated{
			ID:         ru.ID(),
			Nick:       ru.Nick(),
			Subscribed: subscribed,
		}
		notify(NTPostsSubscriberUpdated, v, nil)
	}))

	ntfns.Register(client.OnRemoteSubscriptionChangedNtfn(func(user *client.RemoteUser, subscribed bool) {
		v := postSubscriptionResult{ID: user.ID(), WasSubRequest: subscribed}
		notify(NTRemoteSubChanged, v, nil)
	}))

	ntfns.Register(client.OnRemoteSubscriptionErrorNtfn(func(user *client.RemoteUser, wasSubscribing bool, errMsg string) {
		v := postSubscriptionResult{
			ID:            user.ID(),
			WasSubRequest: wasSubscribing,
			Error:         errMsg,
		}
		notify(NTRemoteSubChanged, v, nil)
	}))

	ntfns.Register(client.OnPostRcvdNtfn(func(user *client.RemoteUser,
		summary clientdb.PostSummary, pm rpc.PostMetadata) {
		notify(NTPostReceived, summary, nil)
	}))

	ntfns.Register(client.OnPostStatusRcvdNtfn(func(user *client.RemoteUser, pid clientintf.PostID,
		statusFrom clientintf.UserID, status rpc.PostMetadataStatus) {
		pr := postStatusReceived{
			PID:        pid,
			StatusFrom: statusFrom,
			Status:     status,
			Mine:       statusFrom == c.PublicID(),
		}
		if user != nil {
			pr.PostFrom = user.ID()
		} else {
			pr.PostFrom = c.PublicID()
		}
		notify(NTPostStatusReceived, pr, nil)
	}))

	ntfns.Register(client.OnKXCompleted(func(_ *clientintf.RawRVID, ru *client.RemoteUser, _ bool) {
		notify(NTKXCompleted, remoteUserFromRU(ru), nil)
	}))

	ntfns.Register(client.OnKXSuggested(func(invitee *client.RemoteUser, target zkidentity.PublicIdentity) {
		alreadyKnown := false
		targetNick := target.Nick
		targetRu, err := c.UserByID(target.Identity)
		if err == nil {
			// Already KX'd with this user.
			alreadyKnown = true
			targetNick = targetRu.Nick()
		}
		skx := suggestKX{
			AlreadyKnown: alreadyKnown,
			InviteeNick:  invitee.Nick(),
			Invitee:      invitee.ID(),
			Target:       target.Identity,
			TargetNick:   targetNick,
		}
		notify(NTKXSuggested, skx, nil)
	}))

	ntfns.Register(client.OnInvoiceGenFailedNtfn(func(user *client.RemoteUser, dcrAmount float64, err error) {
		ntf := invoiceGenFailed{
			UID:       user.ID(),
			Nick:      user.Nick(),
			DcrAmount: dcrAmount,
			Err:       err.Error(),
		}
		notify(NTInvoiceGenFailed, ntf, nil)
	}))

	ntfns.Register(client.OnGCVersionWarning(func(user *client.RemoteUser, gc rpc.RMGroupList, minVersion, maxVersion uint8) {
		alias, _ := c.GetGCAlias(gc.ID)
		warn := gcVersionWarn{
			ID:         gc.ID,
			Alias:      alias,
			Version:    gc.Version,
			MinVersion: minVersion,
			MaxVersion: maxVersion,
		}
		notify(NTGCVersionWarn, warn, nil)
	}))

	ntfns.Register(client.OnInvitedToGCNtfn(func(user *client.RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		inv := gcInvitation{
			Inviter: remoteUserFromRU(user),
			IID:     iid,
			Name:    invite.Name,
			Invite:  invite,
		}
		notify(NTInvitedToGC, inv, nil)
	}))

	ntfns.Register(client.OnGCInviteAcceptedNtfn(func(user *client.RemoteUser, gc rpc.RMGroupList) {
		inv := inviteToGC{GC: gc.ID, UID: user.ID()}
		notify(NTUserAcceptedGCInvite, inv, nil)
	}))

	ntfns.Register(client.OnJoinedGCNtfn(func(gc rpc.RMGroupList) {
		name, err := c.GetGCAlias(gc.ID)
		if err != nil {
			return
		}
		gce := gcAddressBookEntry{
			ID:      gc.ID,
			Members: gc.Members,
			Name:    name,
		}
		notify(NTGCJoined, gce, nil)
	}))

	ntfns.Register(client.OnAddedGCMembersNtfn(func(gc rpc.RMGroupList, uids []clientintf.UserID) {
		ntf := gcAddedMembers{
			ID:   gc.ID,
			UIDs: uids,
		}
		notify(NTGCAddedMembers, ntf, nil)
	}))

	ntfns.Register(client.OnGCUpgradedNtfn(func(gc rpc.RMGroupList, oldVersion uint8) {
		ntf := gcUpgradedVersion{
			ID:         gc.ID,
			OldVersion: oldVersion,
			NewVersion: gc.Version,
		}
		notify(NTGCUpgradedVersion, ntf, nil)
	}))

	ntfns.Register(client.OnGCUserPartedNtfn(func(gc zkidentity.ShortID, uid clientintf.UserID, reason string, kicked bool) {
		ntf := gcMemberParted{
			GCID:   gc,
			UID:    uid,
			Reason: reason,
			Kicked: kicked,
		}
		notify(NTGCMemberParted, ntf, nil)
	}))

	ntfns.Register(client.OnGCAdminsChangedNtfn(func(ru *client.RemoteUser, gc rpc.RMGroupList, added, removed []zkidentity.ShortID) {
		ntfn := gcAdminsChanged{
			Source:       ru.ID(),
			GCID:         gc.ID,
			Added:        added,
			Removed:      removed,
			ChangedOwner: len(added) > 0 && gc.Members[0] == added[0],
		}
		notify(NTGCAdminsChanged, ntfn, nil)
	}))

	ntfns.Register(client.OnServerSessionChangedNtfn(func(connected bool, policy clientintf.ServerPolicy) {
		state := ConnStateOffline
		if connected {
			state = ConnStateOnline
		}
		isServerConnected.Store(connected)
		st := serverSessState{State: state}
		cctx.serverState.Store(st)
		notify(NTServerSessChanged, st, nil)
		cctx.expirationDays = uint64(policy.ExpirationDays)
	}))

	ntfns.Register(client.OnTipAttemptProgressNtfn(func(ru *client.RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
		var errMsg string
		if attemptErr != nil {
			errMsg = attemptErr.Error()
		}
		ntfn := &types.TipProgressEvent{
			Uid:          ru.ID().Bytes(),
			Nick:         ru.Nick(),
			AmountMatoms: amtMAtoms,
			Completed:    completed,
			Attempt:      int32(attempt),
			AttemptErr:   errMsg,
			WillRetry:    willRetry,
		}
		notify(NTTipUserProgress, ntfn, nil)
	}))

	ntfns.Register(client.OnOnboardStateChangedNtfn(func(ostate clientintf.OnboardState, oerr error) {
		// If oerr != null, first notify the change in state, then
		// the error. This is needed because only one of notify/error
		// is triggered.
		if oerr != nil {
			notify(NTOnboardStateChanged, ostate, nil)
		}
		notify(NTOnboardStateChanged, ostate, oerr)
	}))

	ntfns.Register(client.OnResourceFetchedNtfn(func(ru *client.RemoteUser, fr clientdb.FetchedResource, sess clientdb.PageSessionOverview) {
		notify(NTResourceFetched, fr, nil)
	}))

	ntfns.Register(client.OnHandshakeStageNtfn(func(ru *client.RemoteUser, msgtype string) {
		event := handshakeStage{UID: ru.ID(), Stage: msgtype}
		notify(NTHandshakeStage, event, nil)
	}))

	ntfns.Register(client.OnTipReceivedNtfn(func(user *client.RemoteUser, amountMAtoms int64) {
		dcrAmount := float64(amountMAtoms) / 1e11
		v := payTipArgs{UID: user.ID(), Amount: dcrAmount}
		notify(NTTipReceived, v, nil)
	}))

	ntfns.Register(client.OnPostsListReceived(func(user *client.RemoteUser, postList rpc.RMListPostsReply) {
		v := userPostList{
			UID:   user.ID(),
			Posts: postList.Posts,
		}
		notify(NTUserPostsList, v, nil)
	}))

	ntfns.Register(client.OnContentListReceived(func(user *client.RemoteUser, files []clientdb.RemoteFile, listErr error) {
		data := userContentList{
			UID:   user.ID(),
			Files: files,
		}
		notify(NTUserContentList, data, listErr)
	}))

	ntfns.Register(client.OnFileDownloadProgress(func(user *client.RemoteUser, fm rpc.FileMetadata, nbMissingChunks int) {
		fdp := fileDownloadProgress{
			UID:             user.ID(),
			FID:             fm.MetadataHash(),
			Metadata:        fm,
			NbMissingChunks: nbMissingChunks,
		}
		notify(NTFileDownloadProgress, fdp, nil)
	}))

	ntfns.Register(client.OnFileDownloadCompleted(func(user *client.RemoteUser,
		fm rpc.FileMetadata, diskPath string) {
		rf := clientdb.RemoteFile{
			FID:      fm.MetadataHash(),
			UID:      user.ID(),
			DiskPath: diskPath,
			Metadata: fm,
		}
		notify(NTFileDownloadCompleted, rf, nil)
	}))

	ntfns.Register(client.OnServerUnwelcomeError(func(err error) {
		notify(NTServerUnwelcomeError, err.Error(), nil)
	}))

	ntfns.Register(client.OnProfileUpdated(func(ru *client.RemoteUser,
		ab *clientdb.AddressBookEntry, fields []client.ProfileUpdateField) {
		event := profileUpdated{
			UID:           ru.ID(),
			AbEntry:       abEntryFromDB(ab),
			UpdatedFields: fields,
		}
		notify(NTProfileUpdated, event, nil)
	}))

	// Initialize resources router.
	var sstore *simplestore.Store
	resRouter := resources.NewRouter()

	// Initialize dialer
	var d net.Dialer
	dialFunc := d.DialContext
	if args.ProxyAddr != "" {
		proxy := socks.Proxy{
			Addr:         args.ProxyAddr,
			TorIsolation: args.TorIsolation,
			Username:     args.ProxyUsername,
			Password:     args.ProxyPassword,
		}
		if args.TorIsolation && args.CircuitLimit > 0 {
			dialFunc = socks.NewPool(proxy, args.CircuitLimit).DialContext
		} else {
			dialFunc = proxy.DialContext
		}
	}
	brDialer := clientintf.WithDialer(args.ServerAddr, logBknd.logger("CONN"), dialFunc)

	cfg := client.Config{
		DB:                db,
		Dialer:            brDialer,
		PayClient:         pc,
		Logger:            logBknd.logger,
		LogPings:          args.LogPings,
		PingInterval:      time.Duration(args.PingIntervalMs) * time.Millisecond,
		ReconnectDelay:    5 * time.Second,
		CompressLevel:     4,
		Notifications:     ntfns,
		ResourcesProvider: resRouter,
		NoLoadChatHistory: args.NoLoadChatHistory,
		Collator:          collate.New(language.Und, collate.Loose),

		SendReceiveReceipts: args.SendRecvReceipts,

		AutoHandshakeInterval:         time.Duration(args.AutoHandshakeInterval) * time.Second,
		AutoRemoveIdleUsersInterval:   time.Duration(args.AutoRemoveIdleUsersInterval) * time.Second,
		AutoRemoveIdleUsersIgnoreList: args.AutoRemoveIdleUsersIgnore,
		AutoSubscribeToPosts:          args.AutoSubPosts,

		CertConfirmer: func(ctx context.Context, cs *tls.ConnectionState,
			svrID *zkidentity.PublicIdentity) error {

			tlsCert := cs.PeerCertificates[0]
			sc := serverCert{
				InnerFingerprint: svrID.Fingerprint(),
				OuterFingerprint: fingerprintDER(tlsCert),
			}
			notify(NTConfServerCert, sc, nil)
			var doConf bool
			select {
			case doConf = <-certConfChan:
			case <-ctx.Done():
				return ctx.Err()
			}

			if !doConf {
				return fmt.Errorf("user dit not accept server cert")
			}
			return nil
		},

		LocalIDIniter: func(ctx context.Context) (*zkidentity.FullIdentity, error) {
			notify(NTLocalIDNeeded, nil, nil)
			select {
			case id := <-initIDChan:
				return zkidentity.New(id.Nick, id.Name)
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},

		CheckServerSession: func(connCtx context.Context, lnNode string) error {
			if lnpc == nil {
				return fmt.Errorf("ln not initialized")
			}

			trackCtx, cancel := context.WithCancel(connCtx)
			defer cancel()
			trackLNEventsChan, err := client.TrackWalletCheckEvents(trackCtx, lnpc.LNRPC())
			if err != nil {
				return err
			}

			st := serverSessState{State: ConnStateCheckingWallet}
			cctx.serverState.Store(st)
			notify(NTServerSessChanged, st, nil)

			backoff := 10 * time.Second
			maxBackoff := 60 * time.Second
			for {
				// When Onboarding, force-accept connection so
				// that the onboarding steps may proceed.
				if ostate, _ := c.ReadOnboard(); ostate != nil {
					cctx.log.Infof("Skipping LN wallet checks due to onboarding still happening")
					return nil
				}

				err := client.CheckLNWalletUsable(connCtx, lnpc.LNRPC(), lnNode)
				if err == nil {
					// All good.
					cctx.log.Infof("Wallet check performed successfully!")
					return nil
				}
				cctx.log.Debugf("Wallet check failed due to: %v", err)
				cctx.log.Debugf("Performing next wallet check in %s", backoff)
				errMsg := err.Error()
				st := serverSessState{
					State:          ConnStateCheckingWallet,
					CheckWalletErr: &errMsg,
				}
				cctx.serverState.Store(st)
				notify(NTServerSessChanged, st, nil)

				select {
				case <-time.After(backoff):
					backoff = backoff * 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
				case <-cctx.skipWalletCheckChan:
					// Skip the check and proceed with
					// connection.
					cctx.log.Infof("Skipping wallet check as requested")
					return nil
				case <-trackLNEventsChan:
					// Force recheck.
				case <-connCtx.Done():
					return connCtx.Err()
				case <-cctx.ctx.Done():
					return cctx.ctx.Err()
				}

			}
		},

		FileDownloadConfirmer: func(user *client.RemoteUser, fm rpc.FileMetadata) bool {
			fid := fm.MetadataHash()

			cctx.downloadConfMtx.Lock()
			if _, ok := cctx.downloadConfChans[fid]; ok {
				// Already trying to get confirmation.
				cctx.downloadConfMtx.Unlock()
				return false
			}

			// Send ntf to UI and wait for confirmation.
			c := make(chan bool)
			cctx.downloadConfChans[fid] = c
			cctx.downloadConfMtx.Unlock()

			data := confirmFileDownload{
				UID:      user.ID(),
				FID:      fid,
				Metadata: fm,
			}
			notify(NTConfFileDownload, data, nil)

			select {
			case res := <-c:
				cctx.downloadConfMtx.Lock()
				delete(cctx.downloadConfChans, fid)
				cctx.downloadConfMtx.Unlock()
				return res

			case <-time.After(time.Minute):
				// Avoid never returning from here.
				return false

			case <-cctx.ctx.Done():
				return false
			}
		},
	}

	c, err = client.New(cfg)
	if err != nil {
		return err
	}

	// Bind the selected upstream resource provider.
	switch {
	case strings.HasPrefix(args.ResourcesUpstream, "http://"),
		strings.HasPrefix(args.ResourcesUpstream, "https://"):
		p := resources.NewHttpProvider(args.ResourcesUpstream)
		resRouter.BindPrefixPath([]string{}, p)
	case strings.HasPrefix(args.ResourcesUpstream, "simplestore:"):
		// Generate the template store if the path does not exist.
		path := args.ResourcesUpstream[len("simplestore:"):]
		if _, err := os.Stat(path); os.IsNotExist(err) {
			err := simplestore.WriteTemplate(path)
			if err != nil {
				return fmt.Errorf("unable to write simplestore"+
					" template: %v", err)
			}
		}

		scfg := simplestore.Config{
			Root:        path,
			Log:         logBknd.logger("SSTR"),
			LiveReload:  true, // FIXME: parametrize
			Client:      c,
			PayType:     simplestore.PayType(args.SimpleStorePayType),
			Account:     args.SimpleStoreAccount,
			ShipCharge:  args.SimpleStoreShipCharge,
			LNPayClient: lnpc,

			ExchangeRateProvider: func() float64 {
				dcrPrice, _ := cctx.rates.Get()
				return dcrPrice
			},

			OrderPlaced: func(order *simplestore.Order, msg string) {
				event := simpleStoreOrder{
					Order: *order,
					Msg:   msg,
				}
				notify(NTSimpleStoreOrderPlaced, event, nil)
			},

			StatusChanged: func(order *simplestore.Order, msg string) {
				event := simpleStoreOrder{
					Order: *order,
					Msg:   msg,
				}
				notify(NTSimpleStoreOrderPlaced, event, nil)
			},
		}
		sstore, err = simplestore.New(scfg)
		if err != nil {
			return fmt.Errorf("unable to initialize simple store: %v", err)
		}
		resRouter.BindPrefixPath([]string{}, sstore)
	case strings.HasPrefix(args.ResourcesUpstream, "pages:"):
		path := args.ResourcesUpstream[len("pages:"):]
		p := resources.NewFilesystemResource(path, logBknd.logger("PAGE"))
		resRouter.BindPrefixPath([]string{}, p)
	}

	var cancel func()
	ctx, cancel = context.WithCancel(ctx)

	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext:           dialFunc,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          2,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	r := rates.New(rates.Config{
		HTTPClient:  &httpClient,
		Log:         logBknd.logger("RATE"),
		OnionEnable: args.ProxyAddr != "",
	})
	go r.Run(ctx)

	cctx = &clientCtx{
		c:       c,
		lnpc:    lnpc,
		ctx:     ctx,
		cancel:  cancel,
		log:     logBknd.logger("GOLB"),
		logBknd: logBknd,

		skipWalletCheckChan: make(chan struct{}, 1),
		initIDChan:          initIDChan,
		certConfChan:        certConfChan,

		confirmPayReqRecvChan: make(chan bool),
		downloadConfChans:     make(map[zkidentity.ShortID]chan bool),

		httpClient: &httpClient,
		rates:      r,
	}

	cs[handle] = cctx

	if sstore != nil {
		go sstore.Run(ctx)
	}

	go func() {
		err := c.Run(ctx)
		if errors.Is(err, context.Canceled) {
			err = nil
		}
		cctx.runMtx.Lock()
		cctx.runErr = err
		cctx.runMtx.Unlock()
		cmtx.Lock()
		delete(cs, handle)
		cmtx.Unlock()
		notify(NTClientStopped, nil, err)
	}()

	go func() {
		select {
		case <-ctx.Done():
		case <-c.AddressBookLoaded():
			notify(NTAddressBookLoaded, nil, nil)
		}
	}()

	return nil
}

func handleClientCmd(cc *clientCtx, cmd *cmd) (interface{}, error) {
	c := cc.c
	var lnc lnrpc.LightningClient
	var lnWallet walletrpc.WalletKitClient
	if cc.lnpc != nil {
		lnc = cc.lnpc.LNRPC()
		lnWallet = cc.lnpc.LNWallet()
	}

	switch cmd.Type {
	case CTInvite:
		var args writeInvite
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		var funds *rpc.InviteFunds
		if args.FundAmount > 0 {
			if lnc == nil {
				return nil, fmt.Errorf("LN wallet not initialized")
			}
			var err error
			funds, err = cc.lnpc.CreateInviteFunds(cc.ctx,
				args.FundAmount, args.FundAccount)
			if err != nil {
				return nil, fmt.Errorf("unable to create invite funds: %v", err)
			}
		}

		b := &bytes.Buffer{}
		pii, pik, err := c.CreatePrepaidInvite(b, funds)
		if err != nil {
			return nil, err
		}

		if args.GCID != nil && !args.GCID.IsEmpty() {
			err := c.AddInviteOnKX(pii.InitialRendezvous, *args.GCID)
			if err != nil {
				return nil, fmt.Errorf("unable to setup post-kx "+
					"action to invite to GC: %v", err)
			}
		}

		// Return the invite blob.
		res := generatedKXInvite{
			Blob:  b.Bytes(),
			Funds: funds,
			Key:   pik,
		}
		return res, nil

	case CTDecodeInvite:
		var blob []byte
		if err := cmd.decode(&blob); err != nil {
			return nil, err
		}

		br := bytes.NewReader(blob)
		invite, err := c.ReadInvite(br)
		if err != nil {
			return nil, err
		}

		// Return the decoded invite.
		return invite, nil

	case CTAcceptInvite:
		var blob []byte
		if err := cmd.decode(&blob); err != nil {
			return nil, err
		}

		// Decode the invite that was accepted.
		br := bytes.NewReader(blob)
		invite, err := c.ReadInvite(br)
		if err != nil {
			return nil, err
		}

		err = c.AcceptInvite(invite)
		if err == nil {
			return remoteUserFromPII(&invite.Public), nil
		} else {
			return nil, err
		}

	case CTPM:
		var pm pm
		if err := cmd.decode(&pm); err != nil {
			return nil, err
		}

		err := c.PM(pm.UID, pm.Msg)
		return nil, err

	case CTAddressBook:
		ab := c.AddressBook()
		res := make([]addressBookEntry, len(ab))
		for i, entry := range ab {
			res[i] = abEntryFromDB(entry)
		}
		return res, nil

	case CTLocalID:
		var id iDInit
		if err := cmd.decode(&id); err != nil {
			return nil, err
		}
		cc.initIDChan <- id
		return nil, nil

	case CTAcceptServerCert:
		cc.certConfChan <- true
		return nil, nil

	case CTRejectServerCert:
		cc.certConfChan <- false
		return nil, nil

	case CTNewGroupChat:
		var gcName string
		if err := cmd.decode(&gcName); err != nil {
			return nil, err
		}
		return c.NewGroupChat(gcName)

	case CTInviteToGroupChat:
		var invite inviteToGC
		if err := cmd.decode(&invite); err != nil {
			return nil, err
		}
		err := c.InviteToGroupChat(invite.GC, invite.UID)
		return nil, err

	case CTAcceptGCInvite:
		var iid uint64
		if err := cmd.decode(&iid); err != nil {
			return nil, err
		}
		err := c.AcceptGroupChatInvite(iid)
		return nil, err

	case CTGetGC:
		var id zkidentity.ShortID
		if err := cmd.decode(&id); err != nil {
			return nil, err
		}
		gc, err := c.GetGC(id)
		if err != nil {
			return nil, err
		}
		gc.Name, err = c.GetGCAlias(gc.ID)
		if err != nil {
			return nil, err
		}

		return gc, nil

	case CTGCMsg:
		var gcm gcMessageToSend
		if err := cmd.decode(&gcm); err != nil {
			return nil, err
		}
		return nil, c.GCMessage(gcm.GC, gcm.Msg, rpc.MessageModeNormal, nil)

	case CTListGCs:
		gcl, err := c.ListGCs()
		gcs := make([]gcAddressBookEntry, 0, len(gcl))
		if err == nil {
			for _, gc := range gcl {
				name, err := c.GetGCAlias(gc.ID)
				if err != nil {
					continue
				}
				gcs = append(gcs, gcAddressBookEntry{
					ID:      gc.ID,
					Members: gc.Members,
					Name:    name,
				})
			}
		}
		return gcs, err

	case CTGCRemoveUser:
		var args gcRemoveUserArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		return nil, c.GCKick(args.GC, args.UID, "kicked by user")

	case CTShareFile:
		var f shareFileArgs
		if err := cmd.decode(&f); err != nil {
			return nil, err
		}
		var uid *clientintf.UserID
		if f.UID != "" {
			uid = new(clientintf.UserID)
			if err := uid.FromString(f.UID); err != nil {
				return nil, err
			}
		}
		_, _, err := c.ShareFile(f.Filename, uid, f.Cost, f.Description)
		return nil, err

	case CTUnshareFile:
		var f unshareFileArgs
		if err := cmd.decode(&f); err != nil {
			return nil, err
		}
		err := c.UnshareFile(f.FID, f.UID)
		return nil, err

	case CTListSharedFiles:
		return c.ListLocalSharedFiles()

	case CTListUserContent:
		var uid clientintf.UserID
		if err := cmd.decode(&uid); err != nil {
			return nil, err
		}
		dirs := []string{"*"}
		return nil, c.ListUserContent(uid, dirs, "")

	case CTGetUserContent:
		var f getRemoteFileArgs
		if err := cmd.decode(&f); err != nil {
			return nil, err
		}
		return nil, c.GetUserContent(f.UID, f.FID)

	case CTPayTip:
		var args payTipArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		const maxAttempts = 1
		return nil, c.TipUser(args.UID, args.Amount, maxAttempts)

	case CTSubscribeToPosts:
		var args subscribeToPosts
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		if args.FetchPost == nil || args.FetchPost.IsEmpty() {
			return nil, c.SubscribeToPosts(args.Target)
		}
		return nil, c.SubscribeToPostsAndFetch(args.Target, *args.FetchPost)

	case CTUnsubscribeToPosts:
		var uid clientintf.UserID
		if err := cmd.decode(&uid); err != nil {
			return nil, err
		}
		return nil, c.UnsubscribeToPosts(uid)

	case CTKXReset:
		var uid clientintf.UserID
		if err := cmd.decode(&uid); err != nil {
			return nil, err
		}
		return nil, c.ResetRatchet(uid)

	case CTListPosts:
		return c.ListPosts()

	case CTReadPost:
		var args readPostArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return c.ReadPost(args.From, args.PID)

	case CTReadPostUpdates:
		var args readPostArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return c.ListPostStatusUpdates(args.From, args.PID)

	case CTGetUserNick:
		var uid clientintf.UserID
		if err := cmd.decode(&uid); err != nil {
			return nil, err
		}
		return c.UserNick(uid)

	case CTCommentPost:
		var args commentPostArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return c.CommentPost(args.From, args.PID, args.Comment, args.Parent)

	case CTGetLocalInfo:
		res := localInfo{
			ID:   c.PublicID(),
			Nick: c.LocalNick(),
		}
		return res, nil

	case CTRequestMediateID:
		var args mediateIDArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.RequestMediateIdentity(args.Mediator, args.Target)

	case CTKXSearchPostAuthor:
		var args postActionArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.KXSearchPostAuthor(args.From, args.PID)

	case CTRelayPostToAll:
		var args postActionArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.RelayPostToSubscribers(args.From, args.PID)

	case CTCreatePost:
		var args string
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return c.CreatePost(args, "")

	case CTGCGetBlockList:
		var args zkidentity.ShortID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return c.GetGCBlockList(args)

	case CTGCAddToBlockList:
		var args gcRemoveUserArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.AddToGCBlockList(args.GC, args.UID)

	case CTGCRemoveFromBlockList:
		var args gcRemoveUserArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.RemoveFromGCBlockList(args.GC, args.UID)

	case CTGCPart:
		var args zkidentity.ShortID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.PartFromGC(args, "")

	case CTGCKill:
		var args zkidentity.ShortID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.KillGroupChat(args, "")

	case CTBlockUser:
		var args clientintf.UserID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.Block(args)

	case CTIgnoreUser:
		var args clientintf.UserID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.Ignore(args, true)

	case CTUnignoreUser:
		var args clientintf.UserID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.Ignore(args, false)

	case CTIsIgnored:
		var args clientintf.UserID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return c.IsIgnored(args)

	case CTListSubscribers:
		return c.ListPostSubscribers()

	case CTListSubscriptions:
		subs, err := c.ListPostSubscriptions()
		if err != nil {
			return nil, err
		}
		uids := make([]clientintf.UserID, len(subs))
		for i := range subs {
			uids[i] = subs[i].To
		}
		return uids, nil

	case CTListDownloads:
		return c.ListDownloads()

	case CTLNGetInfo:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		info, err := lnc.GetInfo(context.Background(),
			&lnrpc.GetInfoRequest{})
		if err != nil {
			return nil, err
		}
		return info, err

	case CTLNListChannels:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		return lnc.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{})

	case CTLNListPendingChannels:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		return lnc.PendingChannels(context.Background(), &lnrpc.PendingChannelsRequest{})

	case CTLNGenInvoice:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		var inv lnrpc.Invoice
		if err := cmd.decode(&inv); err != nil {
			return nil, err
		}
		return lnc.AddInvoice(context.Background(), &inv)

	case CTLNDecodeInvoice:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		var s string
		if err := cmd.decode(&s); err != nil {
			return nil, err
		}
		return lnc.DecodePayReq(context.Background(), &lnrpc.PayReqString{PayReq: s})

	case CTLNPayInvoice:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		var args lnPayInvoiceRequest
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		ctx := context.Background()
		pc, err := lnc.SendPayment(ctx)
		if err != nil {
			return nil, err
		}

		req := &lnrpc.SendRequest{
			PaymentRequest: args.PaymentRequest,
			Amt:            args.Amount,
		}
		err = pc.Send(req)
		if err != nil {
			return nil, err
		}
		res, err := pc.Recv()
		if err != nil {
			return nil, err

		}
		if res.PaymentError != "" {
			return nil, fmt.Errorf("payment error: %s", res.PaymentError)

		}
		return res, nil

	case CTLNGetServerNode:
		return c.ServerLNNode(), nil

	case CTLNQueryRoute:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		var req lnrpc.QueryRoutesRequest
		if err := cmd.decode(&req); err != nil {
			return nil, err
		}
		return lnc.QueryRoutes(context.Background(), &req)

	case CTLNGetNodeInfo:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		var req lnrpc.NodeInfoRequest
		if err := cmd.decode(&req); err != nil {
			return nil, err
		}
		return lnc.GetNodeInfo(context.Background(), &req)

	case CTLNGetBalances:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		var res lnBalances
		var err error
		res.Channel, err = lnc.ChannelBalance(context.Background(),
			&lnrpc.ChannelBalanceRequest{})
		if err != nil {
			return nil, err

		}

		res.Wallet, err = lnc.WalletBalance(context.Background(),
			&lnrpc.WalletBalanceRequest{})
		if err != nil {
			return nil, err
		}

		return res, nil

	case CTLNListPeers:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		return lnc.ListPeers(context.Background(), &lnrpc.ListPeersRequest{})

	case CTLNConnectToPeer:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		var args string
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		s := strings.Split(args, "@")
		if len(s) != 2 {
			return nil, fmt.Errorf("destination must be in the form pubkey@host")

		}
		cpr := lnrpc.ConnectPeerRequest{
			Addr: &lnrpc.LightningAddress{
				Pubkey: s[0],
				Host:   s[1],
			},
			Perm: false,
		}
		return lnc.ConnectPeer(context.Background(), &cpr)

	case CTLNDisconnectFromPeer:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		var args string
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		dpr := lnrpc.DisconnectPeerRequest{
			PubKey: args,
		}
		return lnc.DisconnectPeer(context.Background(), &dpr)

	case CTLNOpenChannel:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		var req lnrpc.OpenChannelRequest
		if err := cmd.decode(&req); err != nil {
			return nil, err
		}
		stream, err := lnc.OpenChannel(context.Background(), &req)
		if err != nil {
			return nil, err
		}
		return stream.Recv()

	case CTLNCloseChannel:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		var args lnCloseChannelRequest
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		req := &lnrpc.CloseChannelRequest{
			ChannelPoint: &lnrpc.ChannelPoint{
				FundingTxid: &lnrpc.ChannelPoint_FundingTxidStr{
					FundingTxidStr: args.ChannelPoint.Txid,
				},
				OutputIndex: uint32(args.ChannelPoint.OutputIndex),
			},
			Force: args.Force,
		}
		fmt.Println(spew.Sdump(req))
		stream, err := lnc.CloseChannel(context.Background(), req)
		if err != nil {
			return nil, err
		}
		res, err := stream.Recv()
		if err != nil {
			return nil, err
		}
		return res, nil

	case CTLNGetDepositAddr:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		var args string
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		req := &lnrpc.NewAddressRequest{
			Type:    lnrpc.AddressType_PUBKEY_HASH,
			Account: args,
		}
		res, err := lnc.NewAddress(context.Background(), req)
		if err != nil {
			return nil, err
		}
		return res.Address, nil

	case CTLNRequestRecvCapacity:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		var args lnReqChannelArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		chanPendingChan := make(chan string, 1)
		chanOpenChan := make(chan error, 1)
		lpcfg := lpclient.Config{
			LC:           cc.lnpc.LNRPC(),
			Address:      args.Server,
			Key:          args.Key,
			Certificates: []byte(args.Certificates),

			PolicyFetched: func(policy lpclient.ServerPolicy) error {
				estInvoice := lpclient.EstimatedInvoiceAmount(args.ChanSize,
					policy.ChanInvoiceFeeRate)
				cc.log.Infof("Fetched server policy. Estimated Invoice amount: %s",
					dcrutil.Amount(estInvoice))
				cc.log.Debugf("Full server policy: %#v", policy)

				// Notify UI to confirm.
				estValue := lnReqChannelEstValue{
					Amount:       estInvoice,
					ServerPolicy: policy,
					Request:      args,
				}
				notify(NTLNConfPayReqRecvChan, estValue, nil)

				select {
				case <-time.After(time.Minute):
					return fmt.Errorf("confirmation timeout")
				case res := <-cc.confirmPayReqRecvChan:
					if res {
						return nil
					}
					return fmt.Errorf("canceled by user")
				case <-cc.ctx.Done():
					return cc.ctx.Err()
				}

			},

			PayingInvoice: func(payHash string) {
				cc.log.Infof("Paying for invoice %s", payHash)
			},

			InvoicePaid: func() {
				cc.log.Infof("Invoice paid. Waiting for channel to be opened")
			},

			PendingChannel: func(channelPoint string, capacity uint64) {
				cc.log.Infof("Detected new pending channel %s with LP node with capacity %s",
					channelPoint, dcrutil.Amount(capacity))
				chanPendingChan <- channelPoint
			},
		}
		c, err := lpclient.New(lpcfg)
		if err != nil {
			return nil, fmt.Errorf("unable to create lpd client: %v", err)
		}

		go func() {
			chanOpenChan <- c.RequestChannel(cc.ctx, args.ChanSize)
		}()

		// Either RequestChannel() errors or we get a pending channel.
		select {
		case err := <-chanOpenChan:
			return nil, err
		case chanPoint := <-chanPendingChan:
			return chanPoint, nil
		}

	case CTLNConfirmPayReqRecvChan:
		var args bool
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		select {
		case cc.confirmPayReqRecvChan <- args:
			return nil, nil
		default:
			return nil, fmt.Errorf("not waiting for confirmation")
		}

	case CTConfirmFileDownload:
		var args confirmFileDownloadReply
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		cc.downloadConfMtx.Lock()
		c, ok := cc.downloadConfChans[args.FID]
		if !ok {
			cc.downloadConfMtx.Unlock()
			return nil, fmt.Errorf("file was not waiting for download confirmation")
		}
		delete(cc.downloadConfChans, args.FID)
		cc.downloadConfMtx.Unlock()

		select {
		case c <- args.Reply:
			return nil, nil
		case <-cc.ctx.Done():
			return nil, cc.ctx.Err()
		case <-time.After(time.Minute):
			return nil, fmt.Errorf("timeout trying to send reply to reply chan")
		}

	case CTFTSendFile:
		var args sendFileArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		return nil, c.SendFile(args.UID, args.Filepath)

	case CTEstimatePostSize:
		var args string
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		return clientintf.EstimatePostSize(args, "")

	case CTStopClient:
		cc.cancel()
		return nil, nil

	case CTListPayStats:
		m, err := c.ListPaymentStats()
		if err != nil {
			return nil, err
		}
		res := make(map[string]clientdb.UserPayStats, len(m))
		for k, v := range m {
			res[k.String()] = v
		}
		return res, nil

	case CTSummUserPayStats:
		var args clientintf.UserID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		return c.SummarizeUserPayStats(args)

	case CTClearPayStats:
		var args *clientintf.UserID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.ClearPayStats(args)

	case CTListUserPosts:
		var args clientintf.UserID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.ListUserPosts(args)

	case CTGetUserPost:
		var args readPostArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.GetUserPost(args.From, args.PID, true)

	case CTLocalRename:
		var args localRenameArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		if args.IsGC {
			return nil, c.AliasGC(args.ID, args.NewName)
		} else {
			return nil, c.RenameUser(args.ID, args.NewName)
		}

	case CTGoOnline:
		c.GoOnline()

	case CTRemainOffline:
		c.RemainOffline()

	case CTSkipWalletCheck:
		go func() { cc.skipWalletCheckChan <- struct{}{} }()

	case CTLNRestoreMultiSCB:
		if lnc == nil {
			return nil, fmt.Errorf("ln client not initialized")
		}
		var args []byte
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		_, err := lnc.RestoreChannelBackups(context.Background(),
			&lnrpc.RestoreChanBackupRequest{
				Backup: &lnrpc.RestoreChanBackupRequest_MultiChanBackup{
					MultiChanBackup: args,
				},
			})
		return nil, err

	case CTLNSaveMultiSCB:
		if lnc == nil {
			return nil, fmt.Errorf("ln client not initialized")
		}

		res, err := lnc.ExportAllChannelBackups(context.Background(),
			&lnrpc.ChanBackupExportRequest{})
		if err != nil {
			return nil, err
		}
		return res.MultiChanBackup.MultiChanBackup, nil

	case CTListUsersLastMsgTimes:
		times, err := c.ListUsersLastReceivedTime()
		if err != nil {
			return nil, err
		}

		res := make([]lastUserReceivedTime, len(times))
		for i := range times {
			res[i] = lastUserReceivedTime{
				UID:           times[i].UID,
				LastDecrypted: times[i].LastDecrypted.Unix(),
			}
		}
		return res, nil

	case CTUserRatchetDebugInfo:
		var args clientintf.UserID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		ru, err := c.UserByID(args)
		if err != nil {
			return nil, err
		}
		return ru.RatchetDebugInfo(), nil

	case CTResendGCList:
		var args zkidentity.ShortID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		return nil, c.ResendGCList(args, nil)

	case CTGCUpgradeVersion:
		var args zkidentity.ShortID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		gc, err := c.GetGC(args)
		if err != nil {
			return nil, err
		}
		return nil, c.UpgradeGC(args, gc.Version+1)

	case CTGCModifyAdmins:
		var args gcModifyAdmins
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.ModifyGCAdmins(args.GCID, args.NewAdmins, "")

	case CTGetKXSearch:
		var args zkidentity.ShortID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		return c.GetKXSearch(args)

	case CTSuggestKX:
		var args suggestKX
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.SuggestKX(args.Invitee, args.Target)

	case CTListAccounts:
		if lnc == nil {
			return nil, fmt.Errorf("ln client not initialized")
		}

		accts, err := lnWallet.ListAccounts(cc.ctx, &walletrpc.ListAccountsRequest{})
		if err != nil {
			return nil, err
		}

		bal, err := lnc.WalletBalance(cc.ctx, &lnrpc.WalletBalanceRequest{})
		if err != nil {
			return nil, err
		}

		res := make([]account, 0, len(accts.Accounts))
		for _, acc := range accts.Accounts {
			accBal := bal.AccountBalance[acc.Name]
			res = append(res, account{
				Name:               acc.Name,
				ConfirmedBalance:   dcrutil.Amount(accBal.ConfirmedBalance),
				UnconfirmedBalance: dcrutil.Amount(accBal.UnconfirmedBalance),
				InternalKeyCount:   acc.InternalKeyCount,
				ExternalKeyCount:   acc.ExternalKeyCount,
			})
		}
		return res, nil

	case CTCreateAccount:
		var args string
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		req := &walletrpc.DeriveNextAccountRequest{Name: args}
		_, err := lnWallet.DeriveNextAccount(cc.ctx, req)
		return nil, err

	case CTSendOnchain:
		var args sendOnChain
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		req := &lnrpc.SendCoinsRequest{
			Addr:    args.Addr,
			Amount:  int64(args.Amount),
			Account: args.FromAccount,
		}
		res, err := lnc.SendCoins(cc.ctx, req)
		return res, err

	case CTRedeeemInviteFunds:
		var args rpc.InviteFunds
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		total, txh, err := cc.lnpc.RedeemInviteFunds(cc.ctx, &args)
		if err != nil {
			return nil, err
		}
		res := redeemedInviteFunds{Txid: rpc.TxHash(txh), Total: total}
		return res, nil

	case CTFetchInvite:
		var args string
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		pik, err := clientintf.DecodePaidInviteKey(args)
		if err != nil {
			return nil, err
		}

		b := bytes.NewBuffer(nil)
		ctx, cancel := context.WithTimeout(cc.ctx, time.Second*30)
		defer cancel()
		invite, err := c.FetchPrepaidInvite(ctx, pik, b)
		if err != nil {
			return nil, err
		}

		res := invitation{
			Blob:   b.Bytes(),
			Invite: invite,
		}
		return res, nil

	case CTReadOnboard:
		return c.ReadOnboard()

	case CTRetryOnboard:
		return nil, c.RetryOnboarding()

	case CTSkipOnboardStage:
		return nil, c.SkipOnboardingStage()

	case CTStartOnboard:
		var args clientintf.PaidInviteKey
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		go func() { cc.skipWalletCheckChan <- struct{}{} }()

		return nil, c.StartOnboarding(args)

	case CTCancelOnboard:
		return nil, c.CancelOnboarding()

	case CTFetchResource:
		var args fetchResourceArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err

		}

		// If it's for a local page, fetch it directly.
		if c.PublicID() == args.UID {
			return 0, c.FetchLocalResource(args.Path, args.Metadata,
				args.Data)
		}

		if args.SessionID == 0 {
			var err error
			args.SessionID, err = c.NewPagesSession()
			if err != nil {
				return 0, err
			}
		}

		_, err := c.FetchResource(args.UID, args.Path, args.Metadata,
			args.SessionID, args.ParentPage, args.Data, args.AsyncTargetID)
		return args.SessionID, err

	case CTHandshake:
		var args clientintf.UserID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		err := c.Handshake(args)
		return nil, err

	case CTLoadUserHistory:
		var args loadUserHistory
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		chatHistory, _, err := c.ReadHistoryMessages(args.UID, args.IsGC, args.Page, args.PageNum)
		if err != nil {
			return nil, err
		}
		chatLen := args.Page
		if len(chatHistory) < args.Page {
			chatLen = len(chatHistory)
		}
		res := make([]chatLogEntry, 0, chatLen)
		for _, chatLog := range chatHistory {
			res = append(res, chatLogEntry{
				Message:   chatLog.Message,
				From:      chatLog.From,
				Internal:  chatLog.Internal,
				Timestamp: chatLog.Timestamp,
			})
		}
		return res, nil

	case CTAddressBookEntry:
		var args clientintf.UserID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		entry, err := c.AddressBookEntry(args)
		if err != nil {
			return nil, err
		}

		res := abEntryFromDB(entry)
		return res, err

	case CTResetAllOldKX:
		var age int
		if err := cmd.decode(&age); err != nil {
			return nil, err
		}
		var interval time.Duration
		if age > 0 {
			interval = time.Duration(age) * 1 * time.Second
		} else {
			// Use server expiration days if none provided
			cc.log.Debugf("Resetting all KX older than server"+
				" expiration day setting: %v days", cc.expirationDays)
			interval = time.Duration(cc.expirationDays) * 24 * time.Hour
		}
		res, err := c.ResetAllOldRatchets(interval, nil)
		if err != nil {
			return nil, err
		}
		return res, nil

	case CTTransReset:
		var args transReset
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.RequestTransitiveReset(args.Mediator, args.Target)

	case CTGCModifyOwner:
		var args gcModifyAdmins
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		if len(args.NewAdmins) != 1 {
			return nil, fmt.Errorf("new admins must have len == 1")
		}

		err := c.ModifyGCOwner(args.GCID, args.NewAdmins[0], "Changing owner")
		return nil, err

	case CTRescanWallet:
		var beginHeight int32
		if err := cmd.decode(&beginHeight); err != nil {
			return nil, err
		}
		req := &walletrpc.RescanWalletRequest{BeginHeight: beginHeight}
		s, err := lnWallet.RescanWallet(cc.ctx, req)
		if err != nil {
			return nil, err
		}

		ntf, err := s.Recv()
		for ; err == nil; ntf, err = s.Recv() {
			notify(NTRescanWalletProgress, ntf.ScannedThroughHeight, nil)
		}
		if err == nil || errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err

	case CTListTransactions:
		var args listTransactions
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		req := &lnrpc.GetTransactionsRequest{
			StartHeight: args.StartHeight,
			EndHeight:   args.EndHeight,
		}
		txs, err := lnc.GetTransactions(cc.ctx, req)
		if err != nil {
			return nil, err
		}
		res := make([]transaction, len(txs.Transactions))
		for i, tx := range txs.Transactions {
			res[i] = transaction{
				TxHash:      tx.TxHash,
				Amount:      tx.Amount,
				BlockHeight: tx.BlockHeight,
			}
		}
		return res, nil

	case CTListPostRecvReceipts:
		var args clientintf.PostID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return c.ListPostReceiveReceipts(args)

	case CTListPostCommentRecvReceipts:
		var args postAndCommentID
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return c.ListPostCommentReceiveReceipts(args.PostID, args.CommentID)

	case CTMyAvatarSet:
		var args []byte
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.UpdateLocalAvatar(args)

	case CTMyAvatarGet:
		pub := c.Public()
		return pub.Avatar, nil

	case CTZipLogs:
		var args zipLogsArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		destFile, err := os.OpenFile(args.DestPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		defer destFile.Close()

		zipFile := zip.NewWriter(destFile)

		cc.log.Infof("Zipping logs to %s (appLogs=%v, oldAppLogs=%v)",
			args.DestPath, args.IncludeGolib,
			args.IncludeGolib && !args.OnlyLastFile)

		var numFiles int

		// Prevent new log lines in main logger and log rotation while
		// creating the archive.
		//
		// TODO: sanitize log when including in archive.
		cc.logBknd.mtx.Lock()
		err = func() error {
			if args.IncludeGolib {
				w, err := createZipFileFromFile(zipFile,
					cc.logBknd.logFile, "applogs")
				if err != nil {
					return err
				}

				err = copyFileToWriter(cc.logBknd.logFile, w)
				if err != nil {
					return err
				}
				numFiles += 1
			}
			if args.IncludeGolib && !args.OnlyLastFile {
				n, err := archiveOldLogs(zipFile, cc.logBknd.logFile, "applogs")
				if err != nil {
					return err
				}
				numFiles += n
			}
			return nil
		}()
		cc.logBknd.mtx.Unlock()
		if err != nil {
			cc.log.Errorf("Unable to export logs: %s", err)
			return nil, err
		}

		lndc := runningDcrlnd()
		if lndc != nil && args.IncludeLn {
			w, err := createZipFileFromFile(zipFile, lndc.LogFullPath(), "ln-wallet")
			if err != nil {
				return nil, err
			}

			err = copyFileToWriter(lndc.LogFullPath(), w)
			if err != nil {
				return nil, err
			}

			numFiles += 1
		}
		if lndc != nil && args.IncludeLn && !args.OnlyLastFile {
			n, err := archiveOldLogs(zipFile, lndc.LogFullPath(), "ln-wallet")
			if err != nil {
				return nil, err
			}
			numFiles += n
		}

		cc.log.Infof("Zipped %d log files", numFiles)
		return nil, zipFile.Close()

	case CTNotifyServerSessionState:
		state := cc.serverState.Load().(serverSessState)
		go notify(NTServerSessChanged, state, nil)

	case CTListGCInvites:
		invites, err := cc.c.ListGCInvitesFor(nil)
		if err != nil {
			return nil, err
		}

		res := make([]gcInvitation, len(invites))
		for i := range invites {
			ru, _ := cc.c.UserByID(invites[i].User)
			res[i] = gcInvitation{
				Inviter:  remoteUserFromRU(ru),
				IID:      invites[i].ID,
				Name:     invites[i].Invite.Name,
				Invite:   invites[i].Invite,
				Accepted: invites[i].Accepted,
			}
		}
		return res, nil

	case CTCancelDownload:
		var fid zkidentity.ShortID
		if err := cmd.decode(&fid); err != nil {
			return nil, err
		}

		return nil, c.CancelDownload(fid)

	case CTSubAllPosts:
		err := c.SubscribeToAllRemotePosts(nil)
		return nil, err

	case CTLoadFetchedResource:
		var args loadFetchedResourceArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		return c.LoadFetchedResource(args.UID, args.SessionID, args.PageID)
	}
	return nil, nil

}

func handleLNTryExternalDcrlnd(args lnTryExternalDcrlnd) (*lnrpc.GetInfoResponse, error) {
	ctx := context.Background()
	pcCfg := client.DcrlnPaymentClientCfg{
		TLSCertPath:  args.TLSCertPath,
		MacaroonPath: args.MacaroonPath,
		Address:      args.RPCHost,
	}
	lnpc, err := client.NewDcrlndPaymentClient(ctx, pcCfg)
	if err != nil {
		return nil, err
	}

	return lnpc.LNRPC().GetInfo(ctx, &lnrpc.GetInfoRequest{})
}

func dcrlndSyncNotifier(update *initchainsyncrpc.ChainSyncUpdate, err error) {
	notify(NTLNInitialChainSyncUpdt, update, err)
}

func handleLNInitDcrlnd(ctx context.Context, args lnInitDcrlnd) (*lnNewWalletSeed, error) {
	var d net.Dialer
	dialFunc := d.DialContext
	if args.ProxyAddr != "" {
		proxy := socks.Proxy{
			Addr:         args.ProxyAddr,
			Username:     args.ProxyUsername,
			Password:     args.ProxyPassword,
			TorIsolation: args.TorIsolation,
		}
		if args.TorIsolation && args.CircuitLimit > 0 {
			dialFunc = socks.NewPool(proxy, args.CircuitLimit).DialContext
		} else {
			dialFunc = proxy.DialContext
		}
	}

	lndCfg := embeddeddcrlnd.Config{
		RootDir:           args.RootDir,
		Network:           args.Network,
		DebugLevel:        args.DebugLevel,
		MaxLogFiles:       defaultMaxLogFiles,
		TorAddr:           args.ProxyAddr,
		TorIsolation:      args.TorIsolation,
		DialFunc:          dialFunc,
		SyncFreeList:      args.SyncFreeList,
		AutoCompact:       args.AutoCompact,
		AutoCompactMinAge: time.Duration(args.AutoCompactMinAge) * time.Second,
		DisableRelayTx:    runtime.GOOS == "android" || runtime.GOOS == "ios",
	}
	lndc, err := runDcrlnd(ctx, lndCfg)
	if err != nil {
		return nil, err
	}

	// Try to unlock the wallet. We expect to get a errLNWalletNotFound
	// here.
	err = lndc.TryUnlock(ctx, args.Password)
	if err == nil {
		return nil, fmt.Errorf("LN wallet already initialized")
	}
	if !errors.Is(err, embeddeddcrlnd.ErrLNWalletNotFound) {
		return nil, fmt.Errorf("error attempting to unlock wallet: %v", err)
	}

	// Call the create wallet gRPC endpoint.
	seed, err := lndc.Create(ctx, args.Password, args.ExistingSeed, args.MultiChanBackup)
	if err != nil {
		return nil, err
	}

	go lndc.NotifyInitialChainSync(ctx, dcrlndSyncNotifier)

	return &lnNewWalletSeed{
		Seed:    string(seed),
		RPCHost: lndc.RPCAddr(),
	}, nil
}

func handleLNRunDcrlnd(ctx context.Context, args lnInitDcrlnd) (string, error) {
	currentLndcMtx.Lock()
	lndc := currentLndc
	currentLndcMtx.Unlock()
	if lndc == nil {
		var d net.Dialer
		dialFunc := d.DialContext
		if args.ProxyAddr != "" {
			proxy := socks.Proxy{
				Addr:         args.ProxyAddr,
				Username:     args.ProxyUsername,
				Password:     args.ProxyPassword,
				TorIsolation: args.TorIsolation,
			}
			if args.TorIsolation && args.CircuitLimit > 0 {
				dialFunc = socks.NewPool(proxy, args.CircuitLimit).DialContext
			} else {
				dialFunc = proxy.DialContext
			}
		}

		var err error
		lndCfg := embeddeddcrlnd.Config{
			RootDir:           args.RootDir,
			Network:           args.Network,
			DebugLevel:        args.DebugLevel,
			TorAddr:           args.ProxyAddr,
			MaxLogFiles:       defaultMaxLogFiles,
			TorIsolation:      args.TorIsolation,
			DialFunc:          dialFunc,
			SyncFreeList:      args.SyncFreeList,
			AutoCompact:       args.AutoCompact,
			AutoCompactMinAge: time.Duration(args.AutoCompactMinAge) * time.Second,
			DisableRelayTx:    runtime.GOOS == "android" || runtime.GOOS == "ios",
		}
		lndc, err = runDcrlnd(ctx, lndCfg)
		if err != nil {
			return "", err
		}
	}
	if err := lndc.TryUnlock(ctx, args.Password); err != nil {
		return "", err
	}

	go lndc.NotifyInitialChainSync(ctx, dcrlndSyncNotifier)
	return lndc.RPCAddr(), nil
}

func handleLNStopDcrlnd() error {
	currentLndcMtx.Lock()
	lndc := currentLndc
	currentLndcMtx.Unlock()

	if lndc == nil {
		return fmt.Errorf("dcrlnd not running")
	}

	lndc.Stop()
	go func() {
		ctx := context.Background()
		err := lndc.Wait(ctx)
		currentLndcMtx.Lock()
		currentLndc = nil
		currentLndcMtx.Unlock()
		notify(NTLNDcrlndStopped, nil, err)
	}()
	return nil
}

func handleCaptureDcrlndLog() {
	pipeR, pipeW := io.Pipe()
	build.Stdout = pipeW

	reader := bufio.NewReader(pipeR)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			notify(NTLogLine, nil, err)
			return
		}
		notify(NTLogLine, line[:len(line)-1], nil)
	}
}

func handleCreateLockFile(rootDir string) error {
	filePath := filepath.Join(rootDir, clientintf.LockFileName)

	cmtx.Lock()
	defer cmtx.Unlock()

	lf := lfs[filePath]
	if lf != nil {
		// Already running on this DB from this process.
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	lf, err := lockfile.Create(ctx, filePath)
	cancel()
	if err != nil {
		return fmt.Errorf("unable to create lockfile %q: %v", filePath, err)
	}
	lfs[filePath] = lf
	return nil
}

func handleCloseLockFile(rootDir string) error {
	filePath := filepath.Join(rootDir, clientintf.LockFileName)

	cmtx.Lock()
	lf := lfs[filePath]
	delete(lfs, filePath)
	cmtx.Unlock()

	if lf == nil {
		return fmt.Errorf("nil lockfile")
	}
	return lf.Close()
}
