package jsonrpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
)

type testServerImpl struct {
	appName string
}

func (t *testServerImpl) Version(_ context.Context, _ *types.VersionRequest, res *types.VersionResponse) error {

	res.AppName = t.appName
	return nil
}

func (t *testServerImpl) KeepaliveStream(ctx context.Context, req *types.KeepaliveStreamRequest, stream types.VersionService_KeepaliveStreamServer) error {
	interval := time.Duration(req.Interval * int64(time.Millisecond))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}

		event := &types.KeepaliveEvent{
			Timestamp: time.Now().UnixMilli(),
		}
		err := stream.Send(event)
		if err != nil {
			return err
		}
	}
}

func TestPostPeerUnaryRequest(t *testing.T) {
	reqBody := bytes.NewBuffer([]byte(`{"jsonrpc":"2.0","id":1,"method":"VersionService.Version","params":{}}`))
	req := httptest.NewRequest("POST", "/", reqBody)
	w := httptest.NewRecorder()
	w.Body = bytes.NewBuffer(nil)
	services := &types.ServersMap{}
	server := &testServerImpl{appName: "testapp"}
	services.Bind("VersionService", types.VersionServiceDefn(), server)
	p := newServerPostPeer(w, req, services, slog.Disabled)
	err := p.run(context.Background())
	if !errors.Is(err, io.EOF) {
		t.Fatalf("unexpected error: %v", err)
	}
	res := strings.TrimSpace(w.Body.String())
	wantRes := `{"jsonrpc":"2.0","id":1,"result":{"appName":"testapp"}}`
	if res != wantRes {
		t.Fatalf("unexpected response content: %s", res)
	}
}
