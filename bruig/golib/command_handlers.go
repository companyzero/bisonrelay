package golib

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/embeddeddcrlnd"
	"github.com/companyzero/bisonrelay/lockfile"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/davecgh/go-spew/spew"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrlnd/build"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/decred/dcrlnd/lnrpc/initchainsyncrpc"
	lpclient "github.com/decred/dcrlnlpd/client"
	"github.com/decred/slog"
)

type clientCtx struct {
	c      *client.Client
	lnpc   *client.DcrlnPaymentClient
	ctx    context.Context
	cancel func()
	runMtx sync.Mutex
	runErr error
	log    slog.Logger

	// skipWalletCheckChan is called if we should skip the next wallet
	// check.
	skipWalletCheckChan chan struct{}

	initIDChan   chan IDInit
	certConfChan chan bool

	// confirmPayReqRecvChan is written to by the user to confirm or deny
	// paying to open a chan.
	confirmPayReqRecvChan chan bool

	// downloadConfChans tracks confirmation channels about downloads that
	// are about to be initiated.
	downloadConfMtx   sync.Mutex
	downloadConfChans map[zkidentity.ShortID]chan bool
}

var (
	cmtx sync.Mutex
	cs   map[uint32]*clientCtx
	lfs  map[string]*lockfile.LockFile = map[string]*lockfile.LockFile{}
)

func handleHello(name string) (string, error) {
	if name == "*bug" {
		return "", fmt.Errorf("name '%s' is an error", name)
	}
	return "hello " + name, nil
}

