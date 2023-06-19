package clientdb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/syndtr/goleveldb/leveldb/errors"
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

// NextTipAttemptToRetryForUser returns the oldest non-completed, non-expired
// tip attempt for a given user.
func (db *DB) NextTipAttemptToRetryForUser(tx ReadTx, uid UserID, maxLifetime time.Duration) (TipUserAttempt, error) {
	var res TipUserAttempt
	pattern := filepath.Join(db.root, inboundDir, uid.String(), tipsDir, "*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return res, err
	}

	if len(files) == 0 {
		return res, errors.ErrNotFound
	}

	lifetimeLimit := time.Now().Add(-maxLifetime)
	gotRes := false
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
		if ta.Attempts == ta.MaxAttempts && ta.LastInvoiceError != nil {
			continue
		}
		if ta.Created.Before(lifetimeLimit) {
			continue
		}
		if res.Created.IsZero() || ta.Created.Before(res.Created) {
			res = ta
			gotRes = true
		}
	}

	if !gotRes {
		return res, ErrNotFound
	}

	return res, nil
}

// ListOldestValidTipUserAttempts returns the oldest tip user attempts (at most
// one per user) which have not expired yet.
func (db *DB) ListOldestValidTipUserAttempts(tx ReadTx, maxLifetime time.Duration) ([]TipUserAttempt, error) {
	dirPattern := filepath.Join(db.root, inboundDir, "*", tipsDir)
	dirs, err := filepath.Glob(dirPattern)
	if err != nil {
		return nil, err
	}

	lifetimeLimit := time.Now().Add(-maxLifetime)

	var res []TipUserAttempt
	for _, dir := range dirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			db.log.Warnf("Unable to list tips dir %s: %v", dir, err)
			continue
		}

		var oldest TipUserAttempt
		gotOldest := false
		for _, file := range files {
			var ta TipUserAttempt
			fname := filepath.Join(dir, file.Name())
			err := db.readJsonFile(fname, &ta)
			if err != nil {
				db.log.Warnf("Unable to read tip user attempt file %s: %v",
					file, err)
				continue
			}

			if ta.Completed != nil {
				continue
			}
			if ta.Attempts > ta.MaxAttempts {
				continue
			}
			if ta.Attempts == ta.MaxAttempts && ta.LastInvoiceError != nil {
				continue
			}
			if ta.Created.Before(lifetimeLimit) {
				continue
			}
			if gotOldest && ta.Created.Before(oldest.Created) {
				continue
			}

			oldest = ta
			gotOldest = true
		}

		if gotOldest {
			res = append(res, oldest)
		}
	}

	return res, nil
}

// StoreGeneratedTipInvoice stores the specified invoice as one generated for
// the remote client to pay the local client for a tip.
func (db *DB) StoreGeneratedTipInvoice(tx ReadWriteTx, uid UserID, invoice string, amountMAtoms int64) error {
	fname := filepath.Join(db.root, inboundDir, uid.String(), genTipInvoicesFile)
	data := GeneratedInvoiceForTip{
		UID:        uid,
		Created:    time.Now(),
		Invoice:    invoice,
		MilliAtoms: uint64(amountMAtoms),
	}
	return db.appendToJsonFile(fname, data)
}

// ListGeneratedTipInvoices lists all invoices generated for tipping from all
// users.
func (db *DB) ListGeneratedTipInvoices(tx ReadTx) ([]GeneratedInvoiceForTip, error) {
	pattern := filepath.Join(db.root, inboundDir, "*", genTipInvoicesFile)
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var res []GeneratedInvoiceForTip
	for _, fname := range files {
		f, err := os.Open(fname)
		if err != nil {
			db.log.Warnf("Unable to open file %s for reading "+
				"generated tip invoices: %v", fname, err)
			continue
		}

		dec := json.NewDecoder(f)
		for {
			var inv GeneratedInvoiceForTip
			err := dec.Decode(&inv)
			if err != nil {
				break
			}
			res = append(res, inv)
		}
		_ = f.Close()
	}

	return res, nil
}

// removeFromGeneratedTipInvoice removes the invoice from the list of invoices
// generated for the remote user to tip the local client.
//
// Returns the data corresponding to the invoice.
func (db *DB) removeFromGeneratedTipInvoice(uid UserID, invoice string) (GeneratedInvoiceForTip, error) {
	var data GeneratedInvoiceForTip

	// Open file.
	genFname := filepath.Join(db.root, inboundDir, uid.String(), genTipInvoicesFile)
	f, err := os.Open(genFname)
	if os.IsNotExist(err) {
		return data, ErrNotFound
	}
	if err != nil {
		return data, err
	}

	// Read list of existing generated invoices, adding to res[] all but
	// the target one.
	var res []GeneratedInvoiceForTip
	found := false
	dec := json.NewDecoder(f)
	for {
		var inv GeneratedInvoiceForTip
		err := dec.Decode(&inv)
		if err != nil {
			break
		}
		if inv.Invoice == invoice {
			found = true
			data = inv
		} else {
			res = append(res, inv)
		}
	}
	if err := f.Close(); err != nil {
		return data, err
	}

	if !found {
		return data, ErrNotFound
	}

	// Either remove the file (if no other invoices remain) or rewrite the
	// file with the remaining invoices.
	if len(res) == 0 {
		err := removeIfExists(genFname)
		if err != nil {
			return data, err
		}
	} else {
		f, err := os.Create(genFname)
		if err != nil {
			return data, err
		}
		enc := json.NewEncoder(f)
		for _, inv := range res {
			if err := enc.Encode(inv); err != nil {
				return data, err
			}
		}
		if err := f.Close(); err != nil {
			return data, err
		}
	}

	return data, nil
}

// MarkGeneratedTipInvoiceExpired marks an invoice generated for tipping as
// having expired.
func (db *DB) MarkGeneratedTipInvoiceExpired(tx ReadWriteTx, uid UserID, invoice string) error {
	data, err := db.removeFromGeneratedTipInvoice(uid, invoice)
	if err != nil {
		return err
	}
	expiredFname := filepath.Join(db.root, inboundDir, uid.String(), expiredTipInvoicesFile)
	return db.appendToJsonFile(expiredFname, data)
}

// MarkGeneratedTipInvoiceReceived marks an invoice generated for tipping as
// having been received (i.e. invoice was paid).
func (db *DB) MarkGeneratedTipInvoiceReceived(tx ReadWriteTx, uid UserID, invoice string, receivedMAtoms int64) error {
	data, err := db.removeFromGeneratedTipInvoice(uid, invoice)
	if err != nil {
		return err
	}
	recvFname := filepath.Join(db.root, inboundDir, uid.String(), recvTipInvoicesFile)
	return db.appendToJsonFile(recvFname, data)
}
