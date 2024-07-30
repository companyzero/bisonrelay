// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.12
// source: pluginrpc.proto

package grpctypes

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// PluginServiceClient is the client API for PluginService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type PluginServiceClient interface {
	Init(ctx context.Context, in *PluginStartStreamRequest, opts ...grpc.CallOption) (PluginService_InitClient, error)
	CallAction(ctx context.Context, in *PluginCallActionStreamRequest, opts ...grpc.CallOption) (PluginService_CallActionClient, error)
	GetVersion(ctx context.Context, in *PluginVersionRequest, opts ...grpc.CallOption) (*PluginVersionResponse, error)
	Render(ctx context.Context, in *RenderRequest, opts ...grpc.CallOption) (*RenderResponse, error)
}

type pluginServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewPluginServiceClient(cc grpc.ClientConnInterface) PluginServiceClient {
	return &pluginServiceClient{cc}
}

func (c *pluginServiceClient) Init(ctx context.Context, in *PluginStartStreamRequest, opts ...grpc.CallOption) (PluginService_InitClient, error) {
	stream, err := c.cc.NewStream(ctx, &PluginService_ServiceDesc.Streams[0], "/PluginService/Init", opts...)
	if err != nil {
		return nil, err
	}
	x := &pluginServiceInitClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type PluginService_InitClient interface {
	Recv() (*PluginStartStreamResponse, error)
	grpc.ClientStream
}

type pluginServiceInitClient struct {
	grpc.ClientStream
}

func (x *pluginServiceInitClient) Recv() (*PluginStartStreamResponse, error) {
	m := new(PluginStartStreamResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *pluginServiceClient) CallAction(ctx context.Context, in *PluginCallActionStreamRequest, opts ...grpc.CallOption) (PluginService_CallActionClient, error) {
	stream, err := c.cc.NewStream(ctx, &PluginService_ServiceDesc.Streams[1], "/PluginService/CallAction", opts...)
	if err != nil {
		return nil, err
	}
	x := &pluginServiceCallActionClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type PluginService_CallActionClient interface {
	Recv() (*PluginCallActionStreamResponse, error)
	grpc.ClientStream
}

type pluginServiceCallActionClient struct {
	grpc.ClientStream
}

func (x *pluginServiceCallActionClient) Recv() (*PluginCallActionStreamResponse, error) {
	m := new(PluginCallActionStreamResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *pluginServiceClient) GetVersion(ctx context.Context, in *PluginVersionRequest, opts ...grpc.CallOption) (*PluginVersionResponse, error) {
	out := new(PluginVersionResponse)
	err := c.cc.Invoke(ctx, "/PluginService/GetVersion", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *pluginServiceClient) Render(ctx context.Context, in *RenderRequest, opts ...grpc.CallOption) (*RenderResponse, error) {
	out := new(RenderResponse)
	err := c.cc.Invoke(ctx, "/PluginService/Render", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PluginServiceServer is the server API for PluginService service.
// All implementations must embed UnimplementedPluginServiceServer
// for forward compatibility
type PluginServiceServer interface {
	Init(*PluginStartStreamRequest, PluginService_InitServer) error
	CallAction(*PluginCallActionStreamRequest, PluginService_CallActionServer) error
	GetVersion(context.Context, *PluginVersionRequest) (*PluginVersionResponse, error)
	Render(context.Context, *RenderRequest) (*RenderResponse, error)
	mustEmbedUnimplementedPluginServiceServer()
}

// UnimplementedPluginServiceServer must be embedded to have forward compatible implementations.
type UnimplementedPluginServiceServer struct {
}

func (UnimplementedPluginServiceServer) Init(*PluginStartStreamRequest, PluginService_InitServer) error {
	return status.Errorf(codes.Unimplemented, "method Init not implemented")
}
func (UnimplementedPluginServiceServer) CallAction(*PluginCallActionStreamRequest, PluginService_CallActionServer) error {
	return status.Errorf(codes.Unimplemented, "method CallAction not implemented")
}
func (UnimplementedPluginServiceServer) GetVersion(context.Context, *PluginVersionRequest) (*PluginVersionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetVersion not implemented")
}
func (UnimplementedPluginServiceServer) Render(context.Context, *RenderRequest) (*RenderResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Render not implemented")
}
func (UnimplementedPluginServiceServer) mustEmbedUnimplementedPluginServiceServer() {}

// UnsafePluginServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to PluginServiceServer will
// result in compilation errors.
type UnsafePluginServiceServer interface {
	mustEmbedUnimplementedPluginServiceServer()
}

func RegisterPluginServiceServer(s grpc.ServiceRegistrar, srv PluginServiceServer) {
	s.RegisterService(&PluginService_ServiceDesc, srv)
}

func _PluginService_Init_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(PluginStartStreamRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(PluginServiceServer).Init(m, &pluginServiceInitServer{stream})
}

type PluginService_InitServer interface {
	Send(*PluginStartStreamResponse) error
	grpc.ServerStream
}

type pluginServiceInitServer struct {
	grpc.ServerStream
}

func (x *pluginServiceInitServer) Send(m *PluginStartStreamResponse) error {
	return x.ServerStream.SendMsg(m)
}

func _PluginService_CallAction_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(PluginCallActionStreamRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(PluginServiceServer).CallAction(m, &pluginServiceCallActionServer{stream})
}

type PluginService_CallActionServer interface {
	Send(*PluginCallActionStreamResponse) error
	grpc.ServerStream
}

type pluginServiceCallActionServer struct {
	grpc.ServerStream
}

func (x *pluginServiceCallActionServer) Send(m *PluginCallActionStreamResponse) error {
	return x.ServerStream.SendMsg(m)
}

func _PluginService_GetVersion_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PluginVersionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServiceServer).GetVersion(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/PluginService/GetVersion",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PluginServiceServer).GetVersion(ctx, req.(*PluginVersionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PluginService_Render_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RenderRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServiceServer).Render(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/PluginService/Render",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PluginServiceServer).Render(ctx, req.(*RenderRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// PluginService_ServiceDesc is the grpc.ServiceDesc for PluginService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var PluginService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "PluginService",
	HandlerType: (*PluginServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetVersion",
			Handler:    _PluginService_GetVersion_Handler,
		},
		{
			MethodName: "Render",
			Handler:    _PluginService_Render_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Init",
			Handler:       _PluginService_Init_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "CallAction",
			Handler:       _PluginService_CallAction_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "pluginrpc.proto",
}
