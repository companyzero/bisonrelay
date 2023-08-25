package main

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/companyzero/bisonrelay/client/resources/simplestore"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4/stdscript"
	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrlnd/lnrpc"
)

func handleCompletedSimpleStoreOrder(as *appState, order *simplestore.Order, msg string) {
	if order.User == as.c.PublicID() {
		as.diagMsg("Order #%d placed by the local client", order.ID)
		as.diagMsg("Sample message that would be sent:")
		as.diagMsg(msg)
		return
	}

	ru, err := as.c.UserByID(order.User)
	if err != nil {
		as.diagMsg("Order #%d placed by unknown user %s",
			order.ID, order.User)
		return
	}

	cw := as.findOrNewChatWindow(ru.ID(), ru.Nick())
	as.pm(cw, msg)
}

func handleSimpleStoreOrderStatusChanged(as *appState, order *simplestore.Order, msg string) {
	ru, err := as.c.UserByID(order.User)
	if err != nil {
		as.diagMsg("Order #%d placed by unknown user %s",
			order.ID, order.User)
		return
	}

	cw := as.findOrNewChatWindow(ru.ID(), ru.Nick())
	as.pm(cw, msg)
}

func handleNewTransaction(as *appState, tx *lnrpc.Transaction) error {
	b, err := hex.DecodeString(tx.RawTxHex)
	if err != nil {
		return fmt.Errorf("unable to decode hex tx: %v", err)
	}

	var rawTx wire.MsgTx
	if err := rawTx.Deserialize(bytes.NewBuffer(b)); err != nil {
		return fmt.Errorf("unable to deserialize tx: %v", err)
	}

	chainParams, err := as.lnPC.ChainParams(as.ctx)
	if err != nil {
		return fmt.Errorf("unable to fetch chain params: %v", err)
	}

	for _, out := range rawTx.TxOut {
		_, addrs := stdscript.ExtractAddrs(out.Version, out.PkScript, chainParams)
		if len(addrs) != 1 {
			continue
		}
		addr := addrs[0].String()

		uid := as.c.UserWithOnchainRecvAddr(addr)
		if uid == nil {
			continue
		}

		nick, err := as.c.UserNick(*uid)
		if err != nil {
			return err
		}

		cw := as.findOrNewChatWindow(*uid, nick)
		cw.manyHelpMsgs(func(pf printf) {
			pf("Received %s on address %s via tx %s associated with this user",
				dcrutil.Amount(out.Value), addr, tx.TxHash)
		})
		as.repaintIfActive(cw)
	}
	return nil
}
