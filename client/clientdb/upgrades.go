package clientdb

import (
	"fmt"
	"path/filepath"
)

// upgrade01 moves any inbound/[user]/unacked.rm file to
// inbound/[user]/unackedrms/[rv] file.
func (db *DB) upgrade01() error {
	const oldFilename = "unackedrm.json"
	const newDir = "unackedrms"
	const inboundDir = "inbound"

	pattern := filepath.Join(db.root, inboundDir, "*", oldFilename)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, oldFname := range matches {
		var rm UnackedRM
		err := db.readJsonFile(oldFname, &rm)
		if err != nil {
			return fmt.Errorf("unable to read unacked RM file %s: %v",
				oldFname, err)
		}

		newFname := filepath.Join(db.root, inboundDir, rm.UID.String(),
			newDir, rm.RV.String())
		err = db.saveJsonFile(newFname, rm)
		if err != nil {
			return fmt.Errorf("unable to save new unacked RM file %s: %v",
				newFname, err)
		}

		_ = removeIfExists(oldFname)
		db.log.Infof("Moved unacked RM from %s to %s during upgrade",
			oldFname, newFname)
	}

	return nil
}

func (db *DB) performUpgrades() error {
	if err := db.upgrade01(); err != nil {
		return err
	}

	return nil
}
