package client

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"sort"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/internal/lowlevel"
)

// canceled returns true if the given context is done.
func canceled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func zeroSlice(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func (c *Client) mustRandomUint64() uint64 {
	var b [8]byte
	if n, err := rand.Read(b[:]); n < 8 || err != nil {
		panic("out of entropy")
	}
	return binary.LittleEndian.Uint64(b[:])
}

// rvManagerDBAdapter adapts the client to the interface required by the
// RVManagerDB.
type rvManagerDBAdapter struct {
	c *Client
}

func (rvdb *rvManagerDBAdapter) UnpaidRVs(rvs []lowlevel.RVID, expirationDays int) ([]lowlevel.RVID, error) {
	var unpaid []lowlevel.RVID
	err := rvdb.c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		for _, rv := range rvs {
			if paid, err := rvdb.c.db.IsRVPaid(tx, rv, expirationDays); err != nil {
				return err
			} else if !paid {
				unpaid = append(unpaid, rv)
			}
		}
		return nil
	})
	return unpaid, err
}

func (rvdb *rvManagerDBAdapter) SavePaidRVs(rvs []lowlevel.RVID) error {
	err := rvdb.c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		for _, rv := range rvs {
			if err := rvdb.c.db.SaveRVPaid(tx, rv); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (rvdb *rvManagerDBAdapter) MarkRVUnpaid(rv lowlevel.RVID) error {
	err := rvdb.c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return rvdb.c.db.MarkRVUnpaid(tx, rv)
	})
	return err
}

// rmqDBAdapter is an adapter structure that satisfies the RMQDB interface using
// a client's db as backing storage.
type rmqDBAdapter struct {
	c *Client
}

func (rmqdb *rmqDBAdapter) RVHasPaymentAttempt(rv lowlevel.RVID) (string, time.Time, error) {
	var invoice string
	var ts time.Time
	err := rmqdb.c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		invoice, ts, err = rmqdb.c.db.HasPushPaymentAttempt(tx, rv)
		return err
	})
	return invoice, ts, err
}

func (rmqdb *rmqDBAdapter) StoreRVPaymentAttempt(rv lowlevel.RVID, invoice string, ts time.Time) error {
	return rmqdb.c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return rmqdb.c.db.StorePushPaymentAttempt(tx, rv, invoice, ts)
	})
}

func (rmqdb *rmqDBAdapter) DeleteRVPaymentAttempt(rv lowlevel.RVID) error {
	return rmqdb.c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return rmqdb.c.db.DeletePushPaymentAttempt(tx, rv)
	})
}

// SortedUserPayStatsIDs returns a sorted list of IDs from the passed stats
// map, ordered by largest total payments.
func SortedUserPayStatsIDs(stats map[UserID]clientdb.UserPayStats) []UserID {
	res := make([]UserID, len(stats))
	i := 0
	for uid := range stats {
		copy(res[i][:], uid[:])
		i += 1
	}

	sort.Slice(res, func(i, j int) bool {
		si := stats[res[i]]
		sj := stats[res[j]]
		ti := si.TotalSent + si.TotalReceived
		tj := sj.TotalSent + sj.TotalReceived
		return ti > tj
	})

	return res
}

type sliceChanges[T comparable] struct {
	added   []T
	removed []T
}

// sliceDiff returns added and removed items in newSlice compared to oldSlice.
func sliceDiff[T comparable](oldSlice, newSlice []T) sliceChanges[T] {
	var added, removed []T

	// Aux map of items in newAux that already existed in oldSlice.
	newAux := make(map[T]bool, len(newSlice))
	for _, newV := range newSlice {
		newAux[newV] = true
	}

	// Determine items that were removed or already existed in old.
	for _, oldV := range oldSlice {
		if _, ok := newAux[oldV]; !ok {
			// Deleted from new.
			removed = append(removed, oldV)
		} else {
			// Already existed in old.
			newAux[oldV] = false
		}
	}

	// Anything not set in the map is a new item.
	for newV, isNew := range newAux {
		if isNew {
			added = append(added, newV)
		}
	}
	return sliceChanges[T]{added: added, removed: removed}
}
