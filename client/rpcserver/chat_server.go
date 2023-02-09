package rpcserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/internal/replaymsglog"
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

	// RootReplayMsgLogs is the root dir where replaymsglogs are stored for
	// supported message types.
	RootReplayMsgLogs string

	// The following handlers are called when a corresponding request is
	// received via the clientrpc interface. They may be used for displaying
	// the request in a user-friendly way in the client UI or to block the
	// request from propagating (by returning a non-nil error).

	OnPM  func(ctx context.Context, uid client.UserID, req *types.PMRequest) error
	OnGCM func(ctx context.Context, gcid client.GCID, req *types.GCMRequest) error
}

type chatServer struct {
	c   *client.Client
	log slog.Logger
	cfg ChatServerCfg

	replayMtx    sync.Mutex
	pmStreams    *serverStreams[types.ChatService_PMStreamServer]
	gcmStreams   *serverStreams[types.ChatService_GCMStreamServer]
	pmReplayLog  *replaymsglog.Log
	gcmReplayLog *replaymsglog.Log
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
	id := replaymsglog.ID(req.UnackedFrom)
	c.replayMtx.Lock()

	// Send old messages before registering for the new ones.
	ntfn := new(types.ReceivedPM)
	err := c.pmReplayLog.ReadAfter(id, ntfn, func(id replaymsglog.ID) error {
		ntfn.SequenceId = uint64(id)
		err := stream.Send(ntfn)
		if err != nil {
			return err
		}
		ntfn.Reset()
		return nil
	})

	var streamID int32
	if err == nil {
		streamID, ctx = c.pmStreams.register(ctx, stream)
	}
	c.replayMtx.Unlock()
	if err != nil {
		return err
	}

	<-ctx.Done()
	c.pmStreams.unregister(streamID)
	return ctx.Err()
}

// pmNtfnHandler is called by the client when a PM arrived from a remote user.
func (c *chatServer) pmNtfnHandler(ru *client.RemoteUser, p rpc.RMPrivateMessage, ts time.Time) {
	ntfn := &types.ReceivedPM{
		Uid:         ru.ID().Bytes(),
		Nick:        ru.Nick(),
		TimestampMs: ts.UnixMilli(),
		Msg: &types.RMPrivateMessage{
			Message: p.Message,
			Mode:    types.MessageMode(p.Mode),
		},
	}

	// Save in replay file
	c.replayMtx.Lock()
	replayID, err := c.pmReplayLog.Store(ntfn)
	c.replayMtx.Unlock()
	if err != nil {
		c.log.Errorf("Unable to store PM in replay log: %v", err)
		return
	}
	ntfn.SequenceId = uint64(replayID)

	c.pmStreams.iterateOver(func(id int32, stream types.ChatService_PMStreamServer) {
		err := stream.Send(ntfn)
		if err != nil {
			c.log.Debugf("Unregistering PM stream %d due to err: %v",
				id, err)
			c.pmStreams.unregister(id)
		}
	})
}

// AckReceivedPM acks to the server that PMs up to a sequence ID have been
// processed.
func (c *chatServer) AckReceivedPM(ctx context.Context, req *types.AckRequest,
	res *types.AckResponse) error {
	id := replaymsglog.ID(req.SequenceId)
	err := c.pmReplayLog.ClearUpTo(id)
	if err != nil {
		c.log.Errorf("Unable to clear PM log up to id %s: %v", id, err)
	}
	return nil
}

// GCM sends a message in a GC.
func (c *chatServer) GCM(ctx context.Context, req *types.GCMRequest, res *types.GCMResponse) error {
	gcid, err := c.c.GCIDByName(req.Gc)
	if err != nil {
		return err
	}
	if c.cfg.OnGCM != nil {
		err = c.cfg.OnGCM(ctx, gcid, req)
		if err != nil {
			return err
		}
	}
	return c.c.GCMessage(gcid, req.Msg, rpc.MessageModeNormal, nil)
}

