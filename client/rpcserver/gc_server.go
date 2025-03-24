package rpcserver

import (
	"context"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
	"golang.org/x/exp/slices"
)

type GCServerCfg struct {
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
}

type gcServer struct {
	cfg GCServerCfg
	log slog.Logger
	c   *client.Client

	invitesStreams  *serverStreams[*types.ReceivedGCInvite]
	maddedStreams   *serverStreams[*types.GCMembersAddedEvent]
	mremovedStreams *serverStreams[*types.GCMembersRemovedEvent]
	joinedStreams   *serverStreams[*types.JoinedGCEvent]
}

func (g *gcServer) InviteToGC(_ context.Context, req *types.InviteToGCRequest, _ *types.InviteToGCResponse) error {
	gcid, err := g.c.GCIDByName(req.Gc)
	if err != nil {
		return err
	}
	uid, err := g.c.UIDByNick(req.User)
	if err != nil {
		return err
	}

	return g.c.InviteToGroupChat(gcid, uid)
}

func (g *gcServer) AcceptGCInvite(_ context.Context, req *types.AcceptGCInviteRequest, _ *types.AcceptGCInviteResponse) error {
	return g.c.AcceptGroupChatInvite(req.InviteId)
}

func (g *gcServer) KickFromGC(_ context.Context, req *types.KickFromGCRequest, _ *types.KickFromGCResponse) error {
	gcid, err := g.c.GCIDByName(req.Gc)
	if err != nil {
		return err
	}
	uid, err := g.c.UIDByNick(req.User)
	if err != nil {
		return err
	}

	return g.c.GCKick(gcid, uid, req.Reason)
}

func marshalRepeatedIDs(ids []zkidentity.ShortID, res [][]byte) [][]byte {
	if res != nil {
		res = res[:0]
	}
	res = slices.Grow(res, len(ids))
	for i := range ids {
		res = append(res, ids[i][:])
	}
	return res
}

func marshalRMGroupList(gl *rpc.RMGroupList, res *types.RMGroupList) *types.RMGroupList {
	if res == nil {
		res = new(types.RMGroupList)
	}
	*res = types.RMGroupList{
		Id:          gl.ID[:],
		Name:        gl.Name,
		Generation:  gl.Generation,
		Timestamp:   gl.Timestamp,
		Version:     uint32(gl.Version),
		Members:     marshalRepeatedIDs(gl.Members, res.Members),
		ExtraAdmins: marshalRepeatedIDs(gl.ExtraAdmins, res.ExtraAdmins),
	}
	return res
}

func (g *gcServer) GetGC(_ context.Context, req *types.GetGCRequest, res *types.GetGCResponse) error {
	gcid, err := g.c.GCIDByName(req.Gc)
	if err != nil {
		return err
	}
	gc, err := g.c.GetGC(gcid)
	if err != nil {
		return err
	}

	res.Gc = marshalRMGroupList(&gc, res.Gc)
	return nil
}

func (g *gcServer) List(_ context.Context, _ *types.ListGCsRequest, res *types.ListGCsResponse) error {
	gcs, err := g.c.ListGCs()
	if err != nil {
		return err
	}

	for i := range gcs {
		gc := &gcs[i]
		gci := &types.ListGCsResponse_GCInfo{
			Id:        gc.Metadata.ID[:],
			Name:      gc.Name(),
			Version:   uint32(gc.Metadata.Version),
			Timestamp: gc.Metadata.Timestamp,
			NbMembers: uint32(len(gc.Metadata.Members)),
		}
		res.Gcs = append(res.Gcs, gci)
	}

	return nil
}

func (g *gcServer) ReceivedGCInvites(ctx context.Context, req *types.ReceivedGCInvitesRequest, stream types.GCService_ReceivedGCInvitesServer) error {
	return g.invitesStreams.runStream(ctx, req.UnackedFrom, stream)
}

func marshalRMGroupInvite(v rpc.RMGroupInvite, res *types.RMGroupInvite) *types.RMGroupInvite {
	if res == nil {
		res = new(types.RMGroupInvite)
	}
	*res = types.RMGroupInvite{
		Id:          v.ID[:],
		Name:        v.Name,
		Token:       v.Token,
		Description: v.Description,
		Expires:     v.Expires,
		Version:     uint32(v.Version),
	}
	return res
}

func (g *gcServer) invitedToGCNtfnHandler(user *client.RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
	ntfn := &types.ReceivedGCInvite{
		InviterUid:  user.ID().Bytes(),
		InviterNick: user.Nick(),
		InviteId:    iid,
		Invite:      marshalRMGroupInvite(invite, nil),
	}
	g.invitesStreams.send(ntfn)
}

func (g *gcServer) AckReceivedGCInvites(_ context.Context, req *types.AckRequest, _ *types.AckResponse) error {
	return g.invitesStreams.ack(req.SequenceId)
}

