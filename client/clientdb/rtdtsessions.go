package clientdb

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/companyzero/bisonrelay/zkidentity"
)

const (
	rtdtSessionsDir = "rtdtsessions"
)

// UpdateRTDTSession updates the data for the given RTDT session.
func (db *DB) UpdateRTDTSession(tx ReadWriteTx, sess *RTDTSession) error {
	fname := filepath.Join(db.root, rtdtSessionsDir, sess.Metadata.RV.String())
	return db.saveJsonFile(fname, sess)
}

// GetRTDTSession returns data for the given RTDT session.
func (db *DB) GetRTDTSession(tx ReadTx, sessRV *zkidentity.ShortID) (*RTDTSession, error) {
	fname := filepath.Join(db.root, rtdtSessionsDir, sessRV.String())
	res := new(RTDTSession)
	err := db.readJsonFile(fname, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// GetRTDTSessionByPrefix returns data for the RTDT session with the given
// prefix for RV.
func (db *DB) GetRTDTSessionByPrefix(tx ReadTx, prefix string) (*RTDTSession, error) {
	pattern := filepath.Join(db.root, rtdtSessionsDir, prefix+"*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, ErrNotFound
	}
	if len(matches) > 1 {
		return nil, errors.New("multiple entries with the same prefix")
	}

	res := new(RTDTSession)
	err = db.readJsonFile(matches[0], res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ListRTDTSessions lists stored RTDT sessions.
func (db *DB) ListRTDTSessions(tx ReadTx) []zkidentity.ShortID {
	entries, err := os.ReadDir(filepath.Join(db.root, rtdtSessionsDir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		db.log.Warnf("Unable to read RTDT sessions dir: %v", err)
		return nil
	}

	res := make([]zkidentity.ShortID, 0, len(entries))
	for _, entry := range entries {
		var id zkidentity.ShortID
		if err := id.FromString(entry.Name()); err != nil {
			db.log.Debugf("%q is not a valid id: %v", entry.Name(), err)
			continue
		}

		res = append(res, id)
	}

	return res
}

// RemoveRTDTSession removes the given session from the db.
func (db *DB) RemoveRTDTSession(tx ReadWriteTx, sessRV *zkidentity.ShortID) error {
	fname := filepath.Join(db.root, rtdtSessionsDir, sessRV.String())
	err := os.Remove(fname)
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}
