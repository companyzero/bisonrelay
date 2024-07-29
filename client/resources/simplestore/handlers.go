package simplestore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/jsonfile"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/rpc"
)

var orderFnamePattern = jsonfile.MakeDecimalFilePattern("order-", ".json", false)

func (s *Store) handleNotFound(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	return &rpc.RMFetchResourceReply{
		Status: rpc.ResourceStatusNotFound,
	}, nil
}

func (s *Store) handleIndex(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	s.mtx.Lock()
	tmplCtx := &indexContext{
		Products: s.products,
		IsAdmin:  uid == s.c.PublicID(),
	}
	w := &bytes.Buffer{}
	err := s.tmpl.ExecuteTemplate(w, indexTmplFile, tmplCtx)
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

	if request.Data == nil {
		return &rpc.RMFetchResourceReply{
			Status: rpc.ResourceStatusBadRequest,
			Data:   []byte("request data is empty"),
		}, nil
	}

	formData := struct {
		SKU string `json:"sku"`
		Qty uint32 `json:"qty"`
	}{}

	if err := json.Unmarshal(request.Data, &formData); err != nil {
		return &rpc.RMFetchResourceReply{
			Status: rpc.ResourceStatusBadRequest,
			Data:   []byte("request data not valid json"),
		}, nil
	}
	fname := filepath.Join(s.root, cartsDir, uid.String())
	var cart Cart

	s.mtx.Lock()
	defer s.mtx.Unlock()

	prod, ok := s.products[formData.SKU]
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
			item.Quantity += formData.Qty
			hasItem = true
			break
		}
	}

	if !hasItem {
		newItem := &CartItem{
			Product:  prod,
			Quantity: formData.Qty,
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

func (s *Store) handleClearCart(ctx context.Context, uid clientintf.UserID) (*rpc.RMFetchResourceReply, error) {
	fname := filepath.Join(s.root, cartsDir, uid.String())

	err := os.Remove(fname)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	var cart Cart
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

	var exchangeRate float64
	if s.cfg.ExchangeRateProvider != nil {
		exchangeRate = s.cfg.ExchangeRateProvider()
	}
	if exchangeRate <= 0 {
		s.log.Warnf("order rejected due to invalid exchange rate")
		return &rpc.RMFetchResourceReply{
			Status: rpc.ResourceStatusBadRequest,
			Data:   []byte("store has an invalid exchange rate"),
		}, nil
	}

	cartFname := filepath.Join(s.root, cartsDir, uid.String())
	var cart Cart

	s.mtx.Lock()
	defer s.mtx.Unlock()

	err := jsonfile.Read(cartFname, &cart)
	if err != nil && !errors.Is(err, jsonfile.ErrNotFound) {
		return nil, err
	}

	if len(cart.Items) == 0 {
		return &rpc.RMFetchResourceReply{
			Data:   []byte("No items in order.\n\n[Back to Index](/index.md)"),
			Status: rpc.ResourceStatusOk,
		}, nil
	}

	var shipAddr *ShippingAddress
	// Verify the items
	for _, item := range cart.Items {
		prod, ok := s.products[item.Product.SKU]
		if !ok {
			return &rpc.RMFetchResourceReply{
				Status: rpc.ResourceStatusBadRequest,
				Data:   []byte(fmt.Sprintf("SKU %q does not exist", item.Product.SKU)),
			}, nil
		}
		// If a product requires shipping, ensure a shipping address
		// was sent.
		if shipAddr == nil && prod.Shipping {
			// Process form data.
			var formData ShippingAddress
			if err := json.Unmarshal(request.Data, &formData); err != nil {
				return &rpc.RMFetchResourceReply{
					Status: rpc.ResourceStatusBadRequest,
					Data:   []byte("request data not valid json"),
				}, nil
			}

			if formData.Name == "" || formData.Address1 == "" ||
				formData.City == "" || formData.State == "" ||
				formData.PostalCode == "" {
				return &rpc.RMFetchResourceReply{
					Status: rpc.ResourceStatusBadRequest,
					Data:   []byte("incomplete shipping address"),
				}, nil
			}

			// TODO: proper address validation, optional phone
			// number validation.
			shipAddr = &formData
		}
	}

	// Create the order.
	orderDir := filepath.Join(s.root, ordersDir, uid.String())
	lastID, err := orderFnamePattern.Last(orderDir)
	if err != nil {
		return nil, err
	}

	id := lastID.ID + 1
	order := &Order{
		User:         uid,
		Cart:         cart,
		ID:           OrderID(id),
		Status:       StatusPlaced,
		PlacedTS:     time.Now(),
		ShipCharge:   s.cfg.ShipCharge,
		ShipAddr:     shipAddr,
		ExpiresTS:    time.Now().Add(time.Hour),
		ExchangeRate: exchangeRate,
	}

	// Build the message to send to the remote user, and present it to the
	// UI.
	var b strings.Builder
	wpm := func(f string, args ...interface{}) {
		b.WriteString(fmt.Sprintf(f, args...))
	}

	var userNick string
	ru, err := s.c.UserByID(order.User)
	if err != nil && order.User == s.c.PublicID() {
		userNick = "(local client)"
	} else if err != nil {
		return nil, fmt.Errorf("Order #%d placed by unknown user %s",
			order.ID, order.User)
	} else {
		userNick = strescape.Nick(ru.Nick())
	}

	wpm("Thank you for placing your order #%d\n", order.ID)
	if order.ShipAddr != nil {
		shipAddr := order.ShipAddr
		wpm("Shipping address:\n")
		wpm("   Name: %s\n", shipAddr.Name)
		wpm("   Addr: %s\n", shipAddr.Address1)
		if shipAddr.Address2 != "" {
			wpm("   Addr: %s\n", shipAddr.Address2)
		}
		wpm("   City: %s\n", shipAddr.City)
		wpm("  State: %s\n", shipAddr.State)
		wpm("    Zip: %s\n", shipAddr.PostalCode)
		wpm("  Phone: %s\n", shipAddr.Phone)
	}
	wpm("The following were the items in your order:\n")
	for _, item := range order.Cart.Items {
		totalItemUSDCents := int64(item.Quantity) * int64(item.Product.Price*100)
		wpm("  SKU %s - %s - %d units - $%.2f/item - $%.2f\n",
			item.Product.SKU, item.Product.Title,
			item.Quantity, item.Product.Price,
			float64(totalItemUSDCents)/100)
	}

	if order.Cart.HasCharges() && s.cfg.ShipCharge > 0 {
		wpm("Total item amount: $%.2f USD\n", order.Cart.Total())
		wpm("Shipping and handling charge: $%.2f USD\n", s.cfg.ShipCharge)
		wpm("Total amount: $%.2f USD\n", order.Total())
	} else {
		wpm("Total amount: $%.2f USD\n", order.Total())
	}

	totalDCR := order.TotalDCR()
	if totalDCR > 0 {
		wpm("Using the current exchange rate of %.2f USD/DCR, your order is "+
			"%s, valid for the next hour (expires %s)\n",
			order.ExchangeRate, totalDCR, order.ExpiresTS.Format("Mon, 02 Jan 2006 15:04 MST"))
	}

	pt := s.cfg.PayType
	switch {
	case s.cfg.ExchangeRateProvider == nil:
		s.log.Warnf("No exchange rate provider setup in simplestore config")
	case order.ExchangeRate <= 0:
		s.log.Warnf("Invalid exchange rate to charge user %s for order %s",
			userNick, order.ID)
	case totalDCR == 0:
		s.log.Warnf("Order has zero total dcr amount")
	case pt == PayTypeOnChain:
		addr, err := s.c.OnchainRecvAddrForUser(order.User, s.cfg.Account)
		if err != nil {
			s.log.Errorf("Unable to generate on-chain addr for user %s: %v",
				userNick, err)
		} else {
			wpm("On-chain Payment Address: %s\n", addr)
			order.PayType = PayTypeOnChain
			order.Invoice = addr
		}

	case pt == PayTypeLN:
		if s.lnpc == nil {
			s.log.Warnf("Unable to generate LN invoice for user %s "+
				"for order %s: LN not setup", userNick,
				order.ID)
		} else {
			invoice, err := s.lnpc.GetInvoice(ctx, int64(totalDCR*1000), nil)
			if err != nil {
				s.log.Errorf("Unable to generate LN invoice for user %s "+
					"order %s: %v", userNick,
					order.ID, err)

				// Fallback to generating an onchain payment address.
				addr, err := s.c.OnchainRecvAddrForUser(order.User, s.cfg.Account)
				if err != nil {
					s.log.Errorf("Unable to generate on-chain addr for user %s: %v",
						userNick, err)
				} else {
					wpm("On-chain Payment Address: %s\n", addr)
					order.PayType = PayTypeOnChain
					order.Invoice = addr
				}
			} else {
				urlInvoice := "lnpay://" + invoice
				wpm("LN Invoice for payment: %s\n", urlInvoice)
				order.PayType = PayTypeLN
				order.Invoice = invoice
			}
		}

	default:
		wpm("\nYou will be contacted with payment details shortly")
	}

	// Track pending invoice or onchain addr for payment.
	if order.Invoice != "" {
		pendingFname := filepath.Join(s.root, pendingInvoicesDir, fmt.Sprintf("%s-%s", uid, order.ID))
		if err := jsonfile.Write(pendingFname, "", s.log); err != nil {
			return nil, fmt.Errorf("unable to write pending invoice file: %v", err)
		}
		select {
		case s.invoiceCreatedChan <- order:
		case <-s.runCtx.Done():
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if s.cfg.OrderPlaced != nil {
		s.cfg.OrderPlaced(order, b.String())
	}

	// Save order.
	orderFname := filepath.Join(orderDir, orderFnamePattern.FilenameFor(id))
	err = jsonfile.Write(orderFname, order, s.log)
	if err != nil {
		return nil, err
	}

	// Clear cart.
	if err := jsonfile.RemoveIfExists(cartFname); err != nil {
		return nil, err
	}

	// Render result.
	w := &bytes.Buffer{}
	err = s.tmpl.ExecuteTemplate(w, orderPlacedTmplFile, &order)
	if err != nil {
		return nil, fmt.Errorf("unable to execute product template: %v", err)
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

func (s *Store) handleOrderStatus(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	s.mtx.Lock()
	defer s.mtx.Unlock()

	id, err := strconv.ParseUint(request.Path[1], 10, 64)
	if err != nil {
		return &rpc.RMFetchResourceReply{
			Data:   []byte("invalid order id"),
			Status: rpc.ResourceStatusBadRequest,
		}, nil
	}

	fname := filepath.Join(s.root, ordersDir, uid.String(), orderFnamePattern.FilenameFor(id))

	var order Order
	err = jsonfile.Read(fname, &order)
	if err != nil {
		if errors.Is(err, jsonfile.ErrNotFound) {
			return &rpc.RMFetchResourceReply{
				Data:   []byte("order not found"),
				Status: rpc.ResourceStatusBadRequest,
			}, nil
		}
		return nil, fmt.Errorf("Unable to read order %s: %v",
			fname, err)
	}

	tmplCtx := &orderContext{
		Order: order,
	}

	w := &bytes.Buffer{}
	err = s.tmpl.ExecuteTemplate(w, orderTmplFile, tmplCtx)
	if err != nil {
		return nil, fmt.Errorf("unable to execute order template: %v", err)
	}

	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil
}

func (s *Store) handleOrderAddComment(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	s.mtx.Lock()
	defer s.mtx.Unlock()

	// Process form data.
	var formData string
	if err := json.Unmarshal(request.Data, &formData); err != nil {
		return nil, err
	}
	comment := formData

	id, err := strconv.ParseUint(request.Path[1], 10, 64)
	if err != nil {
		return &rpc.RMFetchResourceReply{
			Data:   []byte("invalid order id"),
			Status: rpc.ResourceStatusBadRequest,
		}, nil
	}

	fname := filepath.Join(s.root, ordersDir, uid.String(), orderFnamePattern.FilenameFor(id))

	var order Order
	err = jsonfile.Read(fname, &order)
	if err != nil {
		if errors.Is(err, jsonfile.ErrNotFound) {
			return &rpc.RMFetchResourceReply{
				Data:   []byte("order not found"),
				Status: rpc.ResourceStatusBadRequest,
			}, nil
		}
		return nil, fmt.Errorf("Unable to read order %s: %v",
			fname, err)
	}

	// Add comment.
	order.Comments = append(order.Comments, OrderComment{
		Timestamp: time.Now(),
		FromAdmin: false,
		Comment:   comment,
	})

	// Save order.
	if err := jsonfile.Write(fname, &order, s.log); err != nil {
		return nil, err
	}

	// Generate template.
	w := &bytes.Buffer{}
	w.WriteString("# Comment added\n\n")
	w.WriteString(fmt.Sprintf("[Back to Order](/order/%d)\n\n", id))
	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil

}

func (s *Store) handleStaticRequest(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	page := request.Path[1] + ".tmpl"

	w := &bytes.Buffer{}
	err := s.tmpl.ExecuteTemplate(w, page, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to execute static template: %v", err)
	}

	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil
}
