package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	grpctypes "github.com/companyzero/bisonrelay/clientplugin/grpctypes"
	"github.com/decred/slog"
	"google.golang.org/grpc"
)

type PluginClient struct {
	pluginrpc grpctypes.PluginServiceClient

	ID      clientdb.PluginID
	Name    string
	Version string
	Config  PluginClientCfg
	Enabled bool
	stream  grpctypes.PluginService_InitClient
	log     slog.Logger
}

type PluginClientCfg struct {
	TLSCertPath string
	Address     string
	Log         slog.Logger
}

func NewPluginClient(ctx context.Context, id clientdb.PluginID, cfg PluginClientCfg) (*PluginClient, error) {
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

	p := &PluginClient{
		ID:        id,
		pluginrpc: pc,
		log:       log,
		Config: PluginClientCfg{
			Address:     cfg.Address,
			TLSCertPath: cfg.TLSCertPath,
		},
		Enabled: true,
	}

	version, err := p.GetVersion(ctx)
	if err != nil {
		return nil, err
	}
	p.Name = version.AppName
	p.Version = version.AppVersion

	return p, nil
}

func (p *PluginClient) GetVersion(ctx context.Context) (*grpctypes.PluginVersionResponse, error) {
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

// SavePluginInfo saves the plugin information to the database.
func (c *Client) SavePluginInfo(plugin *PluginClient) error {
	// Ensure plugin does not already exist.
	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		_, err := c.db.GetPlugin(tx, plugin.ID)
		if err == nil {
			return fmt.Errorf("plugin %s already exists: %w", plugin.Name, clientdb.ErrAlreadyExists)
		} else if !errors.Is(err, clientdb.ErrNotFound) {
			return err
		}

		// Convert PluginClientCfg to map[string]interface{}
		config := map[string]interface{}{
			"address":     plugin.Config.Address,
			"tlsCertPath": plugin.Config.TLSCertPath,
		}

		pdb := clientdb.Plugin{
			ID:        plugin.ID.String(),
			Name:      plugin.Name,
			Version:   plugin.Version,
			Config:    config,
			Enabled:   plugin.Enabled,
			Installed: time.Now(),
		}
		// Save the plugin data to the database.
		return c.db.SavePlugin(tx, pdb)
	})
}

// ListPlugins returns plugins saved on db.
func (c *Client) ListPlugins() ([]clientdb.Plugin, error) {
	var res []clientdb.Plugin
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.ListPlugins(tx)
		return err
	})
	return res, err
}

// GetEnabledPlugins returns the list of enabled plugins.
func (c *Client) GetEnabledPlugins() ([]PluginClient, error) {
	var res []PluginClient
	err := c.dbView(func(tx clientdb.ReadTx) error {
		plugins, err := c.db.ListPlugins(tx)
		if err != nil {
			return err
		}

		// Filter enabled plugins and convert to PluginClient.
		for _, plugin := range plugins {
			if plugin.Enabled {
				address, ok := plugin.Config["address"].(string)
				if !ok {
					return fmt.Errorf("address not found in plugin config for %s", plugin.ID)
				}

				pc := PluginClient{
					ID:      UserIDFromStr(plugin.ID),
					Name:    plugin.Name,
					Version: plugin.Version,
					Config: PluginClientCfg{
						Address: address,
					},
					Enabled: plugin.Enabled,
				}
				res = append(res, pc)
			}
		}
		return nil
	})

	return res, err
}
