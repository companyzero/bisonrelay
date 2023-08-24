// chat is an example showing how to use the clientrpc to send and receive
// messages.
//
// Messages to send are read from stdin, one message per line, in a <user> <msg>
// format.
//
// Messages received are sent to stdout, in the same format.

package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/clientrpc/jsonrpc"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
)

func sendLoop(ctx context.Context, chat types.ChatServiceClient, log slog.Logger) error {
	r := bufio.NewScanner(os.Stdin)
	for r.Scan() {
		line := strings.TrimSpace(r.Text())
		if len(line) < 0 {
			continue
		}

		tokens := strings.SplitN(line, " ", 2)
		if len(tokens) != 2 {
			log.Warn("Read line from stdin without 2 tokens")
			continue
		}

		user, msg := tokens[0], tokens[1]
		req := &types.PMRequest{
			User: user,
			Msg: &types.RMPrivateMessage{
				Message: msg,
			},
		}
		var res types.PMResponse
		err := chat.PM(ctx, req, &res)
		if errors.Is(err, context.Canceled) {
			// Program is done.
			return err
		}
		if err != nil {
			// Decide on whether to retry, give up, warn operator,
			// etc.
			log.Warnf("Unable to send last message: %v", err)
			continue
		}

		fmt.Println(nick, msg)
	}
	return r.Err()
}

func receiveLoop(ctx context.Context, chat types.ChatServiceClient, log slog.Logger) error {
	var ackRes types.AckResponse
	var ackReq types.AckRequest
	for {
		// Keep requesting a new stream if the connection breaks. Also
		// request any messages received since the last one we acked.
		streamReq := types.PMStreamRequest{UnackedFrom: ackReq.SequenceId}
		stream, err := chat.PMStream(ctx, &streamReq)
		if errors.Is(err, context.Canceled) {
			// Program is done.
			return err
		}
		if err != nil {
			log.Warn("Error while obtaining PM stream: %v", err)
			time.Sleep(time.Second) // Wait to try again.
			continue
		}

		for {
			var pm types.ReceivedPM
			err := stream.Recv(&pm)
			if errors.Is(err, context.Canceled) {
				// Program is done.
				return err
			}
			if err != nil {
				log.Warnf("Error while receiving stream: %v", err)
				break
			}

			// Escape content before sending it to the terminal.
			nick := escapeNick(pm.Nick)
			var msg string
			if pm.Msg != nil {
				msg = escapeContent(pm.Msg.Message)
			}

			log.Debugf("Received PM from '%s' with len %d and sequence %s",
				nick, len(msg), types.DebugSequenceID(pm.SequenceId))

			fmt.Println(nick, msg)

			// Ack to client that message is processed.
			ackReq.SequenceId = pm.SequenceId
			err = chat.AckReceivedPM(ctx, &ackReq, &ackRes)
			if err != nil {
				log.Warnf("Error while ack'ing received pm: %v", err)
				break
			}
		}

		time.Sleep(time.Second)
	}
}

var (
	flagURL            = flag.String("url", "wss://127.0.0.1:7676/ws", "URL of the websocket endpoint")
	flagServerCertPath = flag.String("servercert", "~/.brclient/rpc.cert", "Path to rpc.cert file")
	flagClientCertPath = flag.String("clientcert", "~/.brclient/rpc-client.cert", "Path to rpc-client.cert file")
	flagClientKeyPath  = flag.String("clientkey", "~/.brclient/rpc-client.key", "Path to rpc-client.key file")
)

func realMain() error {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g, gctx := errgroup.WithContext(ctx)

	bknd := slog.NewBackend(os.Stderr)
	log := bknd.Logger("EXMP")
	log.SetLevel(slog.LevelInfo)

	c, err := jsonrpc.NewWSClient(
		jsonrpc.WithWebsocketURL(*flagURL),
		jsonrpc.WithServerTLSCertPath(*flagServerCertPath),
		jsonrpc.WithClientTLSCert(*flagClientCertPath, *flagClientKeyPath),
		jsonrpc.WithClientLog(log),
	)
	if err != nil {
		return err
	}

	chat := types.NewChatServiceClient(c)
	g.Go(func() error { return c.Run(gctx) })
	g.Go(func() error { return sendLoop(gctx, chat, log) })
	g.Go(func() error { return receiveLoop(gctx, chat, log) })

	return g.Wait()
}

func main() {
	err := realMain()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
