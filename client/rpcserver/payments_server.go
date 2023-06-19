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

	// RootReplayMsgLogs is the root dir where replaymsglogs are stored for
	// supported message types.
	RootReplayMsgLogs string

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

	tipProgressStreams *serverStreams[*types.TipProgressEvent]
}

func (p *paymentsServer) TipUser(ctx context.Context, req *types.TipUserRequest, _ *types.TipUserResponse) error {
	user, err := p.c.UserByNick(req.User)
	if err != nil {
		return err
	}
	if p.cfg.OnTipUser != nil {
		err := p.cfg.OnTipUser(user.ID(), req.DcrAmount)
		if err != nil {
			return err
		}
	}
	return p.c.TipUser(user.ID(), req.DcrAmount, req.MaxAttempts)
}

func (p *paymentsServer) TipProgress(ctx context.Context, req *types.TipProgressRequest, stream types.PaymentsService_TipProgressServer) error {
	return p.tipProgressStreams.runStream(ctx, req.UnackedFrom, stream)
}

func (p *paymentsServer) tipProgressNtfnHandler(ru *client.RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
	var attemptErrMsg string
	if attemptErr != nil {
		attemptErrMsg = attemptErr.Error()
	}
	ntfn := &types.TipProgressEvent{
		Uid:          ru.ID().Bytes(),
		Nick:         ru.Nick(),
		AmountMatoms: amtMAtoms,
		Completed:    completed,
		Attempt:      int32(attempt),
		AttemptErr:   attemptErrMsg,
		WillRetry:    willRetry,
	}
	p.tipProgressStreams.send(ntfn)
}

func (p *paymentsServer) AckTipProgress(_ context.Context, req *types.AckRequest, _ *types.AckResponse) error {
	return p.tipProgressStreams.ack(req.SequenceId)
}

func (p *paymentsServer) registerOfflineMessageStorageHandlers() {
	nmgr := p.c.NotificationManager()
	nmgr.RegisterSync(client.OnTipAttemptProgressNtfn(p.tipProgressNtfnHandler))
}

var _ types.PaymentsServiceServer = (*paymentsServer)(nil)

// InitPostService initializes and binds a PostsService server to the RPC server.
func (s *Server) InitPaymentsService(cfg PaymentsServerCfg) error {
	tipProgressStreams, err := newServerStreams[*types.TipProgressEvent](cfg.RootReplayMsgLogs, "tipprogress", cfg.Log)
	if err != nil {
		return err
	}

	ps := &paymentsServer{
		cfg: cfg,
		log: cfg.Log,
		c:   cfg.Client,

		tipProgressStreams: tipProgressStreams,
	}
	ps.registerOfflineMessageStorageHandlers()
	s.services.Bind("PaymentsService", types.PaymentsServiceDefn(), ps)
	return nil
}
