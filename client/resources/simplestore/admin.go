package simplestore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/jsonfile"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/rpc"
)

func (s *Store) handleAdminIndex(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	w := &bytes.Buffer{}
	w.WriteString("# Admin Section\n\n")
	w.WriteString("[Recent Orders](/admin/orders)\n\n")
	w.WriteString("[Back to Index](/)\n\n")
	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil
}

func (s *Store) handleAdminOrders(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	// List all orders for all users.
	pattern := filepath.Join(s.root, ordersDir, "*", "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	tctx := adminOrdersContext{
		Orders: make([]adminOrderSummary, 0, len(files)),
	}

	for _, f := range files {
		var order Order
		if err := jsonfile.Read(f, &order); err != nil {
			s.log.Warnf("Unable to decode order file %s: %v", f, err)
			continue
		}

		nick, _ := s.c.UserNick(order.User)
		nick = strescape.Nick(nick)

		summ := adminOrderSummary{
			ID:       order.ID,
			User:     order.User,
			UserNick: nick,
			Status:   order.Status,
			PlacedTS: order.PlacedTS,
		}
		tctx.Orders = append(tctx.Orders, summ)
	}

	sort.Slice(tctx.Orders, func(i, j int) bool {
		return tctx.Orders[i].PlacedTS.After(tctx.Orders[j].PlacedTS)
	})

	// Generate template.
	w := &bytes.Buffer{}
	err = s.tmpl.ExecuteTemplate(w, adminOrdersTmplFile, &tctx)
	if err != nil {
		return nil, fmt.Errorf("unable to execute product template: %v", err)
	}
	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil
}

func (s *Store) handleAdminViewOrder(ctx context.Context, _ clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if len(request.Path) < 4 {
		return nil, fmt.Errorf("path has < 4 elements")
	}

	// Load order.
	var uid clientintf.UserID
	if err := uid.FromString(request.Path[2]); err != nil {
		return nil, err
	}
	var oid OrderID
	if err := oid.FromString(request.Path[3]); err != nil {
		return nil, err
	}

	orderDir := filepath.Join(s.root, ordersDir, uid.String())
	orderFname := filepath.Join(orderDir, orderFnamePattern.FilenameFor(uint64(oid)))
	var order Order
	if err := jsonfile.Read(orderFname, &order); err != nil {
		return nil, err
	}

	nick, _ := s.c.UserNick(uid)
	nick = strescape.Nick(nick)

	tctx := &adminOrderContext{
		Order:    order,
		UserNick: nick,
	}

	// Generate template.
	w := &bytes.Buffer{}
	err := s.tmpl.ExecuteTemplate(w, adminOrderTmplFile, tctx)
	if err != nil {
		return nil, fmt.Errorf("unable to execute product template: %v", err)
	}
	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil
}

func (s *Store) handleAdminAddOrderComment(ctx context.Context, _ clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	s.mtx.Lock()
	defer s.mtx.Unlock()

	// Process form data.
	var formData string
	if err := json.Unmarshal(request.Data, &formData); err != nil {
		return nil, err
	}
	comment := formData

	// Load Order.
	if len(request.Path) < 4 {
		return nil, fmt.Errorf("path has < 4 elements")
	}

	// Load order.
	var uid clientintf.UserID
	if err := uid.FromString(request.Path[2]); err != nil {
		return nil, err
	}
	var oid OrderID
	if err := oid.FromString(request.Path[3]); err != nil {
		return nil, err
	}
	orderDir := filepath.Join(s.root, ordersDir, uid.String())
	orderFname := filepath.Join(orderDir, orderFnamePattern.FilenameFor(uint64(oid)))
	var order Order
	if err := jsonfile.Read(orderFname, &order); err != nil {
		return nil, err
	}

	// Add comment.
	order.Comments = append(order.Comments, OrderComment{
		Timestamp: time.Now(),
		Comment:   comment,
	})

	// Save order.
	if err := jsonfile.Write(orderFname, &order, s.log); err != nil {
		return nil, err
	}

	// TODO - notify user of new comment?

	// Generate template.
	w := &bytes.Buffer{}
	w.WriteString("# Comment added\n\n")
	w.WriteString(fmt.Sprintf("[Back to Order](/admin/order/%s/%s)\n\n", uid, oid))
	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil

}

func (s *Store) handleAdminUpdateOrderStatus(ctx context.Context, _ clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	s.mtx.Lock()
	defer s.mtx.Unlock()

	// Load Order.
	if len(request.Path) < 5 {
		return nil, fmt.Errorf("path has < 5 elements")
	}

	// Load order.
	var uid clientintf.UserID
	if err := uid.FromString(request.Path[2]); err != nil {
		return nil, err
	}
	var oid OrderID
	if err := oid.FromString(request.Path[3]); err != nil {
		return nil, err
	}
	orderDir := filepath.Join(s.root, ordersDir, uid.String())
	orderFname := filepath.Join(orderDir, orderFnamePattern.FilenameFor(uint64(oid)))
	var order Order
	if err := jsonfile.Read(orderFname, &order); err != nil {
		return nil, err
	}

	// Modify Status.
	order.Status = OrderStatus(request.Path[4])

	// Save order.
	if err := jsonfile.Write(orderFname, &order, s.log); err != nil {
		return nil, err
	}

	if s.cfg.StatusChanged != nil {
		msg := fmt.Sprintf("Your order %s/%s changed to status %s",
			order.User.ShortLogID(), order.ID, order.Status)
		s.cfg.StatusChanged(&order, msg)
	}

	// Generate template.
	w := &bytes.Buffer{}
	w.WriteString("# Order Status Updated\n\n")
	w.WriteString(fmt.Sprintf("[Back to Order](/admin/order/%s/%s)\n\n", uid, oid))
	return &rpc.RMFetchResourceReply{
		Data:   w.Bytes(),
		Status: rpc.ResourceStatusOk,
	}, nil
}
