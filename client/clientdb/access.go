package clientdb

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/inidb"
	"github.com/companyzero/bisonrelay/lockfile"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
)

type rtx struct {
	ctx context.Context
}

func (tx *rtx) Context() context.Context { return tx.ctx }

type wtx struct {
	ctx context.Context
}

func (tx *wtx) Context() context.Context { return tx.ctx }
func (tx *wtx) Writable() bool           { return tx != nil }

type Config struct {
	// Root is where the db data is stored.
	Root string

	// MsgsRoot is where the logged messages (PMs, GC Msgs, etc) are
	// stored.
	MsgsRoot string

	// LocalID is used to initialize the DB with an ID if one does not yet
	// exist. If nil, a new, random ID is created. Unused if the DB is
	// already initialized.
	LocalIDInitier func() zkidentity.FullIdentity

	Logger slog.Logger

	// ChunkSize is the size to use when chunking files. Values <= 0 means
	// no chunking.
	ChunkSize int

	// DownloadsRoot is where to put final downloaded files.
	DownloadsRoot string
}

type DB struct {
	cfg          Config
	log          slog.Logger
	rnd          io.Reader
	root         string
	downloadsDir string
	idb          *inidb.INIDB
	invites      *inidb.INIDB

	// Keep track of when the last msg of a given conversation was sent.
	// This is used to emit "start-of-conversation", "day-changed" log
	// messages.
	lastMsgTS map[string]time.Time

	sync.Mutex
	running chan struct{}
	runCtx  context.Context

	payStats map[string]UserPayStats

	blockedIDs map[string]time.Time
}

func New(cfg Config) (*DB, error) {
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("unable to determine DB root: %v", err)
	}

	downloadsDir, err := filepath.Abs(cfg.DownloadsRoot)
	if err != nil {
		return nil, fmt.Errorf("unable to determine downloads root: %v", err)
	}

	finfo, err := os.Stat(root)
	switch {
	case errors.Is(err, os.ErrNotExist):
		err := os.MkdirAll(root, 0o700)
		if err != nil {
			return nil, err
		}

	case err == nil:
		if !finfo.IsDir() {
			return nil, fmt.Errorf("root %q is not a dir", root)
		}

	default:
		return nil, err
	}

	if err := os.MkdirAll(filepath.Join(root, inboundDir), 0o700); err != nil {
		return nil, err
	}
	if cfg.MsgsRoot != "" {
		if err := os.MkdirAll(filepath.Join(cfg.MsgsRoot), 0o700); err != nil {
			return nil, err
		}
	}

	// Create the idb db.
	filename := filepath.Join(root, zkcServerDir, zkcServerFile)
	idb, err := inidb.New(filename, true, 10)
	if err != nil && !errors.Is(err, inidb.ErrCreated) {
		return nil, err
	}

	// Create the invites idb.
	filename = filepath.Join(root, invitesDir, "invites.ini")
	invites, err := inidb.New(filename, true, 10)
	if err != nil && !errors.Is(err, inidb.ErrCreated) {
		return nil, err
	}

	log := slog.Disabled
	if cfg.Logger != nil {
		log = cfg.Logger
	}

	blockedIDs := make(map[string]time.Time)
	b, err := os.ReadFile(filepath.Join(root, blockedUsersFile))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil {
		err = json.Unmarshal(b, &blockedIDs)
		if err != nil {
			return nil, err
		}
	}

	db := &DB{
		root:         root,
		downloadsDir: downloadsDir,
		log:          log,
		cfg:          cfg,
		rnd:          rand.Reader,
		running:      make(chan struct{}),
		idb:          idb,
		invites:      invites,
		lastMsgTS:    make(map[string]time.Time),
		blockedIDs:   blockedIDs,
		payStats:     make(map[string]UserPayStats),
	}

	// Perform upgrades as needed.
	if err := db.performUpgrades(); err != nil {
		return nil, err
	}

	// Try to read the pay stats file.
	if err := db.readJsonFile(filepath.Join(root, payStatsFile), &db.payStats); err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("error while loading pay stats file: %v", err)
		}
	}

	return db, nil
}

// Run runs the DB. This should not be called twice for the same db.
func (db *DB) Run(ctx context.Context) error {
	// Attempt to get the lockfile with a small timeout so that we error
	// out immediately instead of waiting until the outer context is
	// canceled.
	lfCtx, cancel := context.WithTimeout(ctx, time.Second)
	lockFilePath := filepath.Join(db.root, lockFileName)
	lockFile, err := lockfile.Create(lfCtx, lockFilePath)
	cancel()
	if err != nil {
		return fmt.Errorf("%w %q: %v", errCreateLockFile, lockFilePath, err)
	}

	db.Lock()
	db.runCtx = ctx
	close(db.running)
	db.Unlock()

	<-ctx.Done()

	if err := lockFile.Close(); err != nil {
		db.log.Errorf("Unable to close lock file: %v", err)
	}
	return ctx.Err()
}

func (db *DB) RunStarted() <-chan struct{} {
	return db.running
}

func (db *DB) View(ctx context.Context, f func(tx ReadTx) error) error {
	<-db.running

	db.Lock()
	ctx, cancel := multiCtx(ctx, db.runCtx)
	tx := &rtx{ctx: ctx}
	err := f(tx)
	cancel()
	db.Unlock()
	return err
}

func (db *DB) Update(ctx context.Context, f func(tx ReadWriteTx) error) error {
	<-db.running

	db.Lock()
	ctx, cancel := multiCtx(ctx, db.runCtx)
	tx := &wtx{ctx: ctx}
	err := f(tx)
	cancel()
	db.Unlock()
	return err
}
