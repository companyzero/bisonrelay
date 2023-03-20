package rpcserver

import (
	"context"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
)

type PaymentsServerCfg struct {
	// Client should be set to the [client.Client] instance.
	Client *client.Client

	// Log should be set to the app's logger.
	Log slog.Logger

	// The following handlers are called when a corresponding request is
	// received via the clientrpc interface. They may be used for displaying
	// the request in a user-friendly way in the client UI or to block the
	// request from propagating (by returning a non-nil error).

	OnTipUser func(uid clientintf.UserID, dcrAmount float64) error
}

type paymentsServer struct {
	cfg PaymentsServerCfg
	log slog.Logger
	c   *client.Client
}

func (c *paymentsServer) TipUser(ctx context.Context, req *types.TipUserRequest, _ *types.TipUserResponse) error {
	user, err := c.c.UserByNick(req.User)
	if err != nil {
		return err
	}
	if c.cfg.OnTipUser != nil {
		err := c.cfg.OnTipUser(user.ID(), req.DcrAmount)
		if err != nil {
			return err
		}
	}
	return c.c.TipUser(user.ID(), req.DcrAmount, req.MaxAttempts)
}

var _ types.PaymentsServiceServer = (*paymentsServer)(nil)

// InitPostService initializes and binds a PostsService server to the RPC server.
func (s *Server) InitPaymentsService(cfg PaymentsServerCfg) error {
	ps := &paymentsServer{
		cfg: cfg,
		log: cfg.Log,
		c:   cfg.Client,
	}
	s.services.Bind("PaymentsService", types.PaymentsServiceDefn(), ps)
	return nil
}
