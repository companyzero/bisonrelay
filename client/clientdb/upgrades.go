package clientdb

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/companyzero/bisonrelay/inidb"
	"github.com/companyzero/bisonrelay/ratchet/disk"
	"github.com/companyzero/bisonrelay/zkidentity"
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

// upgrade02 fills the FirstCreated entry of all address book entries.
func (db *DB) upgrade02() error {
	const inboundDir = "inbound"
	const identityFilename = "publicidentity.json"
	const ratchetFilename = "ratchet.json"

	pattern := filepath.Join(db.root, inboundDir, "*", identityFilename)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	now := time.Now()
	for _, fname := range matches {
		var ab AddressBookEntry
		if err := db.readJsonFile(fname, &ab); err != nil {
			db.log.Warnf("Unable to load identity file %s during upgrade02: %v",
				fname, err)
			continue
		}

		if !ab.FirstCreated.IsZero() {
			continue
		}

		// If the ratchet has a last encrypt or decrypt time encoded, use it.
		var rs disk.RatchetState
		ratchetFname := filepath.Join(db.root, inboundDir, ab.ID.Identity.String(), ratchetFilename)
		if err := db.readJsonFile(ratchetFname, &rs); err != nil {
			db.log.Warnf("Unable to load ratchet file %s: %v", ratchetFname, err)
			continue
		}

		if rs.LastDecryptTime > 0 {
			ab.FirstCreated = time.UnixMilli(rs.LastDecryptTime)
		} else if rs.LastEncryptTime > 0 {
			ab.FirstCreated = time.UnixMilli(rs.LastEncryptTime)
		} else {
			ab.FirstCreated = now
		}

		if err := db.saveJsonFile(fname, &ab); err != nil {
			db.log.Warnf("Unable to update FirstCreated of identity file %s: %v",
				fname, err)
		}
	}

	return nil
}

// upgrade03 moves the known server certs from the old idb structure to the new
// knownservers.json file.
func (db *DB) upgrade03() error {
	innerBase64, err := db.idb.Get("", "serveridentity")
	if errors.Is(err, inidb.ErrNotFound) {
		// No inner cert to migrate.
		return nil
	}
	if err != nil {
		return err
	}
	innerJSON, err := base64.StdEncoding.DecodeString(innerBase64)
	if err != nil {
		return fmt.Errorf("could not decode serveridentity from base64: %v", err)
	}

	var innerPub zkidentity.PublicIdentity
	err = json.Unmarshal(innerJSON, &innerPub)
	if err != nil {
		return fmt.Errorf("could not unmarshal serveridentity: %v", err)
	}

	outerBase64, err := db.idb.Get("", "servercert")
	if errors.Is(err, inidb.ErrNotFound) {
		// No outer cert to migrate.
		return nil
	}
	if err != nil {
		return err
	}
	outerTls, err := base64.StdEncoding.DecodeString(outerBase64)
	if err != nil {
		return fmt.Errorf("could not convert outer cert to bytes: %v", err)
	}

	// Save cert pair to new store location.
	filename := filepath.Join(db.root, serverCertsFile)
	certs := []ServerCertPair{{OuterTLS: outerTls, InnerPub: innerPub}}
	if err := db.saveJsonFile(filename, certs); err != nil {
		return err
	}

	// Remove from old store.
	if err := db.idb.Del("", "serveridentity"); err != nil {
		return err
	}
	if err := db.idb.Del("", "servercert"); err != nil {
		return err
	}
	if err = db.idb.Save(); err != nil {
		return fmt.Errorf("could not save idb: %v", err)
	}

	return nil
}

func (db *DB) performUpgrades() error {
	if err := db.upgrade01(); err != nil {
		return err
	}
	if err := db.upgrade02(); err != nil {
		return err
	}
	if err := db.upgrade03(); err != nil {
		return err
	}

	return nil
}
