package rpcserver

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/resources"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/slog"
)

type ResourcesServerCfg struct {
	Client *client.Client

	// Log should be set to the app's logger.
	Log slog.Logger

	// Router on which to bind the server, to fulfill requests.
	Router *resources.Router
}

type resourcesServer struct {
	c   *client.Client
	log slog.Logger

	mtx            sync.Mutex
	nextID         uint64
	requests       map[uint64]chan interface{}
	upstream       types.ResourcesService_RequestsStreamServer
	upstreamFailed chan error
}

// Fulfill attempts to fulfill a request with an upstream clientrpc client.
func (rs *resourcesServer) Fulfill(ctx context.Context, uid clientintf.UserID,
	req *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	user, err := rs.c.UserByID(uid)
	if err != nil {
		return nil, err
	}

	// Track this request.
	c := make(chan interface{}, 1)
	rs.mtx.Lock()
	id := rs.nextID
	rs.nextID += 1
	upstream := rs.upstream
	if upstream != nil {
		rs.requests[id] = c
	}
	rs.mtx.Unlock()

	// Alert handler.
	if upstream == nil {
		return nil, fmt.Errorf("no upstream to process resource request")
	}

	evnt := &types.ResourceRequestsStreamResponse{
		Id:   id,
		Uid:  uid[:],
		Nick: user.Nick(),
		Request: &types.RMFetchResource{
			Path:  req.Path,
			Meta:  req.Meta,
			Tag:   uint64(req.Tag),
			Index: req.Index,
			Count: req.Count,
		},
	}
	err = upstream.Send(evnt)
	if err != nil {
		rs.mtx.Lock()
		delete(rs.requests, id)
		rs.mtx.Unlock()
		return nil, fmt.Errorf("unable to send request to upstream: %v", err)
	}

	// Wait for reply.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-c:
		switch res := res.(type) {
		case *rpc.RMFetchResourceReply:
			return res, nil
		case error:
			return nil, res
		default:
			return nil, fmt.Errorf("unknown reply typ in req chan %T", res)
		}
	}
}

func (rs *resourcesServer) RequestsStream(ctx context.Context, req *types.ResourceRequestsStreamRequest, res types.ResourcesService_RequestsStreamServer) error {
	rs.mtx.Lock()
	if rs.upstream != nil {
		rs.mtx.Unlock()
		return fmt.Errorf("already have previous upstream handler")
	}
	rs.upstream = res
	rs.mtx.Unlock()

	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-rs.upstreamFailed:
	}

	rs.mtx.Lock()
	rs.upstream = nil
	rs.mtx.Unlock()
	return err
}

func (rs *resourcesServer) FulfillRequest(_ context.Context, res *types.FulfillResourceRequest, _ *types.FulfillResourceRequestResponse) error {

	rs.mtx.Lock()
	c := rs.requests[res.Id]
	delete(rs.requests, res.Id)
	rs.mtx.Unlock()

	if c == nil {
		return fmt.Errorf("request with id %d not found", res.Id)
	}

	if res.ErrorMsg != "" {
		return errors.New(res.ErrorMsg)
	}

	rs.log.Infof("XXXXXXXXXXXXXXX %d", res.Response.Status)

	c <- &rpc.RMFetchResourceReply{
		Tag:    rpc.ResourceTag(res.Response.Tag),
		Status: rpc.ResourceStatus(res.Response.Status),
		Meta:   res.Response.Meta,
		Data:   res.Response.Data,
		Index:  res.Response.Index,
		Count:  res.Response.Count,
	}
	return nil
}

var _ types.ResourcesServiceServer = (*resourcesServer)(nil)

func (s *Server) InitResourcesService(cfg ResourcesServerCfg) error {
	log := slog.Disabled
	if cfg.Log != nil {
		log = cfg.Log
	}
	rs := &resourcesServer{
		c:        cfg.Client,
		log:      log,
		requests: make(map[uint64]chan interface{}),
		nextID:   1,
	}

	if cfg.Router != nil {
		cfg.Router.BindPrefixPath([]string{}, rs)
	}

	s.services.Bind("ResourcesService", types.ResourcesServiceDefn(), rs)
	return nil
}