func handleInitClient(handle uint32, args InitClient) error {
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

	// Initialize DB.
	db, err := clientdb.New(clientdb.Config{
		Root:          args.DBRoot,
		MsgsRoot:      args.MsgsRoot,
		DownloadsRoot: args.DownloadsDir,
		Logger:        logBknd.logger("FDDB"),
		ChunkSize:     rpc.MaxChunkSize,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize DB: %v", err)
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

	initIDChan := make(chan IDInit)
	certConfChan := make(chan bool)

	var c *client.Client
	var cctx *clientCtx

	ntfns := client.NewNotificationManager()
	ntfns.Register(client.OnPMNtfn(func(user *client.RemoteUser, msg rpc.RMPrivateMessage, ts time.Time) {
		// TODO: replace PM{} for types.ReceivedPM{}.
		pm := PM{UID: user.ID(), Msg: msg.Message, TimeStamp: ts.Unix()}
		notify(NTPM, pm, nil)
	},
	))

	// GCM must be sync to order correctly on startup.
	ntfns.RegisterSync(client.OnGCMNtfn(func(user *client.RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		// TODO: replace GCMessage{} for types.ReceivedGCMsg{}.
		gcm := GCMessage{
			SenderUID: user.ID(),
			ID:        msg.ID.String(),
			Msg:       msg.Message,
			TimeStamp: ts.Unix(),
		}
		notify(NTGCMessage, gcm, nil)
	}))

	ntfns.Register(client.OnRemoteSubscriptionChangedNtfn(func(user *client.RemoteUser, subscribed bool) {
		v := PostSubscriptionResult{ID: user.ID(), WasSubRequest: subscribed}
		notify(NTRemoteSubChanged, v, nil)
	}))

	ntfns.Register(client.OnRemoteSubscriptionErrorNtfn(func(user *client.RemoteUser, wasSubscribing bool, errMsg string) {
		v := PostSubscriptionResult{
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
		pr := PostStatusReceived{
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

	ntfns.Register(client.OnKXCompleted(func(_ *clientintf.RawRVID, user *client.RemoteUser) {
		pii := user.PublicIdentity()
		notify(NTKXCompleted, remoteUserFromPII(&pii), nil)
	}))

	ntfns.Register(client.OnKXSuggested(func(invitee *client.RemoteUser, target zkidentity.PublicIdentity) {
		alreadyKnown := false
		_, err := c.UserByID(target.Identity)
		if err == nil {
			// Already KX'd with this user.
			alreadyKnown = true
		}
		ipii := invitee.PublicIdentity()
		skx := SuggestKX{
			AlreadyKnown: alreadyKnown,
			InviteeNick:  ipii.Nick,
			Invitee:      ipii.Identity,
			Target:       target.Identity,
			TargetNick:   target.Nick,
		}
		notify(NTKXSuggested, skx, nil)
	}))

	ntfns.Register(client.OnInvoiceGenFailedNtfn(func(user *client.RemoteUser, dcrAmount float64, err error) {
		ntf := InvoiceGenFailed{
			UID:       user.ID(),
			Nick:      user.Nick(),
			DcrAmount: dcrAmount,
			Err:       err.Error(),
		}
		notify(NTInvoiceGenFailed, ntf, nil)
	}))

	ntfns.Register(client.OnGCVersionWarning(func(user *client.RemoteUser, gc rpc.RMGroupList, minVersion, maxVersion uint8) {
		alias, _ := c.GetGCAlias(gc.ID)
		warn := GCVersionWarn{
			ID:         gc.ID,
			Alias:      alias,
			Version:    gc.Version,
			MinVersion: minVersion,
			MaxVersion: maxVersion,
		}
		notify(NTGCVersionWarn, warn, nil)
	}))

	ntfns.Register(client.OnInvitedToGCNtfn(func(user *client.RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		pubid := user.PublicIdentity()
		inv := GCInvitation{
			Inviter: remoteUserFromPII(&pubid),
			IID:     iid,
			Name:    invite.Name,
		}
		notify(NTInvitedToGC, inv, nil)
	}))

	ntfns.Register(client.OnGCInviteAcceptedNtfn(func(user *client.RemoteUser, gc rpc.RMGroupList) {
		inv := InviteToGC{GC: gc.ID, UID: user.ID()}
		notify(NTUserAcceptedGCInvite, inv, nil)
	}))

	ntfns.Register(client.OnJoinedGCNtfn(func(gc rpc.RMGroupList) {
		name, err := c.GetGCAlias(gc.ID)
		if err != nil {
			return
		}
		gce := GCAddressBookEntry{
			ID:      gc.ID,
			Members: gc.Members,
			Name:    name,
		}
		notify(NTGCJoined, gce, nil)
	}))

	ntfns.Register(client.OnAddedGCMembersNtfn(func(gc rpc.RMGroupList, uids []clientintf.UserID) {
		ntf := GCAddedMembers{
			ID:   gc.ID,
			UIDs: uids,
		}
		notify(NTGCAddedMembers, ntf, nil)
	}))

	ntfns.Register(client.OnGCUpgradedNtfn(func(gc rpc.RMGroupList, oldVersion uint8) {
		ntf := GCUpgradedVersion{
			ID:         gc.ID,
			OldVersion: oldVersion,
			NewVersion: gc.Version,
		}
		notify(NTGCUpgradedVersion, ntf, nil)
	}))

	ntfns.Register(client.OnGCUserPartedNtfn(func(gc zkidentity.ShortID, uid clientintf.UserID, reason string, kicked bool) {
		ntf := GCMemberParted{
			GCID:   gc,
			UID:    uid,
			Reason: reason,
			Kicked: kicked,
		}
		notify(NTGCMemberParted, ntf, nil)
	}))

	ntfns.Register(client.OnGCAdminsChangedNtfn(func(ru *client.RemoteUser, gc rpc.RMGroupList, added, removed []zkidentity.ShortID) {
		ntfn := GCAdminsChanged{
			Source:  ru.ID(),
			GCID:    gc.ID,
			Added:   added,
			Removed: removed,
		}
		notify(NTGCAdminsChanged, ntfn, nil)
	}))

	cfg := client.Config{
		DB:             db,
		Dialer:         clientintf.NetDialer(args.ServerAddr, logBknd.logger("CONN")),
		PayClient:      pc,
		Logger:         logBknd.logger,
		ReconnectDelay: 5 * time.Second,
		CompressLevel:  4,
		Notifications:  ntfns,

		CertConfirmer: func(ctx context.Context, cs *tls.ConnectionState,
			svrID *zkidentity.PublicIdentity) error {

			tlsCert := cs.PeerCertificates[0]
			sc := ServerCert{
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
			st := ServerSessState{State: ConnStateCheckingWallet}
			notify(NTServerSessChanged, st, nil)

			backoff := 10 * time.Second
			maxBackoff := 60 * time.Second
			for {
				err := client.CheckLNWalletUsable(connCtx, lnpc.LNRPC(), lnNode)
				if err == nil {
					// All good.
					cctx.log.Infof("Wallet check performed successfully!")
					return nil
				}
				cctx.log.Debugf("Wallet check failed due to: %v", err)
				cctx.log.Debugf("Performing next wallet check in %s", backoff)
				errMsg := err.Error()
				st.CheckWalletErr = &errMsg
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
				case <-connCtx.Done():
					return connCtx.Err()
				case <-cctx.ctx.Done():
					return cctx.ctx.Err()
				}

			}
		},

		ServerSessionChanged: func(connected bool, pushRate, subRate, expDays uint64) {
			state := ConnStateOffline
			if connected {
				state = ConnStateOnline
			}
			st := ServerSessState{State: state}
			notify(NTServerSessChanged, st, nil)
		},

		TipReceived: func(user *client.RemoteUser, dcrAmount float64) {
			v := PayTipArgs{UID: user.ID(), Amount: dcrAmount}
			notify(NTTipReceived, v, nil)
		},

		PostsListReceived: func(user *client.RemoteUser, postList rpc.RMListPostsReply) {
			v := UserPostList{
				UID:   user.ID(),
				Posts: postList.Posts,
			}
			notify(NTUserPostsList, v, nil)
		},

		ContentListReceived: func(user *client.RemoteUser, files []clientdb.RemoteFile, listErr error) {
			data := UserContentList{
				UID:   user.ID(),
				Files: files,
			}
			notify(NTUserContentList, data, listErr)
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

			data := ConfirmFileDownload{
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

		FileDownloadProgress: func(user *client.RemoteUser, fm rpc.FileMetadata, nbMissingChunks int) {
			fdp := FileDownloadProgress{
				UID:             user.ID(),
				FID:             fm.MetadataHash(),
				Metadata:        fm,
				NbMissingChunks: nbMissingChunks,
			}
			notify(NTFileDownloadProgress, fdp, nil)
		},

		FileDownloadCompleted: func(user *client.RemoteUser,
			fm rpc.FileMetadata, diskPath string) {
			rf := clientdb.RemoteFile{
				FID:      fm.MetadataHash(),
				UID:      user.ID(),
				DiskPath: diskPath,
				Metadata: fm,
			}
			notify(NTFileDownloadCompleted, rf, nil)
		},
	}

	c, err = client.New(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	cctx = &clientCtx{
		c:      c,
		lnpc:   lnpc,
		ctx:    ctx,
		cancel: cancel,
		log:    logBknd.logger("GOLB"),

		skipWalletCheckChan: make(chan struct{}, 1),
		initIDChan:          initIDChan,
		certConfChan:        certConfChan,

		confirmPayReqRecvChan: make(chan bool),
		downloadConfChans:     make(map[zkidentity.ShortID]chan bool),
	}
	cs[handle] = cctx

	go func() {
		err := c.Run(ctx)
		if errors.Is(err, context.Canceled) {
			err = nil
		}
		cctx.runMtx.Lock()
		cctx.runErr = err
		cctx.runMtx.Unlock()
		notify(NTClientStopped, nil, err)
	}()

	return nil
}

func handleClientCmd(cc *clientCtx, cmd *cmd) (interface{}, error) {
	c := cc.c
	var lnc lnrpc.LightningClient
	if cc.lnpc != nil {
		lnc = cc.lnpc.LNRPC()
	}

	switch cmd.Type {
	case CTInvite:
		b := &bytes.Buffer{}
		_, err := c.WriteNewInvite(b)
		if err != nil {
			return nil, err
		}

		// Return the invite blob.
		bts := b.Bytes()
		return bts, nil

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
		return remoteUserFromPII(&invite.Public), nil

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
		var pm PM
		if err := cmd.decode(&pm); err != nil {
			return nil, err
		}

		err := c.PM(pm.UID, pm.Msg)
		return nil, err

	case CTAddressBook:
		return c.AddressBook(), nil

	case CTLocalID:
		var id IDInit
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
		_, err := c.NewGroupChat(gcName) // XXX return ID
		return nil, err

	case CTInviteToGroupChat:
		var invite InviteToGC
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
		var gcm GCMessageToSend
		if err := cmd.decode(&gcm); err != nil {
			return nil, err
		}
		return nil, c.GCMessage(gcm.GC, gcm.Msg, rpc.MessageModeNormal, nil)

	case CTListGCs:
		gcl, err := c.ListGCs()
		gcs := make([]GCAddressBookEntry, 0, len(gcl))
		if err == nil {
			for _, gc := range gcl {
				name, err := c.GetGCAlias(gc.ID)
				if err != nil {
					continue
				}
				gcs = append(gcs, GCAddressBookEntry{
					ID:      gc.ID,
					Members: gc.Members,
					Name:    name,
				})
			}
		}
		return gcs, err

	case CTGCRemoveUser:
		var args GCRemoveUserArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}

		return nil, c.GCKick(args.GC, args.UID, "kicked by user")

	case CTShareFile:
		var f ShareFileArgs
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
		_, _, err := c.ShareFile(f.Filename, uid, f.Cost, false, f.Description)
		return nil, err

	case CTUnshareFile:
		var f UnshareFileArgs
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
		var f GetRemoteFileArgs
		if err := cmd.decode(&f); err != nil {
			return nil, err
		}
		return nil, c.GetUserContent(f.UID, f.FID)

	case CTPayTip:
		var args PayTipArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		const maxAttempts = 1
		return nil, c.TipUser(args.UID, args.Amount, maxAttempts)

	case CTSubscribeToPosts:
		var args SubscribeToPosts
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
		var args ReadPostArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return c.ReadPost(args.From, args.PID)

	case CTReadPostUpdates:
		var args ReadPostArgs
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
		var args CommentPostArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.CommentPost(args.From, args.PID, args.Comment, args.Parent)

	case CTGetLocalInfo:
		res := LocalInfo{
			ID:   c.PublicID(),
			Nick: c.LocalNick(),
		}
		return res, nil

	case CTRequestMediateID:
		var args MediateIDArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.RequestMediateIdentity(args.Mediator, args.Target)

	case CTKXSearchPostAuthor:
		var args PostActionArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.KXSearchPostAuthor(args.From, args.PID)

	case CTRelayPostToAll:
		var args PostActionArgs
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
		var args GCRemoveUserArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.AddToGCBlockList(args.GC, args.UID)

	case CTGCRemoveFromBlockList:
		var args GCRemoveUserArgs
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
		var args LNPayInvoiceRequest
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
		var res LNBalances
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
		var args LNCloseChannelRequest
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
		req := &lnrpc.NewAddressRequest{Type: lnrpc.AddressType_PUBKEY_HASH}
		res, err := lnc.NewAddress(context.Background(), req)
		if err != nil {
			return nil, err
		}
		return res.Address, nil

	case CTLNRequestRecvCapacity:
		if lnc == nil {
			return nil, fmt.Errorf("LN client not initialized")
		}
		var args LNReqChannelArgs
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
				estValue := LNReqChannelEstValue{
					Amount:       estInvoice,
					ServerPolicy: policy,
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
		var args ConfirmFileDownloadReply
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
		var args SendFileArgs
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
		var args ReadPostArgs
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.GetUserPost(args.From, args.PID, true)

	case CTLocalRename:
		var args LocalRenameArgs
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

		res := make([]LastUserReceivedTime, len(times))
		for i := range times {
			res[i] = LastUserReceivedTime{
				UID:           times[i].UID,
				LastDecrypted: times[i].LastDecrypted.Unix(),
			}
		}
		return res, nil

	case CTUserRatchetDebugInfo:
		var args clientintf.UserID
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
		var args GCModifyAdmins
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
		var args SuggestKX
		if err := cmd.decode(&args); err != nil {
			return nil, err
		}
		return nil, c.SuggestKX(args.Invitee, args.Target)
	}
	return nil, nil
}

func handleLNTryExternalDcrlnd(args LNTryExternalDcrlnd) (*lnrpc.GetInfoResponse, error) {
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

func handleLNInitDcrlnd(args LNInitDcrlnd) (*LNNewWalletSeed, error) {
	lndc, err := runDcrlnd(args.RootDir, args.Network)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

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

	return &LNNewWalletSeed{
		Seed:    string(seed),
		RPCHost: lndc.RPCAddr(),
	}, nil
}

func handleLNRunDcrlnd(args LNInitDcrlnd) (string, error) {
	var err error

	ctx := context.Background()
	currentLndcMtx.Lock()
	lndc := currentLndc
	currentLndcMtx.Unlock()
	if lndc == nil {
		lndc, err = runDcrlnd(args.RootDir, args.Network)
	}
	if err != nil {
		return "", err
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	lf, err := lockfile.Create(ctx, filePath)
	cancel()
	if err != nil {
		return fmt.Errorf("unable to create lockfile %q: %v", filePath, err)
	}
	cmtx.Lock()
	lfs[filePath] = lf
	cmtx.Unlock()
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
