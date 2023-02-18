package client

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/internal/gcmcacher"
	"github.com/companyzero/bisonrelay/client/internal/lowlevel"
	"github.com/companyzero/bisonrelay/client/timestats"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
)

type TransitiveEvent string

const (
	TEMediateID      = "mediate identity"
	TERequestInvite  = "request invite"
	TEReceivedInvite = "received invite"
	TEMsgForward     = "message forward"
	TEResetRequest   = "reset request"
	TEResetReply     = "reset reply"
)

// Config holds the necessary config for instantiating a CR client.
type Config struct {
	// ReconnectDelay is how long to wait between attempts to reconnect to
	// the server.
	ReconnectDelay time.Duration

	// PayClient identifies which payment scheme the client is configured
	// to use.
	PayClient clientintf.PaymentClient

	// LocalIDIniter is called when the client needs a new local identity.
	LocalIDIniter func(ctx context.Context) (*zkidentity.FullIdentity, error)

	// Dialer connects to the server. TLS is required.
	Dialer clientintf.Dialer

	// CompressLevel is the zlib compression level to use to compress
	// routed messages. Zero means no compression.
	CompressLevel int

	// DB instace for client operations. The client will call the Run()
	// method of the DB instance itself.
	DB *clientdb.DB

	// CertConfirmer must return nil if the given TLS certificate (outer
	// cert) and public key (inner cert) are accepted or an error if it any
	// of them are rejected.
	CertConfirmer clientintf.CertConfirmer

	// Logger is a function that generates loggers for each of the client's
	// subsystems.
	Logger func(subsys string) slog.Logger

	// LogPings indicates whether to log messages related to pings between
	// the client and the server.
	LogPings bool

	// CheckServerSession is called after a server session is established
	// but before ServerSessionChanged is called and allows clients to check
	// whether the connection is acceptable or if other preconditions are
	// met before continuing to connect with the specified server.
	//
	// If this callback is non nil and returns an error, the connection
	// is dropped and another attempt at the connection is made.
	//
	// If the passed connCtx is canceled, this means the connection was
	// closed (either by the remote end or by the local end).
	CheckServerSession func(connCtx context.Context, lnNode string) error

	Notifications *NotificationManager

	// ServerSessionChanged is called indicating that the connection to the
	// server changed to the specified state (either connected or not).
	//
	// The push and subscription rates are specified in milliatoms/byte.
	ServerSessionChanged func(connected bool, pushRate, subRate, expirationDays uint64)

	// GCInviteHandler is called when the user received an invitation to
	// join a group chat from a remote user.
	GCInviteHandler func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite)

	// GCJoinHandler is called when a user has joined a GC we administer.
	GCJoinHandler func(user *RemoteUser, gc clientdb.GCAddressBookEntry)

	// GCListUpdated is called when we receive remote updates for a GC from
	// the GC admin.
	GCListUpdated func(gc clientdb.GCAddressBookEntry)

	// GCUserParted is called when a user was removed from a GC. Kicked
	// signals whether it was the user that left or whether the admin
	// kicked the user.
	GCUserParted func(gcid GCID, uid UserID, reason string, kicked bool)

	// GCKilled is called when the given GC is dissolved by its admin.
	GCKilled func(gcid GCID, reason string)

	// GCWithUnkxdMember is called when an attempt to send a GC message
	// failed due to a GC member being unkxd with the local client.
	GCWithUnkxdMember func(gcid GCID, uid UserID)

	// KXCompleted is called when a KX processed completed with a remote
	// user.
	KXCompleted func(user *RemoteUser)

	// KXSuggestion is called when a remote user sends a suggestion to KX
	// with a new user.
	KXSuggestion func(user *RemoteUser, pii zkidentity.PublicIdentity)

	// PostsListReceived is called when we receive the list of posts from
	// a remote user.
	PostsListReceived func(user *RemoteUser, postList rpc.RMListPostsReply)

	// PostReceived is called when a new post is received from a remote
	// user.
	//PostReceived func(user *RemoteUser, summary clientdb.PostSummary, post rpc.PostMetadata)

	// PostStatusReceived is called when we receive a status update for a
	// given post.
	//
	// Note: user may be nil if the status update is for a post made by the
	// local user. This can happen both for received status updates and
	// status updates made by the local client.
	//PostStatusReceived func(user *RemoteUser, pid clientintf.PostID,
	//	statusFrom UserID, status rpc.PostMetadataStatus)

	TipReceived func(user *RemoteUser, amount float64)

	// SubscriptionChanged is called whenever the given user changes its
	// subscription status with the local client (either subscribed or
	// unsubscribed).
	SubscriptionChanged func(user *RemoteUser, subscribed bool)

	// RemoteSubscriptionChanged is called whenever the local client
	// receives the result of a {Subscribe,Unsubscribe}ToPosts call. In
	// other words, it's called whenever the status of a local subscription
	// to a remote user's posts changes.
	//RemoteSubscriptionChanged func(user *RemoteUser, subscribed bool)

	// RemoteSubscriptionError is called when the local client receives a
	// reply to a {Subscribe,Unsubscribe}ToPosts call with a remote error
	// message.
	//RemoteSubscriptionError func(user *RemoteUser, wasSubscribing bool, errMsg string)

	// ContentListReceived is called when the list of content of the user is
	// received.
	ContentListReceived func(user *RemoteUser, files []clientdb.RemoteFile, listErr error)

	// FileDownloadConfirmer is called to confirm the start of a file
	// download with the user.
	FileDownloadConfirmer func(user *RemoteUser, fm rpc.FileMetadata) bool

	// FileDownloadCompleted is called whenever a download of a file has
	// completed.
	FileDownloadCompleted func(user *RemoteUser, fm rpc.FileMetadata, diskPath string)

	// FileDownloadProgress is called reporting the progress of a file
	// download process.
	FileDownloadProgress func(user *RemoteUser, fm rpc.FileMetadata, nbMissingChunks int)

	// TransitiveEvent is called whenever a request is made by source for
	// the local client to forward a message to dst.
	TransitiveEvent func(src, dst UserID, event TransitiveEvent)

	// UserBlocked is called when we blocked the specified user due to their
	// request. Note that the passed user cannot be used for messaging
	// anymore.
	UserBlocked func(user *RemoteUser)

	// KXSearchCompleted is called when KX is completed with a user that
	// was the target of a KX search.
	KXSearchCompleted func(user *RemoteUser)
}

