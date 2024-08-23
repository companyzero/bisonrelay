package client

import (
	"context"
	"crypto/tls"
	"errors"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/internal/gcmcacher"
	"github.com/companyzero/bisonrelay/client/internal/lowlevel"
	"github.com/companyzero/bisonrelay/client/internal/singlesetmap"
	"github.com/companyzero/bisonrelay/client/resources"
	"github.com/companyzero/bisonrelay/client/timestats"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
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

	// NoLoadChatHistory indicates whether to load existing chat history from
	// chat log files.
	NoLoadChatHistory bool

	// CheckServerSession is called after a server session is established
	// but before the OnServerSessionChangedNtfn notification is called and
	// allows clients to check whether the connection is acceptable or if
	// other preconditions are met before continuing to connect with the
	// specified server.
	//
	// If this callback is non nil and returns an error, the connection
	// is dropped and another attempt at the connection is made.
	//
	// If the passed connCtx is canceled, this means the connection was
	// closed (either by the remote end or by the local end).
	CheckServerSession func(connCtx context.Context, lnNode string) error

	// Notifications tracks handlers for client events. If nil, the client
	// will initialize a new notification manager. Specifying a
	// notification manager in the config is useful to ensure no
	// notifications are lost due to race conditions in client
	// initialization.
	Notifications *NotificationManager

	// KXSuggestion is called when a remote user sends a suggestion to KX
	// with a new user.
	KXSuggestion func(user *RemoteUser, pii zkidentity.PublicIdentity)

	// FileDownloadConfirmer is called to confirm the start of a file
	// download with the user.
	FileDownloadConfirmer func(user *RemoteUser, fm rpc.FileMetadata) bool

	// TipUserRestartDelay is how long to wait after client start and
	// initial server connection to restart TipUser attempts. If unset,
	// a default value of 1 minute is used.
	TipUserRestartDelay time.Duration

	// TipUserReRequestInvoiceDelay is how long to wait to re-request an
	// invoice from the user, if one has not been received yet when
	// attempting to tip. If unset, a default value of 24 hours is used.
	TipUserReRequestInvoiceDelay time.Duration

	// TipUserMaxLifetime is the maximum amount of time an invoice will
	// be paid after received. After this delay elapses, there won't be
	// attempts to pay received invoices for a tip attempt. This delay is
	// based on the initial TipUser attempt.
	//
	// If unspecified, a default value of 72 hours is used.
	TipUserMaxLifetime time.Duration

	// TipUserPayRetryDelayFactor is the factor of the exponential delay
	// for retrying a payment when the payment error indicates a retry may
	// be possible.
	//
	// If unspecified, a default value of 12 seconds (1/5 minute) is used.
	TipUserPayRetryDelayFactor time.Duration

	// GCMQMaxLifetime is how long to wait for a message from an user,
	// after which the GCMQ considers no other messages from this user
	// will be received.
	//
	// If unspecified, a default value of 10 seconds is used.
	GCMQMaxLifetime time.Duration

	// ResourcesProvider if filled is used to respond to fetch resource
	// requests.
	ResourcesProvider resources.Provider

	// GCMQUpdtDelay is how often to check for GCMQ rules to emit messages.
	//
	// If unspecified, a default value of 1 second is used.
	GCMQUpdtDelay time.Duration

	// GCMQInitialDelay is how long to wait after the initial subscriptions
	// are done on the server to start processing GCMQ messages.
	//
	// If unspecified, a default value of 10 seconds is used.
	GCMQInitialDelay time.Duration

	// RecentMediateIDThreshold is how long to wait until attempting a new
	// mediate ID request for a given target.
	//
	// If unspecified, a default value of 7 days is used.
	RecentMediateIDThreshold time.Duration

	// UnkxdWarningTimeout is how long to wait between warnings about
	// trying to perform an action with unkxd people (for example, trying
	// to send a GC message to an unkxd GC member).
	//
	// If unspecified, a default value of 24 hours is used.
	UnkxdWarningTimeout time.Duration

	// MaxAutoKXMediateIDRequests is the max number of autokx mediate ID
	// requests to make to a particular user id.
	//
	// If unspecified, a default value of 3 is used.
	MaxAutoKXMediateIDRequests int

	// AutoHandshakeInterval is the interval after which, for any ratchets
	// that haven't been communicated with, an automatic handshake attempt
	// is made.
	AutoHandshakeInterval time.Duration

	// AutoRemoveIdleUsersInterval is the interval after which any idle
	// users (i.e. users from which no message has been received) will be
	// automatically removed from GCs the local client admins and will be
	// automatically unsubscribed from posts.
	AutoRemoveIdleUsersInterval time.Duration

	// AutoRemoveIdleUsersIgnoreList is a list of users that should NOT be
	// forcibly unsubscribed even if they are idle. The values may be an
	// user's nick or the prefix of the string representatation of its nick
	// (i.e. anything acceptable as returned by UserByNick()).
	AutoRemoveIdleUsersIgnoreList []string

	// SendReceiveReceipts flags whether to send receive receipts for all
	// domains.
	SendReceiveReceipts bool

	// AutoSubscribeToPosts flags whether to automatically subscribe to
	// posts when kx'ing for the first time with an user.
	AutoSubscribeToPosts bool

	// PingInterval sets how often to send pings to the server to keep
	// the connection open. If unset, defaults to the RPC default ping
	// interval. If negative, pings are not sent.
	PingInterval time.Duration

	// Collator is used to compare nick and other strings to determine
	// equality and sorting order.
	Collator *collate.Collator

	// GCInviteExpiration is how long a GC invitation is valid for. Defaults
	// to 7 days.
	GCInviteExpiration time.Duration
}

