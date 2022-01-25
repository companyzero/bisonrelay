package jsonrpc

import (
	"context"
	"encoding/json"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
)

type testPeer struct {
	p *peer
}

func newPeerPair(t *testing.T, services *types.ServersMap) (*testPeer, *testPeer) {
	cr, cw := io.Pipe()
	sr, sw := io.Pipe()
	logCli := slog.Disabled
	logSvr := slog.Disabled

	svrDec := json.NewDecoder(debugReader{log: logSvr, inner: cr})
	svrEnc := json.NewEncoder(debugWriter{log: logSvr, inner: sw})
	cliDec := json.NewDecoder(debugReader{log: logCli, inner: sr})
	cliEnc := json.NewEncoder(debugWriter{log: logCli, inner: cw})

	svrPeer := newPeer(services, logSvr, func() (*json.Decoder, error) {
		return svrDec, nil
	}, func() (*json.Encoder, error) {
		return svrEnc, nil
	}, func() error { return nil })

	cliPeer := newPeer(services, logCli, func() (*json.Decoder, error) {
		return cliDec, nil
	}, func() (*json.Encoder, error) {
		return cliEnc, nil
	}, func() error { return nil })

	return &testPeer{p: cliPeer}, &testPeer{p: svrPeer}
}

func TestWSPeerMultiStreams(t *testing.T) {
	server := &testServerImpl{appName: "testapp"}
	services := &types.ServersMap{}
	services.Bind("VersionService", types.VersionServiceDefn(), server)
	cli, svr := newPeerPair(t, services)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go cli.p.run(ctx)
	go svr.p.run(ctx)

	req1 := &types.KeepaliveStreamRequest{Interval: 10}
	req2 := &types.KeepaliveStreamRequest{Interval: 50}
	stream1, err := cli.p.requestStream(ctx, "VersionService.KeepaliveStream", req1)
	if err != nil {
		t.Fatal(err)
	}
	stream2, err := cli.p.requestStream(ctx, "VersionService.KeepaliveStream", req2)
	if err != nil {
		t.Fatal(err)
	}

	var nbReq1, nbReq2 int32
	go func() {
		for {
			var res types.KeepaliveEvent
			err := stream1.Recv(&res)
			if err != nil {
				return
			}
			atomic.AddInt32(&nbReq1, 1)
		}
	}()
	go func() {
		for {
			var res types.KeepaliveEvent
			err := stream2.Recv(&res)
			if err != nil {
				return
			}
			atomic.AddInt32(&nbReq2, 1)
		}
	}()

	// Run for 0.5 second.
	time.Sleep(time.Millisecond * 500)
	countReq1, countReq2 := atomic.LoadInt32(&nbReq1), atomic.LoadInt32(&nbReq2)

	// Number of req1 requests should be approximately 5 times nb of req2
	// requests (to within a margin of error due to timing effects).
	diff := countReq1 - (5 * countReq2)
	if diff < -5 || diff > 5 {
		t.Fatalf("Unexpected difference of request counts: %d vs %d",
			countReq1, countReq2*5)
	}
}

// TestPeerStreamMultipleNtfns tests that processing multiple received
// notifications works when the client code takes a while to start processing
// them.
func TestPeerStreamMultipleNtfns(t *testing.T) {
	server := &testServerImpl{appName: "testapp"}
	services := &types.ServersMap{}
	services.Bind("VersionService", types.VersionServiceDefn(), server)
	cli, svr := newPeerPair(t, services)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go cli.p.run(ctx)
	go svr.p.run(ctx)

	// Request the stream.
	req := &types.KeepaliveStreamRequest{Interval: 1}
	stream1, err := cli.p.requestStream(ctx, "VersionService.KeepaliveStream", req)
	if err != nil {
		t.Fatal(err)
	}

	// Wait until a number of remote events has been received and queued
	// internally.
	const nbWaitEvents = 10
	time.Sleep(time.Millisecond * nbWaitEvents)

	// Process the events. The timestamp should be monotonically increasing.
	var prevTS int64
	for i := 0; i < nbWaitEvents; i++ {
		var res types.KeepaliveEvent
		err := stream1.Recv(&res)
		if err != nil {
			t.Fatal(err)
		}
		if res.Timestamp <= prevTS {
			t.Fatalf("unexpected timestamp: got %d, want > %d",
				res.Timestamp, prevTS)
		}
		prevTS = res.Timestamp
	}
}