// logger creates a logger for the given subsystem in the configured backend.
func (cfg *Config) logger(subsys string) slog.Logger {
	if cfg.Logger == nil {
		return slog.Disabled
	}

	return cfg.Logger(subsys)
}

// Client is the main state manager for a CR client connection. It attempts to
// maintain an active connection to a CR server and manages the internal state
// of a client, including remote users it's connected to.
type Client struct {
	cfg *Config
	log slog.Logger

	ctx         context.Context
	cancel      func()
	dbCtx       context.Context
	dbCtxCancel func()
	runDone     chan struct{}

	pc    clientintf.PaymentClient
	id    *zkidentity.FullIdentity
	db    *clientdb.DB
	ck    *lowlevel.ConnKeeper
	q     *lowlevel.RMQ
	rmgr  *lowlevel.RVManager
	kxl   *kxList
	rul   *remoteUserList
	gcmq  *gcmcacher.Cacher
	ntfns *NotificationManager

	// abLoaded is closed when the address book has finished loading.
	abLoaded chan struct{}

	svrLnNodeMtx sync.Mutex
	svrLnNode    string

	newUsersChan chan *RemoteUser

	// gcAliasMap maps a local gc name to a global gc id.
	gcAliasMtx sync.Mutex
	gcAliasMap map[string]zkidentity.ShortID
}