func (g *gcServer) MembersAdded(ctx context.Context, req *types.GCMembersAddedRequest, stream types.GCService_MembersAddedServer) error {
	return g.maddedStreams.runStream(ctx, req.UnackedFrom, stream)
}

func (g *gcServer) marshalUserAndNick(uids []clientintf.UserID, res []*types.UserAndNick) []*types.UserAndNick {
	if len(res) > 0 {
		res = res[:0]
	}
	for _, uid := range uids {
		uid := uid
		nick, err := g.c.UserNick(uid)
		known := err == nil
		v := types.UserAndNick{
			Uid:   uid[:],
			Nick:  nick,
			Known: known,
		}
		res = append(res, &v)
	}
	return res
}

func (g *gcServer) gcMembersAddedNtfnHandler(gc rpc.RMGroupList, uids []clientintf.UserID) {
	alias, _ := g.c.GetGCAlias(gc.ID)
	if alias == "" {
		alias = gc.Name
	}

	ntfn := &types.GCMembersAddedEvent{
		Gc:     gc.ID[:],
		GcName: alias,
		Users:  g.marshalUserAndNick(uids, nil),
	}
	g.maddedStreams.send(ntfn)
}

func (g *gcServer) AckMembersAdded(_ context.Context, req *types.AckRequest, _ *types.AckResponse) error {
	return g.maddedStreams.ack(req.SequenceId)
}

func (g *gcServer) MembersRemoved(ctx context.Context, req *types.GCMembersRemovedRequest, stream types.GCService_MembersRemovedServer) error {
	return g.mremovedStreams.runStream(ctx, req.UnackedFrom, stream)
}

func (g *gcServer) gcMembersRemovedNtfnHandler(gc rpc.RMGroupList, uids []clientintf.UserID) {
	alias, _ := g.c.GetGCAlias(gc.ID)
	if alias == "" {
		alias = gc.Name
	}

	ntfn := &types.GCMembersRemovedEvent{
		Gc:     gc.ID[:],
		GcName: alias,
		Users:  g.marshalUserAndNick(uids, nil),
	}
	g.mremovedStreams.send(ntfn)
}

func (g *gcServer) AckMembersRemoved(_ context.Context, req *types.AckRequest, _ *types.AckResponse) error {
	return g.mremovedStreams.ack(req.SequenceId)
}

func (g *gcServer) JoinedGCs(ctx context.Context, req *types.JoinedGCsRequest, stream types.GCService_JoinedGCsServer) error {
	return g.joinedStreams.runStream(ctx, req.UnackedFrom, stream)
}

func (g *gcServer) joinedGCNtfnHandler(gc rpc.RMGroupList) {
	ntfn := &types.JoinedGCEvent{
		Gc: marshalRMGroupList(&gc, nil),
	}
	g.joinedStreams.send(ntfn)
}

func (g *gcServer) AckJoinedGCs(_ context.Context, req *types.AckRequest, _ *types.AckResponse) error {
	return g.joinedStreams.ack(req.SequenceId)
}

func (g *gcServer) registerOfflineMessageStorageHandlers() {
	nmgr := g.c.NotificationManager()
	nmgr.RegisterSync(client.OnInvitedToGCNtfn(g.invitedToGCNtfnHandler))
	nmgr.RegisterSync(client.OnAddedGCMembersNtfn(g.gcMembersAddedNtfnHandler))
	nmgr.RegisterSync(client.OnRemovedGCMembersNtfn(g.gcMembersRemovedNtfnHandler))
	nmgr.RegisterSync(client.OnJoinedGCNtfn(g.joinedGCNtfnHandler))
}

var _ types.GCServiceServer = (*gcServer)(nil)

// InitGCService initializes and binds a GCService server to the RPC server.
func (s *Server) InitGCService(cfg GCServerCfg) error {

	invitesStreams, err := newServerStreams[*types.ReceivedGCInvite](cfg.RootReplayMsgLogs, "gcinvites", cfg.Log)
	if err != nil {
		return err
	}

	maddedStreams, err := newServerStreams[*types.GCMembersAddedEvent](cfg.RootReplayMsgLogs, "gcaddedmembers", cfg.Log)
	if err != nil {
		return err
	}

	mremovedStreams, err := newServerStreams[*types.GCMembersRemovedEvent](cfg.RootReplayMsgLogs, "gcremmembers", cfg.Log)
	if err != nil {
		return err
	}

	joinedStreams, err := newServerStreams[*types.JoinedGCEvent](cfg.RootReplayMsgLogs, "gcjoined", cfg.Log)
	if err != nil {
		return err
	}

	gcs := &gcServer{
		cfg: cfg,
		log: cfg.Log,
		c:   cfg.Client,

		invitesStreams:  invitesStreams,
		maddedStreams:   maddedStreams,
		mremovedStreams: mremovedStreams,
		joinedStreams:   joinedStreams,
	}
	s.services.Bind("GCService", types.GCServiceDefn(), gcs)
	gcs.registerOfflineMessageStorageHandlers()
	return nil
}