// logger creates a logger for the given subsystem in the configured backend.
func (cfg *Config) logger(subsys string) slog.Logger {
	if cfg.Logger == nil {
		return slog.Disabled
	}

	return cfg.Logger(subsys)
}

// setDefaults sets default options for unset/empty config fields.
func (cfg *Config) setDefaults() {
	if cfg.TipUserRestartDelay == 0 {
		cfg.TipUserRestartDelay = time.Minute
	}
	if cfg.TipUserReRequestInvoiceDelay == 0 {
		cfg.TipUserReRequestInvoiceDelay = time.Hour * 24
	}
	if cfg.TipUserMaxLifetime == 0 {
		cfg.TipUserMaxLifetime = time.Hour * 72
	}

	if cfg.TipUserPayRetryDelayFactor == 0 {
		cfg.TipUserPayRetryDelayFactor = time.Minute / 5
	}

	if cfg.RecentMediateIDThreshold == 0 {
		cfg.RecentMediateIDThreshold = time.Hour * 24 * 7
	}

	if cfg.UnkxdWarningTimeout == 0 {
		cfg.UnkxdWarningTimeout = time.Hour * 24
	}

	if cfg.MaxAutoKXMediateIDRequests == 0 {
		cfg.MaxAutoKXMediateIDRequests = 3
	}

	if cfg.PingInterval == 0 {
		cfg.PingInterval = rpc.DefaultPingInterval
	}

	if cfg.Collator == nil {
		cfg.Collator = collate.New(language.Und)
	}

	if cfg.GCInviteExpiration == 0 {
		cfg.GCInviteExpiration = time.Hour * 24 * 7
	}

	// These following GCMQ times were obtained by profiling a client
	// connected over tor to the server and may need tweaking from time to
	// time.
	if cfg.GCMQMaxLifetime == 0 {
		cfg.GCMQMaxLifetime = time.Second * 10
	}
	if cfg.GCMQUpdtDelay == 0 {
		cfg.GCMQUpdtDelay = time.Second
	}
	if cfg.GCMQInitialDelay == 0 {
		cfg.GCMQInitialDelay = time.Second * 10
	}
}

