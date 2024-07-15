package client

import (
	"context"
	"fmt"

	grpctypes "github.com/companyzero/bisonrelay/clientplugin/grpctypes"
	"github.com/decred/slog"
	"google.golang.org/grpc"
)

type PluginClient struct {
	pluginrpc grpctypes.PluginServiceClient

	ID     string
	Name   string
	Config map[string]interface{}
	stream grpctypes.PluginService_InitClient
	log    slog.Logger
}

type PluginClientCfg struct {
	TLSCertPath string
	Address     string
	Log         slog.Logger
}

func NewPluginClient(ctx context.Context, cfg PluginClientCfg) (*PluginClient, error) {
	// First attempt to establish a connection to lnd's RPC sever.
	// _, err := credentials.NewClientTLSFromFile(cfg.TLSCertPath, "")
	// if err != nil {
	// 	fmt.Printf("cfg Address: %+v\n\n", cfg.Address)
	// 	return nil, fmt.Errorf("unable to read cert file: %v", err)
	// }
	// opts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}

	conn, err := grpc.Dial(cfg.Address, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("unable to dial to dcrlnd's gRPC server: %v", err)
	}

	// // Start RPCs.
	pc := grpctypes.NewPluginServiceClient(conn)

	log := slog.Disabled
	if cfg.Log != nil {
		log = cfg.Log
	}

	return &PluginClient{
		pluginrpc: pc,
		log:       log,
	}, nil
}

func (p *PluginClient) Version(ctx context.Context) (*grpctypes.PluginVersionResponse, error) {
	req := &grpctypes.PluginVersionRequest{}
	return p.pluginrpc.GetVersion(ctx, req)
}

func (p *PluginClient) CallPluginAction(ctx context.Context, req *grpctypes.PluginCallActionStreamRequest, cb func(grpctypes.PluginService_CallActionClient) error) error {
	stream, err := p.pluginrpc.CallAction(ctx, req)
	if err != nil {
		return err
	}

	// Invoke the callback with the stream
	if err := cb(stream); err != nil {
		return err
	}

	return nil
}

func (p *PluginClient) Logger() slog.Logger {
	return p.log
}

func (p *PluginClient) InitPlugin(ctx context.Context, req *grpctypes.PluginStartStreamRequest, cb func(grpctypes.PluginService_InitClient)) error {
	gameStartedStream, err := p.pluginrpc.Init(context.Background(), req)
	if err != nil {
		return fmt.Errorf("error initing stream: %w", err)
	}
	p.stream = gameStartedStream
	cb(p.stream)

	return nil
}

// XXX From now on methods need to be implemented

// initializePlugins initializes all registered plugins.
func (c *Client) initializePlugins() error {
	// for _, plugin := range c.plugins {
	// 	if err := plugin.InitPlugin(c.ctx); err != nil {
	// 		c.log.Errorf("Failed to initialize plugin %s: %v", plugin.ID(), err)
	// 		return err
	// 	}
	// 	c.log.Infof("Initialized plugin %s", plugin.ID())
	// }
	return nil
}
