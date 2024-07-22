package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/companyzero/bisonrelay/clientplugin/grpctypes"
	"github.com/companyzero/bisonrelay/clientrpc/jsonrpc"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var (
	flagURL = flag.String("url", "wss://127.0.0.1:7777/ws", "URL of the websocket endpoint")

	flagServerCertPath = flag.String("servercert", "/home/pongbot/brclient/rpc.cert", "Path to rpc.cert file")
	flagClientCertPath = flag.String("clientcert", "/home/pongbot/brclient/rpc-client.cert", "Path to rpc-client.cert file")
	flagClientKeyPath  = flag.String("clientkey", "/home/pongbot/brclient/rpc-client.key", "Path to rpc-client.key file")
)

type ChatPlugin struct {
	id      string
	name    string
	version string
	config  map[string]interface{}
	logger  slog.Logger
}

type server struct {
	grpctypes.UnimplementedPluginServiceServer

	ID          *zkidentity.ShortID
	mu          sync.Mutex
	clientReady chan string
	plugin      *ChatPlugin
	chatService types.ChatServiceClient
	debug       bool
}

func NewChatPlugin(id string) *ChatPlugin {
	return &ChatPlugin{
		id:      id, // Replace with actual ID generation
		name:    "ChatPlugin",
		version: "0.0.0",
		config:  make(map[string]interface{}),
	}
}

func (s *server) Init(req *grpctypes.PluginStartStreamRequest, stream grpctypes.PluginService_InitServer) error {
	return nil
}

func (s *server) GetVersion(ctx context.Context, req *grpctypes.PluginVersionRequest) (*grpctypes.PluginVersionResponse, error) {
	// Implement your logic here
	return &grpctypes.PluginVersionResponse{
		AppName:    s.plugin.name,
		AppVersion: s.plugin.version,
		GoRuntime:  runtime.Version(),
	}, nil
}

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

		fmt.Printf("-> %v %v\n", user, msg)
	}
	return r.Err()
}

func (s *server) CallAction(req *grpctypes.PluginCallActionStreamRequest, srv grpctypes.PluginService_CallActionServer) error {
	switch req.Action {
	case "chat":
		var ackRes types.AckResponse
		var ackReq types.AckRequest

		for {
			// Request a new stream if the connection breaks.
			streamReq := types.PMStreamRequest{UnackedFrom: ackReq.SequenceId}
			stream, err := s.chatService.PMStream(context.Background(), &streamReq)
			if errors.Is(err, context.Canceled) {
				// Program is done.
				return err
			}
			if err != nil {
				return fmt.Errorf("error while obtaining PM stream: %v", err)
			}

			for {
				var pm types.ReceivedPM
				err := stream.Recv(&pm)
				if errors.Is(err, context.Canceled) {
					// Program is done.
					return err
				}
				if err != nil {
					return fmt.Errorf("error while receiving stream: %v", err)
				}

				// Escape content before sending it to the terminal.
				nick := escapeNick(pm.Nick)
				var msg string
				if pm.Msg != nil {
					msg = escapeContent(pm.Msg.Message)
				}

				fmt.Printf("<- %v %v\n", nick, msg)

				// Ack to client that message is processed.
				ackReq.SequenceId = pm.SequenceId
				err = s.chatService.AckReceivedPM(context.Background(), &ackReq, &ackRes)
				if err != nil {
					return fmt.Errorf("error while ack'ing received pm: %v", err)
				}

				// Send a response back to the caller
				update := &grpctypes.PluginCallActionStreamResponse{
					Response: []byte(fmt.Sprintf("sending message from %v: %v", nick, msg)),
				}
				if err := srv.Send(update); err != nil {
					return fmt.Errorf("failed to send update: %v", err)
				}
			}

		}
	default:
		return fmt.Errorf("unsupported action: %v", req.Action)
	}
}

func (s *server) CallActionPlugin(ctx context.Context, req *grpctypes.PluginCallActionStreamRequest, cb func(grpctypes.PluginService_CallActionClient) error) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return fmt.Errorf("no metadata found in context")
	}

	clientID, ok := md["client-id"]
	if !ok || len(clientID) == 0 {
		return fmt.Errorf("client-id not found in metadata")
	}

	return nil
}

func NewServer(id *zkidentity.ShortID, debug bool) *server {
	return &server{
		ID:          id,
		clientReady: make(chan string, 10),
		debug:       debug,
	}
}

func realMain() error {
	flag.Parse()

	bknd := slog.NewBackend(os.Stderr)
	log := bknd.Logger("EXMP")
	log.SetLevel(slog.LevelDebug)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g, gctx := errgroup.WithContext(ctx)

	c, err := jsonrpc.NewWSClient(
		jsonrpc.WithWebsocketURL(*flagURL),
		jsonrpc.WithServerTLSCertPath(*flagServerCertPath),
		jsonrpc.WithClientTLSCert(*flagClientCertPath, *flagClientKeyPath),
		jsonrpc.WithClientLog(log),
	)
	if err != nil {
		return err
	}
	g.Go(func() error { return c.Run(gctx) })

	chat := types.NewChatServiceClient(c)
	req := &types.PublicIdentityReq{}
	var publicIdentity types.PublicIdentity
	err = chat.UserPublicIdentity(ctx, req, &publicIdentity)
	if err != nil {
		return err
	}

	clientID := hex.EncodeToString(publicIdentity.Identity[:])
	var zkShortID zkidentity.ShortID
	copy(zkShortID[:], clientID)
	srv := NewServer(&zkShortID, true)
	grpcServer := grpc.NewServer()
	plugin := NewChatPlugin(hex.EncodeToString(publicIdentity.Identity))

	srv.plugin = plugin
	srv.chatService = chat

	grpctypes.RegisterPluginServiceServer(grpcServer, srv)
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Errorf("failed to listen: %v", err)
		return err
	}
	fmt.Println("server listening at", lis.Addr())

	// Run the gRPC server in a separate goroutine
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Errorf("failed to serve: %v", err)
		}
	}()

	g.Go(func() error { return sendLoop(gctx, chat, log) })

	fmt.Println("Plugin initialized successfully:", plugin.name, plugin.version)

	return g.Wait()
}

func main() {
	err := realMain()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