// GCMStream returns a stream that gets GC messages received by the client.
func (c *chatServer) GCMStream(ctx context.Context, req *types.GCMStreamRequest, stream types.ChatService_GCMStreamServer) error {
	id := replaymsglog.ID(req.UnackedFrom)
	c.replayMtx.Lock()

	// Send old messages before registering for the new ones.
	ntfn := new(types.GCReceivedMsg)
	err := c.gcmReplayLog.ReadAfter(id, ntfn, func(id replaymsglog.ID) error {
		ntfn.SequenceId = uint64(id)
		err := stream.Send(ntfn)
		if err != nil {
			return err
		}
		ntfn.Reset()
		return nil
	})

	var streamID int32
	if err == nil {
		streamID, ctx = c.gcmStreams.register(ctx, stream)
	}
	c.replayMtx.Unlock()
	if err != nil {
		return err
	}

	<-ctx.Done()
	c.gcmStreams.unregister(streamID)
	return ctx.Err()
}

// gcmNtfnHandler is called by the client when a GC message arrives from a
// remote user.
func (c *chatServer) gcmNtfnHandler(ru *client.RemoteUser, gcm rpc.RMGroupMessage, ts time.Time) {
	gcalias, err := c.c.GetGCAlias(gcm.ID)
	if err != nil {
		c.log.Debugf("Skipping received GCM without group %s", gcm.ID)
	}
	ntfn := &types.GCReceivedMsg{
		Uid:         ru.ID().Bytes(),
		Nick:        ru.Nick(),
		TimestampMs: ts.UnixMilli(),
		GcAlias:     gcalias,
		Msg: &types.RMGroupMessage{
			Id:      gcm.ID[:],
			Message: gcm.Message,
			Mode:    types.MessageMode(gcm.Mode),
		},
	}

	// Save in replay file
	c.replayMtx.Lock()
	replayID, err := c.gcmReplayLog.Store(ntfn)
	c.replayMtx.Unlock()
	if err != nil {
		c.log.Errorf("Unable to store PM in replay log: %v", err)
		return
	}
	ntfn.SequenceId = uint64(replayID)

	c.gcmStreams.iterateOver(func(id int32, stream types.ChatService_GCMStreamServer) {
		err := stream.Send(ntfn)
		if err != nil {
			c.log.Debugf("Unregistering PM stream %d due to err: %v",
				id, err)
			c.pmStreams.unregister(id)
		}
	})
}

// AckReceivedGCM acks to the server that GCMs up to a sequence ID have been
// processed.
func (c *chatServer) AckReceivedGCM(ctx context.Context, req *types.AckRequest,
	res *types.AckResponse) error {
	id := replaymsglog.ID(req.SequenceId)
	err := c.gcmReplayLog.ClearUpTo(id)
	if err != nil {
		c.log.Errorf("Unable to clear GCM log up to id %s: %v", id, err)
	}
	return nil
}

// registerOfflineMessageStorageHandlers registers the handlers for streams on
// the client's notification manager.
func (c *chatServer) registerOfflineMessageStorageHandlers() {
	nmgr := c.c.NotificationManager()
	nmgr.RegisterSync(client.OnPMNtfn(c.pmNtfnHandler))
	nmgr.RegisterSync(client.OnGCMNtfn(c.gcmNtfnHandler))
}

var _ types.ChatServiceServer = (*chatServer)(nil)

// InitChatService initializes and binds a ChatService server to the RPC server.
func (s *Server) InitChatService(cfg ChatServerCfg) error {

	pmReplayLog, err := replaymsglog.New(replaymsglog.Config{
		Log:     cfg.Log,
		Prefix:  "pm",
		RootDir: cfg.RootReplayMsgLogs,
		MaxSize: 1 << 23, // 8MiB
	})
	if err != nil {
		return err
	}

	gcmReplayLog, err := replaymsglog.New(replaymsglog.Config{
		Log:     cfg.Log,
		Prefix:  "gcm",
		RootDir: cfg.RootReplayMsgLogs,
		MaxSize: 1 << 23, // 8MiB
	})
	if err != nil {
		return err
	}

	cs := &chatServer{
		cfg: cfg,
		log: cfg.Log,
		c:   cfg.Client,

		pmStreams:    &serverStreams[types.ChatService_PMStreamServer]{},
		gcmStreams:   &serverStreams[types.ChatService_GCMStreamServer]{},
		pmReplayLog:  pmReplayLog,
		gcmReplayLog: gcmReplayLog,
	}
	cs.registerOfflineMessageStorageHandlers()
	s.services.Bind("ChatService", types.ChatServiceDefn(), cs)
	return nil
}
