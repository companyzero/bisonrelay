package client

import (
	"fmt"

	"github.com/companyzero/bisonrelay/client/clientdb"
)

// OnchainRecvAddrForUser returns the on-chain receive address of the local
// wallet associated with the specified user. If acct is specified, addresses
// are generated from that account.
func (c *Client) OnchainRecvAddrForUser(uid UserID, acct string) (string, error) {
	var addr string
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		addr, err = c.db.OnchainRecvAddrForUser(tx, uid)
		return err
	})
	if err != nil {
		return "", err
	}
	if addr != "" {
		return addr, nil
	}

	if pc, ok := c.cfg.PayClient.(*DcrlnPaymentClient); ok {
		newAddr, err := pc.NewReceiveAddress(c.ctx, acct)
		if err != nil {
			return "", fmt.Errorf("unable to generate new on-chain address: %v", err)
		}

		err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
			return c.db.UpdateOnchainRecvAddrForUser(tx, uid, newAddr.String())
		})
		if err != nil {
			return "", err
		}
		return newAddr.String(), nil
	} else {
		return "", fmt.Errorf("payment client is not able to generate " +
			"new receive addresses")
	}
}

// UpdateOnchainRecvAddrForUser updates the on-chain receive address of the local
// wallet associated with the specified user. If addr is nil, then the current
// address is removed.
func (c *Client) UpdateOnchainRecvAddrForUser(uid UserID, addr string) error {
	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.UpdateOnchainRecvAddrForUser(tx, uid, addr)
	})
}

// UserWithOnchainRecvAddr returns the user that has the specified onchain
// receive address. If there is no user for the specified address, then this
// returns nil.
func (c *Client) UserWithOnchainRecvAddr(addr string) *UserID {
	var uid *UserID
	err := c.dbView(func(tx clientdb.ReadTx) error {
		uid = c.db.UserWithOnchainRecvAddr(tx, addr)
		return nil
	})
	if err != nil {
		c.log.Warn("Unable to find user with onchain addr %s: %v", addr,
			err)
	}
	return uid
}
