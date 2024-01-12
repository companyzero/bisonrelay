package rpcserver

import (
	"context"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/slog"
)

type ContentServerCfg struct {
	// Client should be set to the [client.Client] instance.
	Client *client.Client

	// Log should be set to the app's logger.
	Log slog.Logger

	// RootReplayMsgLogs is the root dir where replaymsglogs are stored for
	// supported message types.
	RootReplayMsgLogs string
}

type contentServer struct {
	cfg ContentServerCfg
	log slog.Logger
	c   *client.Client

	completedStreams *serverStreams[*types.DownloadCompletedResponse]
}

func (cs *contentServer) DownloadsCompletedStream(ctx context.Context,
	req *types.DownloadsCompletedStreamRequest, stream types.ContentService_DownloadsCompletedStreamServer) error {

	return cs.completedStreams.runStream(ctx, req.UnackedFrom, stream)
}

func (cs *contentServer) fileDownloadCompletedHandler(ru *client.RemoteUser, fm rpc.FileMetadata, diskPath string) {
	ntfn := &types.DownloadCompletedResponse{
		Uid:      ru.ID().Bytes(),
		Nick:     ru.Nick(),
		DiskPath: diskPath,
		FileMetadata: &types.FileMetadata{
			Version:     fm.Version,
			Cost:        fm.Cost,
			Size:        fm.Size,
			Directory:   fm.Directory,
			Filename:    fm.Filename,
			Description: fm.Description,
			Hash:        fm.Hash,
			Signature:   fm.Signature,
			Attributes:  fm.Attributes,
		},
	}

	ntfn.FileMetadata.Manifest = make([]*types.FileManifest, len(fm.Manifest))
	for i, m := range fm.Manifest {
		ntfn.FileMetadata.Manifest[i] = &types.FileManifest{
			Index: m.Index,
			Hash:  m.Hash,
			Size:  m.Size,
		}
	}

	cs.completedStreams.send(ntfn)
}

func (cs *contentServer) AckDownloadCompleted(ctx context.Context, req *types.AckRequest, res *types.AckResponse) error {
	return cs.completedStreams.ack(req.SequenceId)
}

// registerOfflineMessageStorageHandlers registers the handlers for streams on
// the client's notification manager.
func (cs *contentServer) registerOfflineMessageStorageHandlers() {
	nmgr := cs.c.NotificationManager()
	nmgr.RegisterSync(client.OnFileDownloadCompleted(cs.fileDownloadCompletedHandler))
}

var _ types.ContentServiceServer = (*contentServer)(nil)

// InitContentService initializes and binds a ContentService server to the RPC server.
func (s *Server) InitContentService(cfg ContentServerCfg) error {

	completedStreams, err := newServerStreams[*types.DownloadCompletedResponse](cfg.RootReplayMsgLogs, "downscompleted", cfg.Log)
	if err != nil {
		return err
	}

	ps := &contentServer{
		cfg: cfg,
		log: cfg.Log,
		c:   cfg.Client,

		completedStreams: completedStreams,
	}
	ps.registerOfflineMessageStorageHandlers()
	s.services.Bind("ContentService", types.ContentServiceDefn(), ps)
	return nil
}
