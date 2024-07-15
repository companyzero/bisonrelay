package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"

	"github.com/companyzero/bisonrelay/clientplugin/grpctypes"
	"github.com/companyzero/bisonrelay/clientrpc/jsonrpc"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type ExamplePlugin struct {
	id      string
	name    string
	version string
	config  map[string]interface{}
	logger  slog.Logger
}

func NewExamplePlugin(id string) *ExamplePlugin {
	return &ExamplePlugin{
		id:      id,
		name:    "ExamplePlugin",
		version: "1.0.0",
		config:  make(map[string]interface{}),
	}
}

var (
	flagURL = flag.String("url", "wss://127.0.0.1:7777/ws", "URL of the websocket endpoint")

	flagServerCertPath = flag.String("servercert", "/home/pongbot/brclient/rpc.cert", "Path to rpc.cert file")
	flagClientCertPath = flag.String("clientcert", "/home/pongbot/brclient/rpc-client.cert", "Path to rpc-client.cert file")
	flagClientKeyPath  = flag.String("clientkey", "/home/pongbot/brclient/rpc-client.key", "Path to rpc-client.key file")
)

type server struct {
	grpctypes.UnimplementedPluginServiceServer

	ID             *zkidentity.ShortID
	plugin         *ExamplePlugin
	paymentService types.PaymentsServiceClient
	chatService    types.ChatServiceClient
	debug          bool
}

func (s *server) GetVersion(ctx context.Context, req *grpctypes.PluginVersionRequest) (*grpctypes.PluginVersionResponse, error) {
	// Implement your logic here
	return &grpctypes.PluginVersionResponse{
		AppName:    s.plugin.name,
		AppVersion: s.plugin.version,
		GoRuntime:  runtime.Version(),
	}, nil
}

func (s *server) CallAction(req *grpctypes.PluginCallActionStreamRequest, srv grpctypes.PluginService_CallActionServer) error {
	return nil
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
	// ID := hex.EncodeToString(clientID)
	// req.ClientId = pc.ID

	// Signal readiness after stream is initialized
	// stream, err := s.clients[ID].(context.Background(), req)
	// if err != nil {
	// 	return fmt.Errorf("error signaling readiness: %w", err)
	// }
	// pc.stream = stream
	// cb(pc.stream)

	return nil
}

func NewServer(id *zkidentity.ShortID, debug bool) *server {
	return &server{
		ID:    id,
		debug: debug,
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
	// plugin := NewExamplePlugin(id)
	plugin := NewExamplePlugin(hex.EncodeToString(publicIdentity.Identity))
	srv.plugin = plugin
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

	// plugin.InitPlugin()
	// Optionally, perform actions with the plugin
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
