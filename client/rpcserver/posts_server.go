package rpcserver

import (
	"context"
	"sync"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/internal/replaymsglog"
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

	replayMtx       sync.Mutex
	postStreams     *serverStreams[types.PostsService_PostsStreamServer]
	statusStreams   *serverStreams[types.PostsService_PostsStatusStreamServer]
	postsReplayLog  *replaymsglog.Log
	statusReplayLog *replaymsglog.Log
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
	id := replaymsglog.ID(req.UnackedFrom)
	p.replayMtx.Lock()

	// Send old messages before registering for the new ones.
	ntfn := new(types.ReceivedPost)
	err := p.postsReplayLog.ReadAfter(id, ntfn, func(id replaymsglog.ID) error {
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
		streamID, ctx = p.postStreams.register(ctx, stream)
	}
	p.replayMtx.Unlock()
	if err != nil {
		return err
	}

	<-ctx.Done()
	p.postStreams.unregister(streamID)
	return ctx.Err()
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

	// Save in replay file
	p.replayMtx.Lock()
	replayID, err := p.postsReplayLog.Store(ntfn)
	p.replayMtx.Unlock()
	if err != nil {
		p.log.Errorf("Unable to store Post in replay log: %v", err)
		return
	}
	ntfn.SequenceId = uint64(replayID)

	p.postStreams.iterateOver(func(id int32, stream types.PostsService_PostsStreamServer) {
		err := stream.Send(ntfn)
		if err != nil {
			p.log.Debugf("Unregistering Posts stream %d due to err: %v",
				id, err)
			p.postStreams.unregister(id)
		}
	})
}

func (p *postsServer) AckReceivedPost(_ context.Context, req *types.AckRequest, _ *types.AckResponse) error {
	id := replaymsglog.ID(req.SequenceId)
	err := p.postsReplayLog.ClearUpTo(id)
	if err != nil {
		p.log.Errorf("Unable to clear Posts log up to id %s: %v", id, err)
	}
	return nil
}

func (p *postsServer) PostsStatusStream(ctx context.Context, req *types.PostsStatusStreamRequest, stream types.PostsService_PostsStatusStreamServer) error {
	id := replaymsglog.ID(req.UnackedFrom)
	p.replayMtx.Lock()

	// Send old messages before registering for the new ones.
	ntfn := new(types.ReceivedPostStatus)
	err := p.statusReplayLog.ReadAfter(id, ntfn, func(id replaymsglog.ID) error {
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
		streamID, ctx = p.statusStreams.register(ctx, stream)
	}
	p.replayMtx.Unlock()
	if err != nil {
		return err
	}

	<-ctx.Done()
	p.statusStreams.unregister(streamID)
	return ctx.Err()
}

func (p *postsServer) postStatusNtfnHandler(user *client.RemoteUser, pid clientintf.PostID,
	statusFrom client.UserID, status rpc.PostMetadataStatus) {
	var relayerID []byte
	if user != nil {
		relayerID = user.ID().Bytes()
	}
	ntfn := &types.ReceivedPostStatus{
		RelayerId:  relayerID,
		PostId:     pid[:],
		StatusFrom: statusFrom[:],
		Status: &types.PostMetadataStatus{
			Version:    status.Version,
			From:       status.From,
			Link:       status.Link,
			Attributes: status.Attributes,
		},
	}

	// Save in replay file
	p.replayMtx.Lock()
	replayID, err := p.statusReplayLog.Store(ntfn)
	p.replayMtx.Unlock()
	if err != nil {
		p.log.Errorf("Unable to store Post in replay log: %v", err)
		return
	}
	ntfn.SequenceId = uint64(replayID)

	p.statusStreams.iterateOver(func(id int32, stream types.PostsService_PostsStatusStreamServer) {
		err := stream.Send(ntfn)
		if err != nil {
			p.log.Debugf("Unregistering Posts stream %d due to err: %v",
				id, err)
			p.statusStreams.unregister(id)
		}
	})
}

func (p *postsServer) AckReceivedPostStatus(_ context.Context, req *types.AckRequest, _ *types.AckResponse) error {
	id := replaymsglog.ID(req.SequenceId)
	err := p.statusReplayLog.ClearUpTo(id)
	if err != nil {
		p.log.Errorf("Unable to clear Posts log up to id %s: %v", id, err)
	}
	return nil
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

	postsReplayLog, err := replaymsglog.New(replaymsglog.Config{
		Log:     cfg.Log,
		Prefix:  "posts",
		RootDir: cfg.RootReplayMsgLogs,
		MaxSize: 1 << 23, // 8MiB
	})
	if err != nil {
		return err
	}

	statusReplayLog, err := replaymsglog.New(replaymsglog.Config{
		Log:     cfg.Log,
		Prefix:  "poststatus",
		RootDir: cfg.RootReplayMsgLogs,
		MaxSize: 1 << 23, // 8MiB
	})
	if err != nil {
		return err
	}

	ps := &postsServer{
		cfg: cfg,
		log: cfg.Log,
		c:   cfg.Client,

		postStreams:     &serverStreams[types.PostsService_PostsStreamServer]{},
		statusStreams:   &serverStreams[types.PostsService_PostsStatusStreamServer]{},
		postsReplayLog:  postsReplayLog,
		statusReplayLog: statusReplayLog,
	}
	ps.registerOfflineMessageStorageHandlers()
	s.services.Bind("PostsService", types.PostsServiceDefn(), ps)
	return nil
}
