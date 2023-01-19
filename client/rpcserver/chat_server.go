package rpcserver

import (
	"context"
	"fmt"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/slog"
)

// ChatServerCfg is the configuration for a new [types.ChatServiceServer]
// deployment.
type ChatServerCfg struct {
	// Client should be set to the [client.Client] instance.
	Client *client.Client

	// Log should be set to the app's logger.
	Log slog.Logger

	// The following handlers are called when a corresponding request is
	// received via the clientrpc interface. They may be used for displaying
	// the request in a user-friendly way in the client UI or to block the
	// request from propagating (by returning a non-nil error).

	OnPM func(ctx context.Context, uid client.UserID, req *types.PMRequest) error
}

type chatServer struct {
	c   *client.Client
	log slog.Logger
	cfg ChatServerCfg
}

func (c *chatServer) PM(ctx context.Context, req *types.PMRequest, res *types.PMResponse) error {
	if req.Msg == nil {
		return fmt.Errorf("msg is nil")
	}
	if req.Msg.Message == "" {
		return fmt.Errorf("msg is empty")
	}
	user, err := c.c.UserByNick(req.User)
	if err != nil {
		return err
	}
	if c.cfg.OnPM != nil {
		err = c.cfg.OnPM(ctx, user.ID(), req)
		if err != nil {
			return err
		}
	}
	return c.c.PM(user.ID(), req.Msg.Message)
}

func (c *chatServer) PMStream(ctx context.Context, req *types.PMStreamRequest, stream types.ChatService_PMStreamServer) error {
	ctx, cancel := context.WithCancel(ctx)
	ntfnHandler := func(ru *client.RemoteUser, p rpc.RMPrivateMessage, ts time.Time) {
		ntfn := &types.ReceivedPM{
			Uid:         ru.ID().Bytes(),
			Nick:        ru.Nick(),
			TimestampMs: ts.UnixMilli(),
			Msg: &types.RMPrivateMessage{
				Message: p.Message,
				Mode:    types.MessageMode(p.Mode),
			},
		}

		err := stream.Send(ntfn)
		if err != nil {
			cancel()
		}
	}
	reg := c.c.NotificationManager().Register(client.OnPMNtfn(ntfnHandler))
	<-ctx.Done()
	reg.Unregister()
	return ctx.Err()
}

var _ types.ChatServiceServer = (*chatServer)(nil)

// InitChatService initializes and binds a ChatService server to the RPC server.
func (s *Server) InitChatService(cfg ChatServerCfg) {
	cs := &chatServer{
		cfg: cfg,
		log: cfg.Log,
		c:   cfg.Client,
	}
	s.services.Bind("ChatService", types.ChatServiceDefn(), cs)
}