// New creates a new CR client with the given config.
func New(cfg Config) (*Client, error) {
	id := new(zkidentity.FullIdentity)

	subsDelayer := func() <-chan time.Time {
		// Delay subscriptions for 100 milliseconds to allow multiple
		// concurrent changes to be sent in a single batched update.
		return time.After(100 * time.Millisecond)
	}
	rmgrLog := cfg.logger("RVMR")
	rmgrdb := &rvManagerDBAdapter{}
	rmgr := lowlevel.NewRVManager(rmgrLog, rmgrdb, subsDelayer)

	// Wrap cert confirmer to update DB on successful confirmation from UI.
	certConfirmer := func(ctx context.Context, cs *tls.ConnectionState,
		spid *zkidentity.PublicIdentity) error {
		if err := cfg.CertConfirmer(ctx, cs, spid); err != nil {
			return err
		}
		return cfg.DB.Update(ctx, func(tx clientdb.ReadWriteTx) error {
			return cfg.DB.UpdateServerID(tx,
				cs.PeerCertificates[0].Raw, spid)
		})
	}

	ckCfg := lowlevel.ConnKeeperCfg{
		PC:                      cfg.PayClient,
		Dialer:                  cfg.Dialer,
		CertConf:                certConfirmer,
		ReconnectDelay:          cfg.ReconnectDelay,
		PingInterval:            rpc.DefaultPingInterval,
		PushedRoutedMsgsHandler: rmgr.HandlePushedRMs,
		Log:                     cfg.logger("CONN"),
		LogPings:                cfg.LogPings,
	}
	ck := lowlevel.NewConnKeeper(ckCfg)

	rmqdb := &rmqDBAdapter{}
	q := lowlevel.NewRMQ(cfg.logger("RMQU"), cfg.PayClient, id, rmqdb)
	ctx, cancel := context.WithCancel(context.Background())

	dbCtx, dbCtxCancel := context.WithCancel(context.Background())

	kxl := newKXList(q, rmgr, id, cfg.DB, ctx)
	kxl.compressLevel = cfg.CompressLevel
	kxl.dbCtx = dbCtx
	kxl.log = cfg.logger("KXLS")

	ntfns := cfg.Notifications
	if ntfns == nil {
		ntfns = NewNotificationManager()
	}

	c := &Client{
		cfg:         &cfg,
		ctx:         ctx,
		cancel:      cancel,
		runDone:     make(chan struct{}),
		dbCtx:       dbCtx,
		dbCtxCancel: dbCtxCancel,

		db:    cfg.DB,
		pc:    cfg.PayClient,
		id:    id,
		ck:    ck,
		q:     q,
		rmgr:  rmgr,
		kxl:   kxl,
		log:   cfg.logger("CLNT"),
		rul:   newRemoteUserList(),
		ntfns: ntfns,

		abLoaded:     make(chan struct{}),
		newUsersChan: make(chan *RemoteUser),
	}

	// Use the GC message cacher to collect gc messages for a few seconds
	// after restarting so that messages are displayed in the order they
	// were received by the server (vs in arbitrary order based on which
	// ratchets are updated first).
	gcmqMaxLifetime := time.Second * 10
	gcmqUpdtDelay := time.Second
	gcmqInitialDelay := time.Second * 30
	c.gcmq = gcmcacher.New(gcmqMaxLifetime, gcmqUpdtDelay, gcmqInitialDelay,
		cfg.logger("GCMQ"), c.handleDelayedGCMessages)

	rmgrdb.c = c
	rmqdb.c = c
	kxl.kxCompleted = c.kxCompleted

	return c, nil
}

func (c *Client) dbView(f func(tx clientdb.ReadTx) error) error {
	return c.db.View(c.dbCtx, f)
}

func (c *Client) dbUpdate(f func(tx clientdb.ReadWriteTx) error) error {
	return c.db.Update(c.dbCtx, f)
}

// loadLocalID loads the local ID from the database.
func (c *Client) loadLocalID(ctx context.Context) error {
	var id *zkidentity.FullIdentity
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		id, err = c.db.LocalID(tx)
		return err
	})
	if errors.Is(err, clientdb.LocalIDEmptyError) {
		id, err = c.cfg.LocalIDIniter(ctx)

		// Update the DB.
		if err == nil {
			err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
				return c.db.UpdateLocalID(tx, id)
			})
		}
	}
	if err != nil {
		return err
	}

	*c.id = *id
	zeroSlice(id.PrivateSigKey[:])
	zeroSlice(id.PrivateKey[:])

	return nil
}

func (c *Client) loadServerCert(ctx context.Context) error {
	return c.dbView(func(tx clientdb.ReadTx) error {
		tlsCert, spid, err := c.db.ServerID(tx)
		if err != nil && !errors.Is(err, clientdb.ServerIDEmptyError) {
			return err
		}
		c.ck.SetKnownServerID(tlsCert, spid)
		return nil
	})
}

