package clientdb

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
)

// UnusedTipUserTag generates a new, random, unused tag to use when asking
// for tips from a remote user.
func (db *DB) UnusedTipUserTag(tx ReadWriteTx, uid clientintf.UserID) int32 {
	dir := filepath.Join(db.root, inboundDir, uid.String(), tipsDir)
	var id int32
	for {
		id = db.mustRandomInt31()
		fname := filepath.Join(dir, strconv.FormatInt(int64(id), 10))
		if !fileExists(fname) {
			return id
		}
	}
}

// StoreTipUserAttempt stores an attempt by the local client to send a remote
// user a tip.
func (db *DB) StoreTipUserAttempt(tx ReadWriteTx, ta TipUserAttempt) error {
	fname := filepath.Join(db.root, inboundDir, ta.UID.String(), tipsDir,
		strconv.FormatInt(int64(ta.Tag), 10))
	return db.saveJsonFile(fname, ta)
}

// ReadTipAttempt reads an existing tip attempt.
func (db *DB) ReadTipAttempt(tx ReadWriteTx, uid UserID, tag int32) (TipUserAttempt, error) {
	var res TipUserAttempt
	fname := filepath.Join(db.root, inboundDir, uid.String(), tipsDir,
		strconv.FormatInt(int64(tag), 10))
	err := db.readJsonFile(fname, &res)
	return res, err
}

// RemoveTipUserAttempt removes the given tip user attempt.
func (db *DB) RemoveTipUserAttempt(tx ReadWriteTx, uid UserID, tag int32) error {
	fname := filepath.Join(db.root, inboundDir, uid.String(), tipsDir,
		strconv.FormatInt(int64(tag), 10))
	return removeIfExists(fname)
}

// ListTipUserAttempts lists existing attempts to tip remote users.
func (db *DB) ListTipUserAttempts(tx ReadTx, uid UserID) ([]TipUserAttempt, error) {
	dir := filepath.Join(db.root, inboundDir, uid.String(), tipsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	res := make([]TipUserAttempt, 0, len(entries))
	for _, entry := range entries {
		fname := filepath.Join(dir, entry.Name())
		var ta TipUserAttempt
		err := db.readJsonFile(fname, &ta)
		if err != nil {
			db.log.Warnf("Unable to read tip user attempt file %s: %v",
				fname, err)
			continue
		}

		res = append(res, ta)
	}

	return res, nil
}

// ListTipUserAttemptsToRetry lists attempts to tip remote users that should
// be restarted/retried.
func (db *DB) ListTipUserAttemptsToRetry(tx ReadTx, ignoreAfter time.Time, maxLifetime time.Duration) ([]TipUserAttempt, error) {
	pattern := filepath.Join(db.root, inboundDir, "*", tipsDir, "*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	lifetimeLimit := time.Now().Add(-maxLifetime)

	var res []TipUserAttempt
	for _, fname := range files {
		var ta TipUserAttempt
		err := db.readJsonFile(fname, &ta)
		if err != nil {
			db.log.Warnf("Unable to read tip user attempt file %s: %v",
				fname, err)
			continue
		}

		if ta.Completed != nil {
			continue
		}
		if ta.Attempts > ta.MaxAttempts {
			continue
		}
		if ta.InvoiceRequested.After(ignoreAfter) {
			continue
		}
		if ta.Created.Before(lifetimeLimit) {
			continue
		}

		res = append(res, ta)
	}

	return res, nil
}
