package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/companyzero/bisonrelay/client/resources/simplestore"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4/stdscript"
	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrlnd/lnrpc"
)

func handleCompletedSimpleStoreOrder(as *appState, order *simplestore.Order) {
	ru, err := as.c.UserByID(order.User)
	if err != nil {
		as.diagMsg("Order #%d placed by unknown user %s",
			order.ID, order.User)
		return
	}

	cw := as.findOrNewChatWindow(ru.ID(), ru.Nick())
	var b strings.Builder
	wpm := func(f string, args ...interface{}) {
		b.WriteString(fmt.Sprintf(f, args...))
	}
	wpm("Thank you for placing your order #%d\n", order.ID)
	wpm("The following were the items in your order:\n")
	var totalUSDCents int64
	for _, item := range order.Cart.Items {
		totalItemUSDCents := int64(item.Quantity) * int64(item.Product.Price*100)
		wpm("  SKU %s - %s - %d units - $%.2f/item - $%.2f\n",
			item.Product.SKU, item.Product.Title,
			item.Quantity, item.Product.Price,
			float64(totalItemUSDCents)/100)
		totalUSDCents += totalItemUSDCents
	}
	wpm("Total amount: $%.2f USD\n", float64(totalUSDCents)/100)

	if (as.ssPayType != ssPayTypeLN) && (as.ssPayType != ssPayTypeOnChain) {
		wpm("\nYou will be contacted with payment details shortly")
		as.pm(cw, b.String())
		return
	}

	rate := as.exchangeRate()
	dcrPriceCents := int64(rate.DCRPrice * 100)
	if rate.DCRPrice <= 0 {
		as.diagMsg("Invalid exchange rate to charge user %s for order %s",
			strescape.Nick(ru.Nick()), order.ID)
		return
	}

	totalDCR, err := dcrutil.NewAmount(float64(totalUSDCents) / float64(dcrPriceCents))
	if err != nil {
		as.diagMsg("Invalid total amount to charge user %s for order %s: %v",
			strescape.Nick(ru.Nick()), order.ID, err)
		return
	}
	wpm("Using the current exchange rate of %.2f USD/DCR, your order is "+
		"%s, valid for the next 60 minutes\n", float64(dcrPriceCents)/100, totalDCR)

	switch as.ssPayType {
	case ssPayTypeLN:
		if as.lnPC == nil {
			as.diagMsg("Unable to generate LN invoice for user %s "+
				"for order %s: LN not setup", strescape.Nick(ru.Nick()),
				order.ID)
			return
		}

		invoice, err := as.lnPC.GetInvoice(as.ctx, int64(totalDCR*1000), nil)
		if err != nil {
			as.diagMsg("Unable to generate LN invoice for user %s "+
				"for order %s: %v", strescape.Nick(ru.Nick()),
				order.ID, err)
			return
		}

		wpm("LN Invoice for payment: %s\n", invoice)
	case ssPayTypeOnChain:
		addr, err := as.c.OnchainRecvAddrForUser(order.User)
		if err != nil {
			as.diagMsg("Unable to generate on-chain addr for user %s: %v",
				strescape.Nick(ru.Nick()), err)
		}
		wpm("On-chain Payment Address: %s\n", addr)

	}
	as.pm(cw, b.String())
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