// localIdentity stores identity related data that is not modified throughout
// the lifetime of the client.
type localIdentity struct {
	id         zkidentity.ShortID
	nick       string
	name       string
	privKey    zkidentity.FixedSizeSntrupPrivateKey
	pubKey     zkidentity.FixedSizeSntrupPublicKey
	privSigKey zkidentity.FixedSizeEd25519PrivateKey
	pubSigKey  zkidentity.FixedSizeEd25519PublicKey
	digest     zkidentity.FixedSizeDigest
	signature  zkidentity.FixedSizeSignature
}

func (li *localIdentity) zero() {
	zeroSlice(li.privKey[:])
	zeroSlice(li.privSigKey[:])
	zeroSlice(li.pubKey[:])
	zeroSlice(li.pubSigKey[:])
}

func (li *localIdentity) signMessage(message []byte) zkidentity.FixedSizeSignature {
	return zkidentity.SignMessage(message, &li.privSigKey)
}

func (li *localIdentity) verifyMessage(msg []byte, sig *zkidentity.FixedSizeSignature) bool {
	return zkidentity.VerifyMessage(msg, sig, &li.pubSigKey)
}

// public returns a public identity instance with the fixed data filled.
func (li *localIdentity) public() zkidentity.PublicIdentity {
	return zkidentity.PublicIdentity{
		Name:      li.name,
		Nick:      li.nick,
		SigKey:    li.pubSigKey,
		Key:       li.pubKey,
		Identity:  li.id,
		Digest:    li.digest,
		Signature: li.signature,
	}
}

func localIdentityFromFull(id *zkidentity.FullIdentity) localIdentity {
	return localIdentity{
		privKey:    id.PrivateKey,
		privSigKey: id.PrivateSigKey,
		pubKey:     id.Public.Key,
		pubSigKey:  id.Public.SigKey,
		id:         id.Public.Identity,
		nick:       id.Public.Nick,
		name:       id.Public.Name,
		digest:     id.Public.Digest,
		signature:  id.Public.Signature,
	}
}

type localProfile struct {
	avatar []byte
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

	// localID are the unchanging properties of the local client's ID.
	// This is filled at the start of Run() and is not modified afterwards.
	localID localIdentity

	profileMtx sync.Mutex
	profile    localProfile

	pc    clientintf.PaymentClient
	db    *clientdb.DB
	ck    *lowlevel.ConnKeeper
	q     *lowlevel.RMQ
	rmgr  *lowlevel.RVManager
	kxl   *kxList
	rul   *remoteUserList
	gcmq  *gcmcacher.Cacher
	ntfns *NotificationManager

	plugins map[string]*PluginClient

	// abLoaded is closed when the address book has finished loading.
	abLoaded chan struct{}

	// firstSubDone is closed when the first subscription to remote RVs is
	// done after the client starts.
	firstSubDone chan struct{}

	svrMtx     sync.Mutex
	svrLnNode  string
	svrSession clientintf.ServerSessionIntf

	newUsersChan chan *RemoteUser

	// gcAliasMap maps a local gc name to a global gc id.
	gcAliasMtx sync.Mutex
	gcAliasMap map[string]zkidentity.ShortID

	// gcWarnedVersions tracks GCs for which the warning about an
	// incompatible version has been issued.
	gcWarnedVersions *singlesetmap.Map[zkidentity.ShortID]

	// unkxdWarnings tracks the time used to warn about unkxd remote clients
	// (for example, because they are GC members).
	unkxdWarningsMtx sync.Mutex
	unkxdWarnings    map[clientintf.UserID]time.Time

	// onboardRunning tracks whether there's a running onboard instance.
	onboardMtx        sync.Mutex
	onboardRunning    bool
	onboardCancelChan chan struct{}

	tipAttemptsChan            chan *clientdb.TipUserAttempt
	listRunningTipAttemptsChan chan chan []RunningTipUserAttempt
	tipAttemptsRunning         chan struct{}

	// filters are used to filter content so it is not presented
	// to the user.
	filtersMtx     sync.Mutex
	filters        []clientdb.ContentFilter
	filtersRegexps map[uint64]*regexp.Regexp
}

