package clientdb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RecordUserPayEvent records the given amount as a payment event. If amount is
// < 0, then this means a payment was made related to this user. If amount > 0
// this means a payment was received from this user.
//
// The amount is recorded in Milli-atoms.
func (db *DB) RecordUserPayEvent(tx ReadWriteTx, user UserID, event string, amount, payFee int64) error {
	fname := filepath.Join(db.root, inboundDir, user.String(), payStatsFile)
	evnt := PayStatEvent{
		Timestamp: time.Now().Unix(),
		Event:     event,
		Amount:    amount,
		PayFee:    payFee,
	}
	if err := db.appendToJsonFile(fname, evnt); err != nil {
		return err
	}

	uid := user.String()
	userStats := db.payStats[uid]
	if amount < 0 {
		userStats.TotalSent += -amount
	} else {
		userStats.TotalReceived += amount
	}
	userStats.TotalPayFee += -payFee
	db.payStats[uid] = userStats

	statsFname := filepath.Join(db.root, payStatsFile)
	return db.saveJsonFile(statsFname, &db.payStats)
}

// ListPayStats lists the global (per-user) payment stats.
func (db *DB) ListPayStats(tx ReadTx) (map[UserID]UserPayStats, error) {
	res := make(map[UserID]UserPayStats, len(db.payStats))
	for sid, stats := range db.payStats {
		var id UserID
		if err := id.FromString(sid); err != nil {
			db.log.Warnf("Not a valid user ID while listing pay stats: %s", sid)
			continue
		}

		res[id] = stats
	}
	return res, nil
}

// SummarizeUserPayStats returns a summary of the payments recorded for the
// given user. These are grouped by the first level.
func (db *DB) SummarizeUserPayStats(tx ReadTx, uid UserID) ([]PayStatsSummary, error) {
	fname := filepath.Join(db.root, inboundDir, uid.String(), payStatsFile)
	f, err := os.Open(fname)
	if os.IsNotExist(err) {
		// No stats.
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()

	feeTotal := PayStatsSummary{Prefix: "payfees"}

	dec := json.NewDecoder(f)
	var evnt PayStatEvent
	aux := make(map[string]*PayStatsSummary)
	for err := dec.Decode(&evnt); err == nil; err = dec.Decode(&evnt) {
		prefix := evnt.Event
		if p := strings.Index(prefix, "."); p > -1 {
			prefix = prefix[:p]
		}

		stats, ok := aux[prefix]
		if !ok {
			stats = &PayStatsSummary{Prefix: prefix}
			aux[prefix] = stats
		}
		stats.Total += evnt.Amount
		feeTotal.Total += evnt.PayFee
	}

	res := make([]PayStatsSummary, len(aux)+1)
	i := 0
	for _, s := range aux {
		res[i] = *s
		i += 1
	}

	// Add fees last.
	res[len(res)-1] = feeTotal

	// Sort result so that positive values come first (sorted in descending
	// order) and negative values come next (in ascending order)
	sort.Slice(res, func(i, j int) bool {
		if res[i].Total > 0 && res[i].Total < 0 {
			return true
		}
		if res[i].Total < 0 && res[j].Total > 0 {
			return false
		}
		if res[i].Total < 0 {
			return res[i].Total < res[j].Total
		}
		return res[i].Total > res[j].Total
	})

	return res, nil
}

// ClearPayStats removes pay stats for the given user or for all users if user
// equals nil.
func (db *DB) ClearPayStats(tx ReadWriteTx, user *UserID) error {
	if user == nil {
		// Remove stats summary file.
		statsFname := filepath.Join(db.root, payStatsFile)
		if err := os.Remove(statsFname); err != nil && !os.IsNotExist(err) {
			return err
		}
		db.payStats = make(map[string]UserPayStats)

		// Remove all individual stats files.
		pattern := filepath.Join(db.root, inboundDir, "*", payStatsFile)
		files, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}

		for _, f := range files {
			if err := os.Remove(f); err != nil {
				db.log.Warnf("Unable to remove pay stat file %s: %v", f, err)
			}
		}

		return nil
	}

	// Remove a specific user stats.
	delete(db.payStats, user.String())
	statsFname := filepath.Join(db.root, payStatsFile)
	if err := db.saveJsonFile(statsFname, &db.payStats); err != nil {
		return err
	}

	statsFname = filepath.Join(db.root, inboundDir, user.String(), payStatsFile)
	if err := os.Remove(statsFname); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