func (c *Client) loadGCAliases(ctx context.Context) error {
	var gcAliasMap map[string]zkidentity.ShortID
	err := c.db.View(ctx, func(tx clientdb.ReadTx) error {
		var err error
		gcAliasMap, err = c.db.GetGCAliases(tx)
		return err
	})
	if err != nil {
		return err
	}

	c.gcAliasMtx.Lock()
	c.gcAliasMap = gcAliasMap
	c.gcAliasMtx.Unlock()
	return nil
}

// queueUnackedUserRM queues the specified unacked user RM to be sent by the RMQ.
func (c *Client) queueUnackedUserRM(ctx context.Context, unacked clientdb.UnackedRM) error {
	// Prepare the outbound RM.
	replyChan := make(chan error)
	orm := rawRM{
		pri: priorityUnacked,
		msg: unacked.Encrypted,
		rv:  unacked.RV,

		// Callback to register the paid amount.
		paidRMCB: func(amount, fees int64) {
			// Amount is set to negative due to being an
			// outbound payment.
			amount = -amount
			fees = -fees
			err := c.db.Update(c.dbCtx, func(tx clientdb.ReadWriteTx) error {
				return c.db.RecordUserPayEvent(tx,
					unacked.UID, unacked.PayEvent, amount, fees)
			})
			if err != nil {
				c.log.Warnf("Unable to store payment %d (fees %d) "+
					"of event %q to user %s: %v", amount, fees,
					unacked.PayEvent, unacked.UID, err)
			}

		},
	}

	// Queue the RM in the RMQ. Should only error when the client is
	// shutting down.
	err := c.q.QueueRM(orm, replyChan)
	if err != nil {
		return err
	}

	// Wait for the async reply from the server.
	go func() {
		var err error
		select {
		case err = <-replyChan:
		case <-ctx.Done():
			return
		}

		if err != nil {
			if !errors.Is(err, clientintf.ErrSubsysExiting) {
				c.log.Errorf("Error sending previously "+
					"unacked RM of user %s to RV %s: %v",
					unacked.UID, unacked.RV)
			}
			return
		}

		c.log.Debugf("Previously unacked user %s RM was sent to RV %s",
			unacked.UID, unacked.RV)

		// RM was sent! Remove from list of unsent.
		var removed bool
		err = c.db.Update(c.dbCtx, func(tx clientdb.ReadWriteTx) error {
			var err error
			removed, err = c.db.RemoveUserUnackedRMWithRV(tx,
				unacked.UID, unacked.RV)
			return err
		})
		if err != nil {
			c.log.Errorf("Unable to delete unacked user %s "+
				"RM: %v", unacked.UID, err)
		} else if removed {
			c.log.Debugf("Removed unacked user %s RM with "+
				"RV %s", unacked.UID, unacked.RV)
		}
	}()
	return nil
}

// queueUnackedUserRMs looks for unsent user RMs and enqueues them, so that they
// are the first ones to be relayed to the server. When this exists, this will
// usually be at most one RM (i.e. the one being sent just before the last time
// the client was executed).
func (c *Client) queueUnackedUserRMs(ctx context.Context) error {
	var unsents []clientdb.UnackedRM
	err := c.db.View(ctx, func(tx clientdb.ReadTx) error {
		var err error
		unsents, err = c.db.ListUnackedUserRMs(tx)
		return err
	})
	if err != nil {
		return err
	}
	if len(unsents) == 0 {
		c.log.Debugf("No previously unsent RMs to send")
		return nil
	}

	c.log.Infof("Sending %d previously unsent RMs", len(unsents))
	for _, unsent := range unsents {
		if err := c.queueUnackedUserRM(ctx, unsent); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) loadInitialDBData(ctx context.Context) error {
	if err := c.loadLocalID(ctx); err != nil {
		return err
	}
	if err := c.loadServerCert(ctx); err != nil {
		return err
	}
	if err := c.loadGCAliases(ctx); err != nil {
		return err
	}

	return nil
}

func (c *Client) loadAddressBook(ctx context.Context) error {
	defer func() { close(c.abLoaded) }()

	var ab []*clientdb.AddressBookEntry
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		ab, err = c.db.LoadAddressBook(tx, c.id)
		return err
	})
	if err != nil {
		return err
	}

	c.log.Debugf("Loaded %d entries from the address book", len(ab))

	for _, entry := range ab {
		_, err := c.initRemoteUser(entry.ID, entry.R, false,
			entry.MyResetRV, entry.TheirResetRV, entry.Ignored)
		if err != nil {
			c.log.Errorf("Unable to init remote user %s: %v",
				entry.ID.Identity, err)
		}
	}

	return nil
}

