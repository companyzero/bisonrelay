package frfsdb

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/server/serverdb"
)

type fsdb struct {
	rootMsgs                 string
	rootSubs                 string
	rootRedeemedPushPayments string
}

func NewFSDB(rootMsgs, rootSubs string) (serverdb.ServerDB, error) {
	err := os.MkdirAll(rootMsgs, 0700)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(rootSubs, 0700)
	if err != nil {
		return nil, err
	}
	rootRedeemedPushPayments := filepath.Join(rootMsgs, "redeemedPayments")
	if err := os.MkdirAll(rootRedeemedPushPayments, 0700); err != nil {
		return nil, err
	}

	return &fsdb{
		rootMsgs:                 rootMsgs,
		rootSubs:                 rootSubs,
		rootRedeemedPushPayments: rootRedeemedPushPayments,
	}, nil
}

// Static assertion that fsdb implements ServerDB.
var _ serverdb.ServerDB = (*fsdb)(nil)

func (db *fsdb) HealthCheck(ctx context.Context) error {
	return nil
}

func (db *fsdb) IsMaster(ctx context.Context) (bool, error) {
	return true, nil
}

func (db *fsdb) StorePayload(ctx context.Context, rv ratchet.RVPoint, payload []byte, insertTime time.Time) error {
	filename := filepath.Join(db.rootMsgs, rv.String())
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		// File already exists. Return appropriate error.
		return fmt.Errorf("RV %s: %w", rv, serverdb.ErrAlreadyStoredRV)
	}
	return os.WriteFile(filename, payload, 0600)
}

func (db *fsdb) FetchPayload(ctx context.Context, rv ratchet.RVPoint) (*serverdb.FetchPayloadResult, error) {
	filename := filepath.Join(db.rootMsgs, rv.String())

	// Read content
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		// File doesn't exist.
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	stat, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	return &serverdb.FetchPayloadResult{
		Payload:    data,
		InsertTime: stat.ModTime(),
	}, nil
}

func (db *fsdb) RemovePayload(ctx context.Context, rv ratchet.RVPoint) error {
	filename := filepath.Join(db.rootMsgs, rv.String())
	err := os.Remove(filename)
	if os.IsNotExist(err) {
		// File doesn't exist, so not an error.
		return nil
	}
	return err
}

func (db *fsdb) IsSubscriptionPaid(ctx context.Context, rv ratchet.RVPoint) (bool, error) {
	fname := filepath.Join(db.rootSubs, rv.String())
	_, err := os.Stat(fname)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (db *fsdb) StoreSubscriptionPaid(ctx context.Context, rv ratchet.RVPoint, insertTime time.Time) error {
	fname := filepath.Join(db.rootSubs, rv.String())
	return os.WriteFile(fname, nil, 0o600)
}

func (db *fsdb) IsPushPaymentRedeemed(ctx context.Context, payID []byte) (bool, error) {
	fname := filepath.Join(db.rootRedeemedPushPayments, hex.EncodeToString(payID))
	_, err := os.Stat(fname)
	return err == nil, nil
}

func (db *fsdb) StorePushPaymentRedeemed(ctx context.Context, payID []byte, insertTime time.Time) error {
	fname := filepath.Join(db.rootRedeemedPushPayments, hex.EncodeToString(payID))
	content, err := time.Now().MarshalJSON()
	if err != nil {
		return err
	}
	return os.WriteFile(fname, content, 0o600)
}

// Expire the old messages from the specified date.
//
// Note: this is currently significantly slow, as it involves listing all
// entries of the dir.
func (db *fsdb) Expire(ctx context.Context, date time.Time) (uint64, error) {
	date = date.UTC()
	y, m, d := date.Date()
	dirs := []string{db.rootMsgs, db.rootSubs, db.rootRedeemedPushPayments}

	var count uint64
	for i, dirPath := range dirs {
		dir, err := os.Open(dirPath)
		if err != nil {
			return 0, fmt.Errorf("unable to list %s: %v", dirPath, err)
		}

		const nbListEntries = 1024
		for {
			files, err := dir.Readdir(nbListEntries)

			for _, finfo := range files {
				if finfo.IsDir() {
					continue
				}

				fy, fm, fd := finfo.ModTime().Date()
				if y == fy && m == fm && d == fd {
					err := os.Remove(filepath.Join(dirPath, finfo.Name()))
					if err != nil {
						return 0, err
					}
					if i == 0 {
						count += 1
					}
				}
			}

			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return 0, err
			}

		}
	}

	return count, nil
}
