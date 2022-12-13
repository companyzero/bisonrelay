package client

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"sort"

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

func mustRandomUint32() uint32 {
	var b [4]byte
	if n, err := rand.Read(b[:]); n < 4 || err != nil {
		panic("out of entropy")
	}
	return binary.LittleEndian.Uint32(b[:])
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
