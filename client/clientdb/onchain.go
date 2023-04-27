package clientdb

import (
	"errors"
	"os"
	"path/filepath"
)

type onchainAddr struct {
	Addr string `json:"addr"`
}

// OnchainRecvAddrForUser returns the onchain address for an user or an empty
// string if a valid address does not exist.
func (db *DB) OnchainRecvAddrForUser(tx ReadTx, uid UserID) (string, error) {
	filename := filepath.Join(db.root, inboundDir, uid.String(), recvAddrForUserFile)
	var jsonAddr onchainAddr
	err := db.readJsonFile(filename, &jsonAddr)
	if errors.Is(err, ErrNotFound) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	return jsonAddr.Addr, nil
}

// UpdateOnchainRecvAddrForUser updates the on-chain address of the local node
// for receiving payments for the specified user. If addr is an empty string,
// then this removes the existing address.
func (db *DB) UpdateOnchainRecvAddrForUser(tx ReadWriteTx, uid UserID, addr string) error {
	filename := filepath.Join(db.root, inboundDir, uid.String(), recvAddrForUserFile)
	if addr == "" {
		err := os.Remove(filename)
		if err == nil || os.IsNotExist(err) {
			return nil
		}
	}
	jsonAddr := onchainAddr{Addr: addr}
	return db.saveJsonFile(filename, jsonAddr)
}

// UserWithOnchainRecvAddr returns the user id associated with the given
// receive address or nil if no such id exists.
func (db *DB) UserWithOnchainRecvAddr(tx ReadTx, addr string) *UserID {
	fi, err := os.ReadDir(filepath.Join(db.root, inboundDir))
	if err != nil {
		return nil
	}

	for _, v := range fi {
		// Read ID.
		var uid UserID
		if err := uid.FromString(v.Name()); err != nil {
			db.log.Warnf("Unable to identify user id %s: %v",
				v.Name(), err)
			continue
		}

		var jsonAddr onchainAddr
		filename := filepath.Join(db.root, inboundDir, uid.String(), recvAddrForUserFile)
		if err := db.readJsonFile(filename, &jsonAddr); err != nil && !errors.Is(err, ErrNotFound) {
			db.log.Warnf("Unable to load onchain addr file %s: %v",
				filename, err)
			continue
		}

		if jsonAddr.Addr == addr {
			return &uid
		}
	}

	return nil
}
