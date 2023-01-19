package rpcserver

import (
	"context"
	"runtime"
	"time"

	"github.com/companyzero/bisonrelay/clientrpc/types"
)

// versionServer is the server side implementation of the [types.VersionService].
type versionServer struct {
	AppName    string
	AppVersion string
}

var _ types.VersionServiceServer = (*versionServer)(nil)

func (v *versionServer) Version(_ context.Context, _ *types.VersionRequest, res *types.VersionResponse) error {
	*res = types.VersionResponse{
		AppName:    v.AppName,
		AppVersion: v.AppVersion,
		GoRuntime:  runtime.Version(),
	}
	return nil
}

func (s *versionServer) KeepaliveStream(ctx context.Context, req *types.KeepaliveStreamRequest,
	stream types.VersionService_KeepaliveStreamServer) error {

	interval := time.Duration(req.Interval * int64(time.Millisecond))
	if interval < time.Second {
		interval = time.Second
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}

		event := &types.KeepaliveEvent{
			Timestamp: time.Now().Unix(),
		}
		err := stream.Send(event)
		if err != nil {
			return err
		}
	}
}

// InitVersionService inits and binds a VersionService server to the RPC server.
func (s *Server) InitVersionService(appName, appVersion string) {
	s.services.Bind("VersionService", types.VersionServiceDefn(), &versionServer{
		AppName:    appName,
		AppVersion: appVersion,
	})
}