// cleanupPaidRVsDir cleans up the paid rvs dir of the db based on the
// server provided expirationDays parameter.
func (c *Client) cleanupPaidRVsDir(expirationDays int) {
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.CleanupPaidRVs(tx, expirationDays)
	})
	if err != nil {
		c.log.Warnf("Unable to cleanup paid rvs: %v", err)
	}
}

// cleanupPushPaymentAttempts cleans up the push payment attempts db based on
// the passed limit for payment time.
func (c *Client) cleanupPushPaymentAttempts(maxLifetime time.Duration) {
	limit := time.Now().Add(-maxLifetime)
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.CleanupPushPaymentAttempts(tx, limit)
	})
	if err != nil {
		c.log.Warnf("Unable to cleanup push payment attempts: %v", err)
	}
}

func (c *Client) NotificationManager() *NotificationManager {
	return c.ntfns
}

// PublicID is the public local identity of this client.
func (c *Client) PublicID() UserID {
	return c.id.Public.Identity
}

// LocalNick is the nick of this client.
func (c *Client) LocalNick() string {
	return c.id.Public.Nick
}

// ServerLNNode returns the LN Node ID of the server we're connected to. This
// can be empty if we're not connected to any servers.
func (c *Client) ServerLNNode() string {
	c.svrLnNodeMtx.Lock()
	res := c.svrLnNode
	c.svrLnNodeMtx.Unlock()
	return res
}

// RemainOffline requests the client to remain offline.
func (c *Client) RemainOffline() {
	c.ck.RemainOffline()
}

// GoOnline requests the client to connect to the server (if not yet connected)
// and to remain connected as long as possible (including by attempting to
// re-connect if the connection closes).
func (c *Client) GoOnline() {
	c.ck.GoOnline()
}

// RMQLen is the number of outstanding messages in the outbound routed messages
// queue.
func (c *Client) RMQLen() int {
	return c.q.Len()
}

// RVsUpToDate returns true if the subscriptions to remote RVs are up to date
// in the server.
func (c *Client) RVsUpToDate() bool {
	return c.rmgr.IsUpToDate()
}

// RMQTimingStat returns the latest timing stats for the outbound RMQ.
func (c *Client) RMQTimingStat() []timestats.Quantile {
	return c.q.TimingStats()
}

func (c *Client) AddressBook() []*AddressBookEntry {
	<-c.abLoaded
	return c.rul.addressBook()
}

func (c *Client) UserExists(id UserID) bool {
	var res bool
	err := c.dbView(func(tx clientdb.ReadTx) error {
		res = c.db.AddressBookEntryExists(tx, id)
		return nil
	})
	return res && err == nil
}

// UIDByNick returns the user ID associated with the given nick.
func (c *Client) UIDByNick(nick string) (UserID, error) {
	<-c.abLoaded
	ru, err := c.rul.byNick(nick)
	if err != nil {
		return UserID{}, err
	}
	return ru.ID(), nil
}

// UserByNick returns the user identified by the given nick. Nick may be the
// actual user nick or a prefix of the user's ID (to disambiguate between users
// with the same nick).
func (c *Client) UserByNick(nick string) (*RemoteUser, error) {
	<-c.abLoaded
	return c.rul.byNick(nick)
}

// UserByID returns the remote user of the given ID.
func (c *Client) UserByID(uid UserID) (*RemoteUser, error) {
	<-c.abLoaded
	return c.rul.byID(uid)
}

// UserNick returns the nick of the given user.
func (c *Client) UserNick(uid UserID) (string, error) {
	<-c.abLoaded
	ru, err := c.rul.byID(uid)
	if err != nil {
		return "", err
	}
	return ru.Nick(), nil
}

