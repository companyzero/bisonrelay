package rpcserver

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/companyzero/bisonrelay/clientrpc/jsonrpc"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
)

// Config is the available configuration for a [Server].
type Config struct {
	JSONRPCListeners []net.Listener
	Log              slog.Logger
	RPCUser          string
	RPCPass          string
	AuthMode         string
}

// Server is an RPC server for a corresponding BR Client instance.
type Server struct {
	runOnce    sync.Once
	services   *types.ServersMap
	jsonServer *jsonrpc.Server
}

func (s *Server) Run(ctx context.Context) error {
	var notRunning bool
	s.runOnce.Do(func() { notRunning = true })
	if !notRunning {
		return fmt.Errorf("Run() is already running")
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return s.jsonServer.Run(gctx) })

	return g.Wait()
}

func New(cfg Config) *Server {
	services := new(types.ServersMap)
	jsonServer := jsonrpc.NewServer(
		jsonrpc.WithServices(services),
		jsonrpc.WithListeners(cfg.JSONRPCListeners),
		jsonrpc.WithServerLog(cfg.Log),
		jsonrpc.WithAuth(cfg.RPCUser, cfg.RPCPass, cfg.AuthMode),
	)
	s := &Server{
		services:   services,
		jsonServer: jsonServer,
	}
	return s
}
