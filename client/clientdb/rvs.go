package clientdb

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/companyzero/bisonrelay/ratchet"
)

// isDateLte returns true if the date (y-m-d) in a is less than or equal to
// the date in time b.
func isDateLte(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay < by ||
		(ay == by && am < bm) ||
		(ay == by && am == bm && ad <= bd)
}

type paidRV struct {
	TS time.Time
}

// CleanupPaidRVs cleans up the paid RVs dir.
func (db *DB) CleanupPaidRVs(tx ReadWriteTx, expirationDays int) error {
	// Cleanup the paid RVs dir.
	paidRVsDir := filepath.Join(db.root, paidRVsDir)
	files, err := os.ReadDir(paidRVsDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	paidRVExpirationDuration := 24 * time.Hour * time.Duration(expirationDays)
	validLimit := time.Now().Add(-paidRVExpirationDuration)
	total := 0
	for _, f := range files {
		var prv paidRV
		filename := filepath.Join(paidRVsDir, f.Name())
		err := db.readJsonFile(filename, &prv)
		if err != nil {
			db.log.Debugf("Unable to read json file %s: %v", filename, err)
			continue
		}
		if isDateLte(prv.TS, validLimit) {
			err = os.Remove(filename)
			if err != nil {
				db.log.Debugf("Unable to remove file %s: %v",
					filename, err)
				continue
			}
			total += 1
		}
	}
	if total > 0 {
		db.log.Debugf("Cleaned up %d paid RV entries", total)
	}

	return nil
}

// IsRVPaid returns true if the specified RV has been paid for.
func (db *DB) IsRVPaid(tx ReadWriteTx, rv ratchet.RVPoint, expirationDays int) (bool, error) {
	filename := filepath.Join(db.root, paidRVsDir, rv.String())
	var prv paidRV
	err := db.readJsonFile(filename, &prv)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	paidRVExpirationDuration := 24 * time.Hour * time.Duration(expirationDays)

	// Determine whether it's still valid.
	validLimit := time.Now().UTC().Add(-paidRVExpirationDuration)
	if isDateLte(prv.TS, validLimit) {
		// Not valid anymore.
		if err := os.Remove(filename); err != nil {
			return false, err
		}
		return false, nil
	}

	// Still valid.
	return true, nil
}

// SaveRVPaid marks the given RV as paid.
func (db *DB) SaveRVPaid(tx ReadWriteTx, rv ratchet.RVPoint) error {
	prv := paidRV{TS: time.Now()}
	filename := filepath.Join(db.root, paidRVsDir, rv.String())
	return db.saveJsonFile(filename, &prv)
}

// MarkRVUnpaid forcefully marks the given RV as unpaid.
func (db *DB) MarkRVUnpaid(tx ReadWriteTx, rv ratchet.RVPoint) error {
	filename := filepath.Join(db.root, paidRVsDir, rv.String())
	err := os.Remove(filename)
	if os.IsNotExist(err) {
		// Ignore unknown RVs.
		return nil
	}
	return err
}