// New creates a new CR client with the given config.
func New(cfg Config) (*Client, error) {
	var c *Client

	cfg.setDefaults()

	ntfns := cfg.Notifications
	if ntfns == nil {
		ntfns = NewNotificationManager()
	}

	subsDelayer := func() <-chan time.Time {
		// Delay subscriptions for 100 milliseconds to allow multiple
		// concurrent changes to be sent in a single batched update.
		return time.After(100 * time.Millisecond)
	}
	subsDoneCB := func() {
		c.gcmq.SessionChanged(true)
		select {
		case <-c.firstSubDone:
		default:
			close(c.firstSubDone)
		}
	}
	rmgrLog := cfg.logger("RVMR")
	rmgrdb := &rvManagerDBAdapter{}
	rmgr := lowlevel.NewRVManager(rmgrLog, rmgrdb, subsDelayer, subsDoneCB)

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
		PingInterval:            cfg.PingInterval,
		PushedRoutedMsgsHandler: rmgr.HandlePushedRMs,
		Log:                     cfg.logger("CONN"),
		LogPings:                cfg.LogPings,
		OnUnwelcomeError:        ntfns.notifyServerUnwelcomeError,
	}
	ck := lowlevel.NewConnKeeper(ckCfg)

	rmqdb := &rmqDBAdapter{}
	q := lowlevel.NewRMQ(cfg.logger("RMQU"), rmqdb)
	ctx, cancel := context.WithCancel(context.Background())

	dbCtx, dbCtxCancel := context.WithCancel(context.Background())

	c = &Client{
		cfg:         &cfg,
		ctx:         ctx,
		cancel:      cancel,
		dbCtx:       dbCtx,
		dbCtxCancel: dbCtxCancel,

		db:    cfg.DB,
		pc:    cfg.PayClient,
		ck:    ck,
		q:     q,
		rmgr:  rmgr,
		log:   cfg.logger("CLNT"),
		rul:   newRemoteUserList(cfg.Collator),
		ntfns: ntfns,

		abLoaded:         make(chan struct{}),
		firstSubDone:     make(chan struct{}),
		newUsersChan:     make(chan *RemoteUser),
		gcWarnedVersions: &singlesetmap.Map[zkidentity.ShortID]{},
		unkxdWarnings:    make(map[clientintf.UserID]time.Time),
		plugins:          make(map[string]*PluginClient),

		onboardCancelChan: make(chan struct{}, 1),

		tipAttemptsChan:            make(chan *clientdb.TipUserAttempt),
		listRunningTipAttemptsChan: make(chan chan []RunningTipUserAttempt),
		tipAttemptsRunning:         make(chan struct{}),
	}

	kxl := newKXList(q, rmgr, &c.localID, c.Public, cfg.DB, ctx)
	kxl.compressLevel = cfg.CompressLevel
	kxl.dbCtx = dbCtx
	kxl.log = cfg.logger("KXLS")
	c.kxl = kxl

	// Use the GC message cacher to collect gc messages for a few seconds
	// after restarting so that messages are displayed in the order they
	// were received by the server (vs in arbitrary order based on which
	// ratchets are updated first).
	c.gcmq = gcmcacher.New(cfg.GCMQMaxLifetime, cfg.GCMQUpdtDelay, cfg.GCMQInitialDelay,
		cfg.logger("GCMQ"), c.handleDelayedGCMessages)

	rmgrdb.c = c
	rmqdb.c = c
	kxl.kxCompleted = c.kxCompleted

	return c, nil
}

