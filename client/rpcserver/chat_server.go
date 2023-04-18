package rpcserver

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientintf"
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

	pmStreams  *serverStreams[*types.ReceivedPM]
	gcmStreams *serverStreams[*types.GCReceivedMsg]
	kxStreams  *serverStreams[*types.KXCompleted]
}

func (c *chatServer) SendFile(_ context.Context, req *types.SendFileRequest, _ *types.SendFileResponse) error {
	user, err := c.c.UserByNick(req.User)
	if err != nil {
		return err
	}

	return c.c.SendFile(user.ID(), req.Filename)
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
	return c.pmStreams.runStream(ctx, req.UnackedFrom, stream)
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

	c.pmStreams.send(ntfn)
}

// AckReceivedPM acks to the server that PMs up to a sequence ID have been
// processed.
func (c *chatServer) AckReceivedPM(ctx context.Context, req *types.AckRequest,
	res *types.AckResponse) error {
	return c.pmStreams.ack(req.SequenceId)
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
	return c.gcmStreams.runStream(ctx, req.UnackedFrom, stream)
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

	c.gcmStreams.send(ntfn)
}

// AckReceivedGCM acks to the server that GCMs up to a sequence ID have been
// processed.
func (c *chatServer) AckReceivedGCM(ctx context.Context, req *types.AckRequest,
	res *types.AckResponse) error {
	return c.gcmStreams.ack(req.SequenceId)
}

func (c *chatServer) MediateKX(ctx context.Context, req *types.MediateKXRequest, res *types.MediateKXResponse) error {
	mediator, err := c.c.UserByNick(req.Mediator)
	if err != nil {
		return err
	}

	var target clientintf.UserID
	if err := target.FromString(req.Target); err != nil {
		return err
	}

	return c.c.RequestMediateIdentity(mediator.ID(), target)
}

func (c *chatServer) KXStream(ctx context.Context, req *types.KXStreamRequest, stream types.ChatService_KXStreamServer) error {
	return c.kxStreams.runStream(ctx, req.UnackedFrom, stream)
}

// kxNtfnHandler is called by the client when the client completes a KX with a user.
func (c *chatServer) kxNtfnHandler(ir *clientintf.RawRVID, ru *client.RemoteUser) {
	ntfn := &types.KXCompleted{
		Uid:  ru.ID().Bytes(),
		Nick: ru.Nick(),
	}
	if ir != nil {
		ntfn.InitialRendezvous = ir.Bytes()
	}
	c.kxStreams.send(ntfn)
}

func (c *chatServer) AckKXCompleted(_ context.Context, req *types.AckRequest, _ *types.AckResponse) error {
	return c.kxStreams.ack(req.SequenceId)
}

func marshalOOBPublicIDInvite(invite *rpc.OOBPublicIdentityInvite, res *types.OOBPublicIdentityInvite) *types.OOBPublicIdentityInvite {
	if res == nil {
		res = &types.OOBPublicIdentityInvite{}
	}
	*res = types.OOBPublicIdentityInvite{
		InitialRendezvous: invite.InitialRendezvous[:],
		ResetRendezvous:   invite.ResetRendezvous[:],
	}
	if res.Public == nil {
		res.Public = &types.PublicIdentity{}
	}
	*res.Public = types.PublicIdentity{
		Name:      invite.Public.Name,
		Nick:      invite.Public.Nick,
		SigKey:    invite.Public.SigKey[:],
		Key:       invite.Public.Key[:],
		Identity:  invite.Public.Identity[:],
		Digest:    invite.Public.Digest[:],
		Signature: invite.Public.Signature[:],
	}
	return res
}

func (c *chatServer) WriteNewInvite(_ context.Context, req *types.WriteNewInviteRequest, res *types.WriteNewInviteResponse) error {
	b := bytes.NewBuffer(nil)
	invite, err := c.c.WriteNewInvite(b)
	if err != nil {
		return err
	}
	*res = types.WriteNewInviteResponse{
		InviteBytes: b.Bytes(),
		Invite:      marshalOOBPublicIDInvite(&invite, res.Invite),
	}
	if req.Gc != "" {
		gcid, err := c.c.GCIDByName(req.Gc)
		if err != nil {
			return err
		}
		if err = c.c.AddInviteOnKX(invite.InitialRendezvous, gcid); err != nil {
			return err
		}
	}

	return nil
}

func (c *chatServer) AcceptInvite(_ context.Context, req *types.AcceptInviteRequest, res *types.AcceptInviteResponse) error {
	b := bytes.NewBuffer(req.InviteBytes)
	invite, err := c.c.ReadInvite(b)
	if err != nil {
		return err
	}
	err = c.c.AcceptInvite(invite)
	if err != nil {
		return err
	}
	res.Invite = marshalOOBPublicIDInvite(&invite, res.Invite)
	return nil
}

// registerOfflineMessageStorageHandlers registers the handlers for streams on
// the client's notification manager.
func (c *chatServer) registerOfflineMessageStorageHandlers() {
	nmgr := c.c.NotificationManager()
	nmgr.RegisterSync(client.OnPMNtfn(c.pmNtfnHandler))
	nmgr.RegisterSync(client.OnGCMNtfn(c.gcmNtfnHandler))
	nmgr.RegisterSync(client.OnKXCompleted(c.kxNtfnHandler))
}

var _ types.ChatServiceServer = (*chatServer)(nil)

// InitChatService initializes and binds a ChatService server to the RPC server.
func (s *Server) InitChatService(cfg ChatServerCfg) error {
	pmStreams, err := newServerStreams[*types.ReceivedPM](cfg.RootReplayMsgLogs, "pm", cfg.Log)
	if err != nil {
		return err
	}

	gcmStreams, err := newServerStreams[*types.GCReceivedMsg](cfg.RootReplayMsgLogs, "gcm", cfg.Log)
	if err != nil {
		return err
	}

	kxStreams, err := newServerStreams[*types.KXCompleted](cfg.RootReplayMsgLogs, "kx", cfg.Log)
	if err != nil {
		return err
	}

	cs := &chatServer{
		cfg: cfg,
		log: cfg.Log,
		c:   cfg.Client,

		pmStreams:  pmStreams,
		gcmStreams: gcmStreams,
		kxStreams:  kxStreams,
	}
	cs.registerOfflineMessageStorageHandlers()
	s.services.Bind("ChatService", types.ChatServiceDefn(), cs)
	return nil
}
