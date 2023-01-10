package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/companyzero/bisonrelay/clientrpc/jsonrpc"
	"golang.org/x/sync/errgroup"
)

func unaryRequest(ctx context.Context, c *jsonrpc.WSClient, cfg *config) error {
	res := cfg.resProducer()
	err := c.Request(ctx, cfg.cmdName, cfg.req, res)
	if err != nil {
		return err
	}

	encoded, err := cfg.marshalOpts.Marshal(res)
	if err != nil {
		return err
	}
	os.Stdout.Write(encoded)
	os.Stdout.Write([]byte{'\n'})
	return errCmdDone
}

func streamRequest(ctx context.Context, c *jsonrpc.WSClient, cfg *config) error {
	stream, err := c.Stream(ctx, cfg.cmdName, cfg.req)
	if err != nil {
		return err
	}

	for {
		res := cfg.resProducer()
		err := stream.Recv(res)
		if err != nil {
			return err
		}
		encoded, err := cfg.marshalOpts.Marshal(res)
		if err != nil {
			return err
		}
		os.Stdout.Write(encoded)
		os.Stdout.Write([]byte{'\n'})
	}
}

func realMain() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Setup the main context and errgroup.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g, gctx := errgroup.WithContext(ctx)

	// Setup the peer.
	c, err := jsonrpc.NewWSClient(
		jsonrpc.WithWebsocketURL(cfg.url),
		jsonrpc.WithServerTLSCertPath(cfg.serverCertPath),
		jsonrpc.WithClientTLSCert(cfg.clientCertPath, cfg.clientKeyPath),
		jsonrpc.WithClientLog(cfg.log),
	)
	if err != nil {
		return err
	}
	g.Go(func() error { return c.Run(gctx) })

	if !cfg.isStreaming {
		g.Go(func() error { return unaryRequest(gctx, c, cfg) })
	} else {
		g.Go(func() error { return streamRequest(gctx, c, cfg) })
	}

	return g.Wait()
}

func main() {
	err := realMain()
	if err != nil && !errors.Is(err, errCmdDone) {
		fmt.Println(err)
		os.Exit(1)
	}
}