// Log returns the main client logger.
func (c *Client) Logger() slog.Logger {
	return c.log
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
	if errors.Is(err, clientdb.ErrLocalIDEmpty) {
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

	c.localID = localIdentityFromFull(id)
	c.profile.avatar = id.Public.Avatar
	zeroSlice(id.PrivateSigKey[:])
	zeroSlice(id.PrivateKey[:])

	return nil
}

func (c *Client) loadServerCert(ctx context.Context) error {
	return c.dbView(func(tx clientdb.ReadTx) error {
		tlsCert, spid, err := c.db.ServerID(tx)
		if err != nil && !errors.Is(err, clientdb.ErrServerIDEmpty) {
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
	if err := c.loadContentFilters(ctx); err != nil {
		return err
	}
	if err := c.removeExpiredGCInvites(ctx); err != nil {
		return err
	}

	return nil
}

// AddressBookLoaded returns a channel that is closed when the addressbook has
// been loaded for the first time, after Run() is started.
func (c *Client) AddressBookLoaded() <-chan struct{} {
	return c.abLoaded
}

func (c *Client) loadAddressBook(ctx context.Context) error {
	defer func() { close(c.abLoaded) }()

	var ab []clientdb.AddressBookAndRatchet
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		ab, err = c.db.LoadAddressBook(tx, &c.localID.privKey)
		return err
	})
	if err != nil {
		return err
	}

	c.log.Debugf("Loaded %d entries from the address book", len(ab))

	for _, entry := range ab {
		_, _, err := c.initRemoteUser(entry.AddressBook.ID, entry.Ratchet, false,
			clientdb.RawRVID{}, entry.AddressBook.MyResetRV,
			entry.AddressBook.TheirResetRV, entry.AddressBook.Ignored,
			entry.AddressBook.NickAlias)
		if err != nil {
			c.log.Errorf("Unable to init remote user %s: %v",
				entry.AddressBook.ID.Identity, err)
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

// PublicID is the public local identity of this client. This is only available
// after the Run() method has loaded the user ID from the DB.
func (c *Client) PublicID() UserID {
	return c.localID.id
}

// Public returns the full public identity data for the local client.
func (c *Client) Public() zkidentity.PublicIdentity {
	// Fixed fields.
	public := c.localID.public()

	// Variable fields.
	c.profileMtx.Lock()
	public.Avatar = c.profile.avatar
	c.profileMtx.Unlock()

	return public
}

// LocalNick is the nick of this client. This is only available after the Run()
// method has loaded the user ID from the DB.
func (c *Client) LocalNick() string {
	return c.localID.nick
}

// ServerLNNode returns the LN Node ID of the server we're connected to. This
// can be empty if we're not connected to any servers.
func (c *Client) ServerLNNode() string {
	c.svrMtx.Lock()
	res := c.svrLnNode
	c.svrMtx.Unlock()
	return res
}

// ServerSession returns the current server session the client is connected to.
// Returns nil if not connected to the server. Note this is set before a
// connection to the server is fully validated as usable.
func (c *Client) ServerSession() clientintf.ServerSessionIntf {
	c.svrMtx.Lock()
	res := c.svrSession
	c.svrMtx.Unlock()
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
// queue. There are two queues involved in the reply: msgs that are waiting
// to be sent and messages that are in the process of being paid/sent/ack.
func (c *Client) RMQLen() (int, int) {
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

// getAddressBookEntry returns an individual address book entry.
func (c *Client) getAddressBookEntry(uid UserID) (*clientdb.AddressBookEntry, error) {
	var res *clientdb.AddressBookEntry
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.GetAddressBookEntry(tx, uid)
		return err
	})
	return res, err
}

// AddressBook returns the full address book of remote users.
func (c *Client) AddressBook() []*clientdb.AddressBookEntry {
	<-c.abLoaded
	ids := c.rul.userList(false)

	// Ignore errors, because it just means some operation may be in
	// progress.
	var res []*clientdb.AddressBookEntry
	_ = c.dbView(func(tx clientdb.ReadTx) error {
		for _, uid := range ids {
			entry, err := c.db.GetAddressBookEntry(tx, uid)
			if err == nil {
				res = append(res, entry)
			}
		}
		return nil
	})
	return res
}

// AllRemoteUsers returns a list of all existing remote users.
func (c *Client) AllRemoteUsers() []*RemoteUser {
	<-c.abLoaded
	return c.rul.allUsers()
}

// AddressBookEntry returns the address book information of a given user.
func (c *Client) AddressBookEntry(uid UserID) (*clientdb.AddressBookEntry, error) {
	return c.getAddressBookEntry(uid)
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

// UserLogNick returns the nick to use when logging a remote user. If the user
// does not exist, returns the UID itself as string. If this uid is for the
// client itself, returns the string "local client".
func (c *Client) UserLogNick(uid UserID) string {
	<-c.abLoaded
	if uid == c.PublicID() {
		return "local client"
	}
	ru, err := c.rul.byID(uid)
	if err != nil {
		return uid.String()
	}
	return strescape.Nick(ru.Nick())
}

// NicksWithPrefix returns a list of nicks for users that have the specified
// prefix.
func (c *Client) NicksWithPrefix(prefix string) []string {
	<-c.abLoaded
	return c.rul.nicksWithPrefix(prefix)
}

// PM sends a private message to the given user, identified by its public id.
// The user must have been already KX'd with for this to work.
func (c *Client) PM(uid UserID, msg string) error {
	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}

	myNick := c.LocalNick()
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.LogPM(tx, uid, false, myNick, msg, time.Now())
	})
	if err != nil {
		return err
	}
	return ru.sendPM(msg)
}

// Handshake starts a 3-way handshake with the specified user. When the local
// client receives a SYNACK, it means the ratchet with the user is fully
// operational.
func (c *Client) Handshake(uid UserID) error {
	ru, err := c.rul.byID(uid)
	if err != nil {
		return nil
	}

	// Store when the last local attempt at a handshake was done.
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		ab, err := c.db.GetAddressBookEntry(tx, uid)
		if err != nil {
			return err
		}
		ab.LastHandshakeAttempt = time.Now()
		return c.db.UpdateAddressBookEntry(tx, ab)
	})
	if err != nil {
		return err
	}

	return c.sendWithSendQ("syn", rpc.RMHandshakeSYN{}, ru.ID())
}

// ChatHistoryEntry contains information parsed from a single line in a chat
// log.
type ChatHistoryEntry struct {
	Message   string `json:"message"`
	From      string `json:"from"`
	Timestamp int64  `json:"timestamp"`
	Internal  bool   `json:"internal"`
}

// ReadHistoryMessages determines which log parsing to use based on whether
// a group chat name was provided in the arguments.  This function will return
// an array of ChatHistoryEntry's that contain information from each line of
// saved logs.
func (c *Client) ReadHistoryMessages(uid UserID, isGC bool, page, pageNum int) ([]ChatHistoryEntry, time.Time, error) {
	var now time.Time
	if c.cfg.NoLoadChatHistory {
		return nil, now, nil
	}

	// Double check the user/GC exists.
	var err error
	var gcName string
	if !isGC {
		_, err = c.rul.byID(uid)
	} else {
		gcName, err = c.GetGCAlias(uid)
	}
	if err != nil {
		return nil, now, err
	}

	var chatHistory []ChatHistoryEntry
	myNick := c.LocalNick()
	err = c.dbView(func(tx clientdb.ReadTx) error {
		var messages []clientdb.PMLogEntry
		if gcName != "" {
			messages, err = c.db.ReadLogGCMsg(tx, gcName, uid, page, pageNum)
			if err != nil {
				return err
			}
		} else {
			messages, err = c.db.ReadLogPM(tx, uid, page, pageNum)
			if err != nil {
				return err
			}
		}
		chatHistory = make([]ChatHistoryEntry, 0, len(messages))
		for _, entry := range messages {
			var filter bool
			if gcName == "" && entry.From != myNick {
				filter, _ = c.shouldFilter(uid, nil, nil, nil, entry.Message)
			} else if gcName != "" && entry.From != myNick {
				userID, err := c.UIDByNick(entry.From)
				if err == nil {
					filter, _ = c.shouldFilter(userID, &uid, nil, nil, entry.Message)
				}
			}
			if !filter {
				chatHistory = append(chatHistory, ChatHistoryEntry{
					Message:   entry.Message,
					From:      entry.From,
					Internal:  entry.Internal,
					Timestamp: entry.Timestamp})
			}
		}
		now = time.Now()
		return nil
	})

	return chatHistory, now, err
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

		// Start automatic handshake with idle users.
		if err := c.handshakeIdleUsers(); err != nil {
			c.log.Errorf("Unable to handshake idle users: %v", err)
		}

		// Remove idle clients users from GCs and posts.
		if err := c.unsubIdleUsers(); err != nil {
			c.log.Errorf("Unable to unsubscribe idle users: %v", err)
		}

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

// Backup
func (c *Client) Backup(_ context.Context, rootDir, destPath string) (string, error) {
	err := os.MkdirAll(destPath, 0o700)
	if err != nil {
		return "", err
	}

	var backupFile string
	err = c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		backupFile, err = c.db.Backup(tx, rootDir, destPath)
		return err
	})
	return backupFile, err
}