// PM sends a private message to the given user, identified by its public id.
// The user must have been already KX'd with for this to work.
func (c *Client) PM(uid UserID, msg string) error {
	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}

	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.LogPM(tx, uid, false, c.id.Public.Nick, msg, time.Now())
	})
	if err != nil {
		return err
	}
	return ru.sendPM(msg)
}

// maybeResetAllKXAfterConn checks whether it's needed to reset KX with all
// existing users due to the local client being offline for too long.
func (c *Client) maybeResetAllKXAfterConn(expDays int) {
	var oldConnDate time.Time
	now := time.Now()
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		oldConnDate, err = c.db.ReplaceLastConnDate(tx, now)
		return err
	})
	if err != nil {
		c.log.Errorf("Unable to replace last conn date in db: %v", err)
		return
	}

	if oldConnDate.IsZero() {
		// No old stored last conn date. Ignore.
		return
	}

	limitInterval := time.Duration(expDays) * 24 * time.Hour
	limitDate := now.Add(-limitInterval)
	if !oldConnDate.Before(limitDate) {
		c.log.Debugf("Skipping resetting all KX due to local "+
			"client offline since %s with limit date %s", oldConnDate,
			limitDate)
		return
	}

	c.ntfns.notifyOnLocalClientOfflineTooLong(oldConnDate)

	c.log.Warnf("Local client offline since %s which is before "+
		"the limit date imposed by the server message retention policy of %d "+
		"days. Resetting all KXs", oldConnDate.Format(time.RFC3339), expDays)
	res, err := c.ResetAllOldRatchets(limitInterval, nil)
	if err != nil {
		c.log.Errorf("Unable to reset all old ratchets: %v", err)
		return
	}
	c.log.Infof("Started reset KX procedures with %d users due to offline "+
		"local client", len(res))
}

