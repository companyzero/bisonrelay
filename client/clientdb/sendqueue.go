package clientdb

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// AddToSendQueue creates a new send queue element to send the given msg to the
// specified destinations.
func (db *DB) AddToSendQueue(tx ReadWriteTx, typ string, dests []UserID,
	msg []byte, priority uint) (SendQID, error) {

	dir := filepath.Join(db.root, sendqDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return SendQID{}, err
	}

	// Get a new random ID.
	id, err := db.randomIDInDir(dir)
	if err != nil {
		return SendQID{}, err
	}

	el := SendQueueElement{
		ID:       id,
		Type:     typ,
		Dests:    dests,
		Msg:      msg,
		Priority: priority,
	}

	fname := filepath.Join(dir, id.String())
	return id, db.saveJsonFile(fname, el)
}

// RemoveFromSendQueue marks the given destination as sent on the specified
// queue.  If the queue is now empty, it is removed from the db.
func (db *DB) RemoveFromSendQueue(tx ReadWriteTx, id SendQID, dest UserID) error {
	var el SendQueueElement
	fname := filepath.Join(db.root, sendqDir, id.String())
	if err := db.readJsonFile(fname, &el); err != nil {
		return err
	}

	for i := range el.Dests {
		if el.Dests[i] != dest {
			continue
		}

		copy(el.Dests[i:], el.Dests[i+1:])
		el.Dests = el.Dests[:len(el.Dests)-1]
		break
	}

	if len(el.Dests) == 0 {
		// All dests sent. Remove file.
		return os.Remove(fname)
	}

	// Save updated file.
	return db.saveJsonFile(fname, el)
}

type sortableSendQ struct {
	q     []SendQueueElement
	times []time.Time
}

func (ssq *sortableSendQ) Len() int           { return len(ssq.q) }
func (ssq *sortableSendQ) Less(i, j int) bool { return ssq.times[i].Sub(ssq.times[j]) < 0 }
func (ssq *sortableSendQ) Swap(i, j int) {
	ssq.q[i], ssq.q[j] = ssq.q[j], ssq.q[i]
	ssq.times[i], ssq.times[j] = ssq.times[j], ssq.times[i]
}

// ListSendQueue lists all send queues registered.
func (db *DB) ListSendQueue(tx ReadTx) ([]SendQueueElement, error) {
	dir := filepath.Join(db.root, sendqDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var res []SendQueueElement
	var times []time.Time
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		var id SendQID
		if err := id.FromString(entry.Name()); err != nil {
			// Skip: file name is not an id.
			continue
		}
		var el SendQueueElement
		fname := filepath.Join(dir, entry.Name())
		if err := db.readJsonFile(fname, &el); err != nil {
			// Skip damaged file.
			db.log.Warnf("Unable to read sendq element file %s: %v",
				fname, err)
			continue
		}

		if len(el.Dests) == 0 {
			// Already sent all of these.
			if err := os.Remove(fname); err != nil {
				db.log.Warnf("Unable to remove already sent "+
					"sendq file: %s: %v", fname, err)
			}
			continue
		}

		finfo, err := entry.Info()
		if err != nil {
			db.log.Warnf("Unable to read modtime from sendq file %s: %v",
				fname, err)
			continue
		}

		res = append(res, el)
		times = append(times, finfo.ModTime())
	}

	// Sort by mod time.
	ssq := &sortableSendQ{q: res, times: times}
	sort.Sort(ssq)

	return ssq.q, nil
}

// ReplaceUserUnackedRM replaces the unacked RM registered for the given user
// with the specified one. It is not an error if no previous unacked rm exists.
func (db *DB) ReplaceUserUnackedRM(tx ReadWriteTx, uid UserID, encrypted []byte,
	rv RawRVID, payEvent string) error {

	rm := UnackedRM{
		UID:       uid,
		Encrypted: encrypted,
		RV:        rv,
		PayEvent:  payEvent,
	}
	fname := filepath.Join(db.root, inboundDir, uid.String(), unackedRMFile)
	return db.saveJsonFile(fname, rm)
}

// RemoveUserUnackedRMWithRV removes the unacked rm of the specified user if it
// exists and if it has the specified RV. It does not return an error if the
// unacked RM did not exist. The return bool indicates whether the unacked rm
// existed.
func (db *DB) RemoveUserUnackedRMWithRV(tx ReadWriteTx, uid UserID, rv RawRVID) (bool, error) {
	fname := filepath.Join(db.root, inboundDir, uid.String(), unackedRMFile)
	var unsent UnackedRM
	if err := db.readJsonFile(fname, &unsent); err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}

	if unsent.RV != rv {
		// The saved RV is not the specified one. Log a debug line, but
		// otherwise this is not an error. This can happen when sending
		// an RM happens faster then processing the one just sent.
		db.log.Debugf("Skip removing unsent RM from user %s since stored "+
			"RV %s is not the specified %s (pay event %s)", uid,
			unsent.RV, rv, unsent.PayEvent)
		return false, nil
	}

	err := os.Remove(fname)
	return err == nil, err
}

// ListUnackedtUserRMs lists unacked RMs from all users.
func (db *DB) ListUnackedUserRMs(tx ReadTx) ([]UnackedRM, error) {
	pattern := filepath.Join(db.root, inboundDir, "*", unackedRMFile)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	res := make([]UnackedRM, len(matches))
	for i := range matches {
		err := db.readJsonFile(matches[i], &res[i])
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}
