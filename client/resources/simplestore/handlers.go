package simplestore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/jsonfile"
	"github.com/companyzero/bisonrelay/rpc"
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

	if s.cfg.OrderPlaced != nil {
		s.cfg.OrderPlaced(order)
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