// Run runs all client goroutines until the given context is canceled.
//
// Must only be called once.
func (c *Client) Run(ctx context.Context) error {
	defer func() { close(c.runDone) }()
	defer func() { c.cancel() }()

	// runctx enables canceling in case of run initialization errors.
	runctx, cancel := context.WithCancel(ctx)

	g, gctx := errgroup.WithContext(runctx)

	// Wait until the errorgroup context is done + a final shutdown delay
	// before cancelling the db ctx. This allows outstanding db ops to
	// finish while preventing new processing calls from starting.
	g.Go(func() error {
		<-gctx.Done()

		// TODO: instead of a sleep here, keep track of outstanding
		// calls that need completion and only delay until those are
		// finished.
		c.log.Tracef("Starting to wait for DB shutdown")
		time.Sleep(300 * time.Millisecond)
		c.log.Tracef("Shutting down db context")
		c.dbCtxCancel()
		return nil
	})

	// Run the DB and wait for it to initialize.
	g.Go(func() error {
		err := c.db.Run(c.dbCtx)
		if err != nil && !errors.Is(err, context.Canceled) {
			c.log.Errorf("DB errored: %v", err)
		}
		return err
	})
	select {
	case <-gctx.Done():
		cancel()
		return g.Wait()
	case <-c.db.RunStarted():
	}

	// Load initial DB data.
	if err := c.loadInitialDBData(ctx); err != nil {
		c.log.Errorf("Unable to load local ID: %v", err)
		cancel()
		_ = g.Wait()
		return err
	}

	c.log.Infof("Starting client %s", c.id.Public.Identity)

	// From now on, all initialization data has been loaded. Init
	// subsystems.
	defer cancel()

	g.Go(func() error {
		// Cancel the client-global context once one of the subsystems
		// fail.
		<-gctx.Done()
		c.cancel()
		return nil
	})

	g.Go(func() error { return c.loadAddressBook(gctx) })

	g.Go(func() error {
		err := c.ck.Run(gctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			c.log.Errorf("Failed to keep online: %v", err)
		}
		return err
	})

	g.Go(func() error {
		err := c.q.Run(gctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			c.log.Errorf("Error running RMQ: %v", err)
		}
		return err
	})
	g.Go(func() error {
		err := c.rmgr.Run(gctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			c.log.Errorf("Error running RV Manager: %v", err)
		}
		return err
	})

	// Bind session changes to the other services.
	firstConnChan := make(chan struct{})
	lastExpDays := 0
	g.Go(func() error {
		firstConn := true
		for {
			nextSess := c.ck.NextSession(gctx)
			var lnNode string
			if lnSess, ok := nextSess.(lnNodeSession); lnSess != nil && ok {
				lnNode = lnSess.LNNode()
				c.svrLnNodeMtx.Lock()
				c.svrLnNode = lnSess.LNNode()
				c.svrLnNodeMtx.Unlock()
			}

			// Let users check if this server conn is usable.
			if nextSess != nil && c.cfg.CheckServerSession != nil {
				err := c.cfg.CheckServerSession(nextSess.Context(), lnNode)
				if err != nil {
					nextSess.RequestClose(err)
					continue
				}
			}

			// Clean old unpaid RVs based on server expirationDays
			// setting if it changed.
			if nextSess != nil {
				expDays := nextSess.ExpirationDays()
				if lastExpDays != expDays {
					c.log.Infof("Cleaning up expired RVs "+
						"older than %d days", expDays)
					c.cleanupPaidRVsDir(nextSess.ExpirationDays())
					lastExpDays = expDays
				}

				c.cleanupPushPaymentAttempts(nextSess.Policy().PushPaymentLifetime)
			}

			c.gcmq.SessionChanged(nextSess != nil)
			c.rmgr.BindToSession(nextSess)
			c.q.BindToSession(nextSess)
			var pushRate, subRate, expDays uint64
			if c.cfg.ServerSessionChanged != nil {
				connected := nextSess != nil
				if nextSess != nil {
					pushRate, subRate = nextSess.PaymentRates()
					expDays = uint64(nextSess.ExpirationDays())
				}
				go c.cfg.ServerSessionChanged(connected, pushRate, subRate, expDays)
			}
			if canceled(gctx) {
				return nil
			}
			if nextSess != nil && firstConn {
				close(firstConnChan)
				firstConn = false
				kxExpiryLimit := time.Duration(expDays) * time.Hour * 24
				g.Go(func() error {
					err := c.kxl.listenAllKXs(kxExpiryLimit)
					if err != nil && !errors.Is(err, context.Canceled) {
						c.log.Errorf("Unable to listen to all KXs: %v", err)
					}
					return err
				})
			}
			if nextSess != nil {
				go c.maybeResetAllKXAfterConn(nextSess.ExpirationDays())
			}
		}
	})

	// Helper to wait for the first conn to server to happen, then wait a
	// specified time before continuing.
	waitAfterFirstConn := func(d time.Duration) error {
		// Wait for first server conn.
		select {
		case <-firstConnChan:
		case <-gctx.Done():
			return gctx.Err()
		}

		// Wait some time after that.
		select {
		case <-time.After(d):
		case <-gctx.Done():
			return gctx.Err()
		}

		return nil
	}

	// Run the remote user ratchets.
	g.Go(func() error {
		gu, guCtx := errgroup.WithContext(gctx)
	nextUser:
		for {
			select {
			case ru := <-c.newUsersChan:
				gu.Go(func() error { return ru.run(guCtx) })
			case <-guCtx.Done():
				break nextUser
			}
		}
		return gu.Wait()
	})

	// Queue encrypted but unsent user RMs. This must be done before any
	// other previously unsent messages are queued.
	queuedUnsetRMs := make(chan struct{})
	g.Go(func() error {
		err := c.queueUnackedUserRMs(ctx)
		close(queuedUnsetRMs)
		return err
	})
	select {
	case <-queuedUnsetRMs:
	case <-gctx.Done():
	}

	// Start sending unsent msgs.
	g.Go(func() error { return c.runSendQ(gctx) })

	// Start the GC message cacher.
	g.Go(func() error { return c.gcmq.Run(gctx) })

	// Restart downloads.
	g.Go(func() error {
		if err := waitAfterFirstConn(1 * time.Second); err != nil {
			return err
		}
		return c.restartDownloads(gctx)
	})

	// Restart uploads.
	g.Go(func() error {
		if err := waitAfterFirstConn(1 * time.Second); err != nil {
			return err
		}
		return c.restartUploads(gctx)
	})

	// Clear old mediate id requests.
	g.Go(func() error {
		c.clearOldMediateIDs()
		return nil
	})

	return g.Wait()
}
