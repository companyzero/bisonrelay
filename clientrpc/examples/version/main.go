// Simple example program that uses BR's client rpc package to connect to a
// running client and fetch its version.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

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

	fmt.Println("AppName:", res.AppName)
	fmt.Println("AppVersion:", res.AppVersion)
	fmt.Println("GoRuntime:", res.GoRuntime)
	return nil
}

func main() {
	err := realMain()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
