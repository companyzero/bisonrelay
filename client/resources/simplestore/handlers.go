package simplestore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/jsonfile"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/dcrd/dcrutil/v4"
)

func (s *Store) handleNotFound(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	return &rpc.RMFetchResourceReply{
		Status: rpc.ResourceStatusNotFound,
	}, nil
}

func (s *Store) handleIndex(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	s.mtx.Lock()
	w := &bytes.Buffer{}
	err := s.tmpl.ExecuteTemplate(w, indexTmplFile, &indexContext{Products: s.products})
	s.mtx.Unlock()
	if err != nil {
		return nil, fmt.Errorf("unable to execute index template: %v", err)
	}

	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil
}

func (s *Store) handleProduct(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	s.mtx.Lock()
	prod := s.products[request.Path[1]]
	s.mtx.Unlock()

	if prod == nil {
		return s.handleNotFound(ctx, uid, request)
	}

	w := &bytes.Buffer{}
	err := s.tmpl.ExecuteTemplate(w, prodTmplFile, prod)
	if err != nil {
		return nil, fmt.Errorf("unable to execute product template: %v", err)
	}

	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil
}

func (s *Store) handleAddToCart(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	sku := request.Path[1]
	fname := filepath.Join(s.root, cartsDir, uid.String())
	var cart Cart

	s.mtx.Lock()
	defer s.mtx.Unlock()

	prod, ok := s.products[sku]
	if !ok {
		return nil, fmt.Errorf("product does not exist")
	}

	err := jsonfile.Read(fname, &cart)
	if err != nil && !errors.Is(err, jsonfile.ErrNotFound) {
		return nil, err
	}

	hasItem := false
	for _, item := range cart.Items {
		if item.Product.SKU == prod.SKU {
			item.Quantity += 1
			hasItem = true
			break
		}
	}

	if !hasItem {
		newItem := &CartItem{
			Product:  prod,
			Quantity: 1,
		}
		cart.Items = append(cart.Items, newItem)
	}
	cart.Updated = time.Now()

	err = jsonfile.Write(fname, &cart, s.log)
	if err != nil {
		return nil, err
	}

	tmplCtx := addToCartContext{
		Product: prod,
		Cart:    &cart,
	}
	w := &bytes.Buffer{}
	err = s.tmpl.ExecuteTemplate(w, addToCartTmplFile, tmplCtx)
	if err != nil {
		return nil, fmt.Errorf("unable to execute product template: %v", err)
	}

	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil
}

func (s *Store) handleCart(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	fname := filepath.Join(s.root, cartsDir, uid.String())
	var cart Cart

	s.mtx.Lock()
	err := jsonfile.Read(fname, &cart)
	s.mtx.Unlock()

	if err != nil && !errors.Is(err, jsonfile.ErrNotFound) {
		return nil, err
	}

	w := &bytes.Buffer{}
	err = s.tmpl.ExecuteTemplate(w, cartTmplFile, &cart)
	if err != nil {
		return nil, fmt.Errorf("unable to execute product template: %v", err)
	}

	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil
}