// Run runs all client goroutines until the given context is canceled.
//
// Must only be called once.
func (c *Client) Run(ctx context.Context) error {
	// The last thing done is clearing the local client's private data.
	defer c.localID.zero()

	// runctx enables canceling in case of run initialization errors.
	runctx, cancel := context.WithCancel(ctx)

	g, gctx := errgroup.WithContext(runctx)

	// Cancel client context immediately after any of the goroutines fail.
	g.Go(func() error {
		<-gctx.Done()
		return nil
	})

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

	c.log.Infof("Starting client %s", c.localID.id)

	if err := c.initializePlugins(); err != nil {
		cancel()
		return err
	}
	c.log.Info("Initialized plugin")

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
			}

			c.svrMtx.Lock()
			c.svrSession = nextSess
			c.svrLnNode = lnNode
			c.svrMtx.Unlock()

			// Let users check if this server conn is usable.
			if nextSess != nil && c.cfg.CheckServerSession != nil {
				err := c.cfg.CheckServerSession(nextSess.Context(), lnNode)
				if err != nil {
					nextSess.RequestClose(err)
					continue
				}
			}

			var policy clientintf.ServerPolicy

			// Clean old unpaid RVs based on server expirationDays
			// setting if it changed.
			if nextSess != nil {
				policy = nextSess.Policy()
				if lastExpDays != policy.ExpirationDays {
					c.log.Infof("Cleaning up expired RVs "+
						"older than %d days", policy.ExpirationDays)
					c.cleanupPaidRVsDir(policy.ExpirationDays)
					lastExpDays = policy.ExpirationDays
				}

				c.cleanupPushPaymentAttempts(nextSess.Policy().PushPaymentLifetime)
			} else {
				// c.gcmq.SessionChanged(true) is called after
				// the initial batch of subscriptions is done
				// after restart.
				c.gcmq.SessionChanged(false)
			}

			c.rmgr.BindToSession(nextSess)
			c.q.BindToSession(nextSess)
			connected := nextSess != nil
			c.ntfns.notifyServerSessionChanged(connected, policy)
			if canceled(gctx) {
				return nil
			}
			if nextSess != nil && firstConn {
				// Take actions that require having info from
				// the first server connection.
				close(firstConnChan)
				firstConn = false
				kxExpiryLimit := time.Duration(policy.ExpirationDays) * time.Hour * 24
				g.Go(func() error {
					err := c.clearOldMediateIDs(kxExpiryLimit)
					if err != nil && !errors.Is(err, context.Canceled) {
						c.log.Errorf("Unable to clear old mediate IDs: %v", err)
						return err
					}
					err = c.kxl.listenAllKXs(kxExpiryLimit)
					if err != nil && !errors.Is(err, context.Canceled) {
						c.log.Errorf("Unable to listen to all KXs: %v", err)
						return err
					}
					return nil
				})
			}
			if nextSess != nil {
				go c.maybeResetAllKXAfterConn(policy.ExpirationDays)
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

	// Run tip user payments.
	g.Go(func() error { return c.runTipAttempts(gctx) })

	// Restart client onboarding.
	g.Go(func() error { return c.restartOnboarding(gctx) })

	// Reload cached RGCMs.
	g.Go(func() error { return c.loadCachedRGCMs(gctx) })

	// Restart tracking tip receiving.
	g.Go(func() error { return c.restartTrackGeneratedTipInvoices(gctx) })

	return g.Wait()
}
