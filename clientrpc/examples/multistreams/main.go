package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/companyzero/bisonrelay/clientrpc/jsonrpc"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
)

var (
	flagURL            = flag.String("url", "wss://127.0.0.1:7676/ws", "URL of the websocket endpoint")
	flagServerCertPath = flag.String("servercert", "~/.brclient/rpc.cert", "Path to rpc.cert file")
	flagClientCertPath = flag.String("clientcert", "~/.brclient/rpc-client.cert", "Path to rpc-client.cert file")
	flagClientKeyPath  = flag.String("clientkey", "~/.brclient/rpc-client.key", "Path to rpc-client.key file")
)

func requestStream(label string, intervalMS int64, vc types.VersionServiceClient) {
	req := &types.KeepaliveStreamRequest{Interval: intervalMS}
	stream, err := vc.KeepaliveStream(context.Background(), req)
	if err != nil {
		panic(err)
	}

	for {
		var e types.KeepaliveEvent
		if err := stream.Recv(&e); err != nil {
			panic(err)
		}

		serverTime := time.Unix(e.Timestamp, 0)
		fmt.Printf("%s: %s\n", label, serverTime)
	}
}

func realMain() error {
	flag.Parse()

	bknd := slog.NewBackend(os.Stderr)
	log := bknd.Logger("EXMP")
	log.SetLevel(slog.LevelDebug)

	c, err := jsonrpc.NewWSClient(
		jsonrpc.WithWebsocketURL(*flagURL),
		jsonrpc.WithServerTLSCertPath(*flagServerCertPath),
		jsonrpc.WithClientTLSCert(*flagClientCertPath, *flagClientKeyPath),
		jsonrpc.WithClientLog(log),
	)
	if err != nil {
		return err
	}
	go c.Run(context.Background())

	vc := types.NewVersionServiceClient(c)
	res := &types.VersionResponse{}
	err = vc.Version(context.Background(), nil, res)
	if err != nil {
		return err
	}

	// Start streams.
	go requestStream("stream1", 1000, vc)
	time.Sleep(time.Millisecond * 500)
	go requestStream("stream2", 5000, vc)

	<-context.Background().Done()

	return nil
}

func main() {
	err := realMain()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