func (s *Store) handlePlaceOrder(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	cartFname := filepath.Join(s.root, cartsDir, uid.String())
	var cart Cart

	s.mtx.Lock()
	defer s.mtx.Unlock()

	err := jsonfile.Read(cartFname, &cart)
	if err != nil && !errors.Is(err, jsonfile.ErrNotFound) {
		return nil, err
	}

	// Create the order.
	order := &Order{
		User: uid,
		Cart: cart,
	}

	if len(cart.Items) > 0 {
		var filename string
		for order.ID = 1; order.ID < math.MaxUint32; order.ID++ {
			filename = filepath.Join(s.root, ordersDir, uid.String(),
				order.ID.String())
			if !jsonfile.Exists(filename) {
				break
			}
		}

		err = jsonfile.Write(filename, order, s.log)
		if err != nil {
			return nil, err
		}

		if err := jsonfile.RemoveIfExists(cartFname); err != nil {
			return nil, err
		}
	}

	w := &bytes.Buffer{}
	err = s.tmpl.ExecuteTemplate(w, orderPlacedTmplFile, &order)
	if err != nil {
		return nil, fmt.Errorf("unable to execute product template: %v", err)
	}

	// Build the message to send to the remote user, and present it to the
	// UI.
	var b strings.Builder
	wpm := func(f string, args ...interface{}) {
		b.WriteString(fmt.Sprintf(f, args...))
	}

	ru, err := s.c.UserByID(order.User)
	if err != nil {
		return nil, fmt.Errorf("Order #%d placed by unknown user %s",
			order.ID, order.User)
	} else {
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

		if totalUSDCents > 0 && s.cfg.ShipCharge > 0 {
			wpm("Total item amount: $%.2f USD\n", float64(totalUSDCents)/100)
			wpm("Shipping and handling charge: $%.2f USD\n", s.cfg.ShipCharge)
			totalUSDCents += int64(s.cfg.ShipCharge * 100)
			wpm("Total amount: $%.2f USD\n", float64(totalUSDCents)/100)
		} else {
			wpm("Total amount: $%.2f USD\n", float64(totalUSDCents)/100)
		}

		var dcrPriceCents int64
		var totalDCR dcrutil.Amount
		if s.cfg.ExchangeRateProvider != nil {
			dcrPrice := s.cfg.ExchangeRateProvider()
			dcrPriceCents = int64(dcrPrice * 100)
			if dcrPriceCents > 0 {
				totalDCR, _ = dcrutil.NewAmount(float64(totalUSDCents) / float64(dcrPriceCents))
			}
		}

		if totalDCR > 0 {
			wpm("Using the current exchange rate of %.2f USD/DCR, your order is "+
				"%s, valid for the next 60 minutes\n", float64(dcrPriceCents)/100, totalDCR)
		}

		pt := s.cfg.PayType
		switch {
		case s.cfg.ExchangeRateProvider == nil:
			s.log.Warnf("No exchange rate provider setup in simplestore config")
		case dcrPriceCents <= 0:
			s.log.Warnf("Invalid exchange rate to charge user %s for order %s",
				strescape.Nick(ru.Nick()), order.ID)
		case totalDCR == 0:
			s.log.Warnf("Order has zero total dcr amount")
		case pt == PayTypeOnChain:
			addr, err := s.c.OnchainRecvAddrForUser(order.User, s.cfg.Account)
			if err != nil {
				s.log.Errorf("Unable to generate on-chain addr for user %s: %v",
					strescape.Nick(ru.Nick()), err)
			} else {
				wpm("On-chain Payment Address: %s\n", addr)
			}
		case pt == PayTypeLN:
			if s.lnpc == nil {
				s.log.Warnf("Unable to generate LN invoice for user %s "+
					"for order %s: LN not setup", strescape.Nick(ru.Nick()),
					order.ID)
			} else {
				invoice, err := s.lnpc.GetInvoice(ctx, int64(totalDCR*1000), nil)
				if err != nil {
					s.log.Warnf("Unable to generate LN invoice for user %s "+
						"for order %s: %v", strescape.Nick(ru.Nick()),
						order.ID, err)
				} else {
					wpm("LN Invoice for payment: %s\n", invoice)
				}
			}

		default:
			wpm("\nYou will be contacted with payment details shortly")
		}
	}

	if s.cfg.OrderPlaced != nil {
		s.cfg.OrderPlaced(order, b.String())
	}

	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil
}

func (s *Store) handleOrders(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	s.mtx.Lock()
	defer s.mtx.Unlock()

	dir := filepath.Join(s.root, ordersDir, uid.String())
	files, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	var orders []*Order
	for _, file := range files {
		order := &Order{}
		fname := filepath.Join(dir, file.Name())
		err := jsonfile.Read(fname, order)
		if err != nil {
			s.log.Warnf("Unable to read order %s: %v",
				fname, err)
			continue
		}
		orders = append(orders, order)
	}

	tmplCtx := &ordersContext{
		Orders: orders,
	}

	w := &bytes.Buffer{}
	err = s.tmpl.ExecuteTemplate(w, ordersTmplFile, tmplCtx)
	if err != nil {
		return nil, fmt.Errorf("unable to execute product template: %v", err)
	}

	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil

}
