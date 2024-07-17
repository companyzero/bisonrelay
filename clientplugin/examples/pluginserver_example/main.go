package main

import (
	"context"
	"log"
	"net"

	"github.com/companyzero/bisonrelay/clientplugin/grpctypes"
	"google.golang.org/grpc"
)

type pluginServer struct {
	grpctypes.UnimplementedPluginServiceServer
}

func (s *pluginServer) Init(req *grpctypes.PluginStartStreamRequest, stream grpctypes.PluginService_InitServer) error {
	// Implement your Init logic here
	return nil
}

func (s *pluginServer) CallAction(req *grpctypes.PluginCallActionStreamRequest, stream grpctypes.PluginService_CallActionServer) error {
	// Implement your CallAction logic here
	return nil
}

func (s *pluginServer) GetVersion(ctx context.Context, req *grpctypes.PluginVersionRequest) (*grpctypes.PluginVersionResponse, error) {
	// Implement your GetVersion logic here
	return &grpctypes.PluginVersionResponse{
		AppName:    "ExamplePlugin",
		AppVersion: "1.0.0",
	}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	grpctypes.RegisterPluginServiceServer(s, &pluginServer{})

	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
