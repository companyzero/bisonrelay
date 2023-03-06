package rpcserver

import (
	"context"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/slog"
)

type PostsServerCfg struct {
	// Client should be set to the [client.Client] instance.
	Client *client.Client

	// Log should be set to the app's logger.
	Log slog.Logger

	// RootReplayMsgLogs is the root dir where replaymsglogs are stored for
	// supported message types.
	RootReplayMsgLogs string
}

type postsServer struct {
	cfg PostsServerCfg
	log slog.Logger
	c   *client.Client

	postStreams   *serverStreams[*types.ReceivedPost]
	statusStreams *serverStreams[*types.ReceivedPostStatus]
}

func (p *postsServer) SubscribeToPosts(_ context.Context, req *types.SubscribeToPostsRequest, _ *types.SubscribeToPostsResponse) error {
	user, err := p.c.UserByNick(req.User)
	if err != nil {
		return err
	}
	return p.c.SubscribeToPosts(user.ID())
}

func (p *postsServer) UnsubscribeToPosts(_ context.Context, req *types.UnsubscribeToPostsRequest, _ *types.UnsubscribeToPostsResponse) error {
	user, err := p.c.UserByNick(req.User)
	if err != nil {
		return err
	}
	return p.c.UnsubscribeToPosts(user.ID())
}

func (p *postsServer) PostsStream(ctx context.Context, req *types.PostsStreamRequest, stream types.PostsService_PostsStreamServer) error {
	return p.postStreams.runStream(ctx, req.UnackedFrom, stream)
}

func (p *postsServer) postsNtfnHandler(ru *client.RemoteUser, summ clientdb.PostSummary, pm rpc.PostMetadata) {
	var relayerID []byte
	if ru != nil {
		relayerID = ru.ID().Bytes()
	}
	ntfn := &types.ReceivedPost{
		RelayerId: relayerID,
		Summary: &types.PostSummary{
			Id:           summ.ID[:],
			From:         summ.From[:],
			AuthorId:     summ.AuthorID[:],
			AuthorNick:   summ.AuthorNick,
			Date:         summ.Date.Unix(),
			LastStatusTs: summ.LastStatusTS.Unix(),
			Title:        summ.Title,
		},
		Post: &types.PostMetadata{
			Version:    pm.Version,
			Attributes: pm.Attributes,
		},
	}

	p.postStreams.send(ntfn)
}

func (p *postsServer) AckReceivedPost(_ context.Context, req *types.AckRequest, _ *types.AckResponse) error {
	return p.postStreams.ack(req.SequenceId)
}

func (p *postsServer) PostsStatusStream(ctx context.Context, req *types.PostsStatusStreamRequest, stream types.PostsService_PostsStatusStreamServer) error {
	return p.statusStreams.runStream(ctx, req.UnackedFrom, stream)
}

func (p *postsServer) postStatusNtfnHandler(user *client.RemoteUser, pid clientintf.PostID,
	statusFrom client.UserID, status rpc.PostMetadataStatus) {

	var relayerID []byte
	if user != nil {
		relayerID = user.ID().Bytes()
	}

	// Determine the best nick.
	var statusFromNick string
	if fromNick, err := p.c.UserNick(statusFrom); err == nil {
		// Local client has this user, use this as nick.
		statusFromNick = fromNick
	} else if fromNick, ok := status.Attributes[rpc.RMPFromNick]; ok {
		// Status includes a nick, use the included one.
		statusFromNick = fromNick
	} else {
		// Status does not include nick, use the prefix of the ID.
		statusFromNick = statusFrom.ShortLogID()
	}

	ntfn := &types.ReceivedPostStatus{
		RelayerId:      relayerID,
		PostId:         pid[:],
		StatusFrom:     statusFrom[:],
		StatusFromNick: statusFromNick,
		Status: &types.PostMetadataStatus{
			Version:    status.Version,
			From:       status.From,
			Link:       status.Link,
			Attributes: status.Attributes,
		},
	}

	p.statusStreams.send(ntfn)
}

func (p *postsServer) AckReceivedPostStatus(_ context.Context, req *types.AckRequest, _ *types.AckResponse) error {
	return p.statusStreams.ack(req.SequenceId)
}

// registerOfflineMessageStorageHandlers registers the handlers for streams on
// the client's notification manager.
func (p *postsServer) registerOfflineMessageStorageHandlers() {
	nmgr := p.c.NotificationManager()
	nmgr.RegisterSync(client.OnPostRcvdNtfn(p.postsNtfnHandler))
	nmgr.RegisterSync(client.OnPostStatusRcvdNtfn(p.postStatusNtfnHandler))
}

var _ types.PostsServiceServer = (*postsServer)(nil)

// InitPostService initializes and binds a PostsService server to the RPC server.
func (s *Server) InitPostsService(cfg PostsServerCfg) error {

	postsStreams, err := newServerStreams[*types.ReceivedPost](cfg.RootReplayMsgLogs, "posts", cfg.Log)
	if err != nil {
		return err
	}

	statusStreams, err := newServerStreams[*types.ReceivedPostStatus](cfg.RootReplayMsgLogs, "poststatus", cfg.Log)
	if err != nil {
		return err
	}

	ps := &postsServer{
		cfg: cfg,
		log: cfg.Log,
		c:   cfg.Client,

		postStreams:   postsStreams,
		statusStreams: statusStreams,
	}
	ps.registerOfflineMessageStorageHandlers()
	s.services.Bind("PostsService", types.PostsServiceDefn(), ps)
	return nil
}
