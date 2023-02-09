// Code generated by protoc-gen-go-svcintf. DO NOT EDIT.
// source: clientrpc.proto

package types

import (
	context "context"
	proto "google.golang.org/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

// VersionServiceClient is the client API for VersionService service.
type VersionServiceClient interface {
	// Version returns version information about the server.
	Version(ctx context.Context, in *VersionRequest, out *VersionResponse) error
	// KeepaliveStream returns a stream where the server continuously writes
	// keepalive events.
	//
	// The stream only terminates if the client requests it or the connection to
	// the server is closed.
	KeepaliveStream(ctx context.Context, in *KeepaliveStreamRequest) (VersionService_KeepaliveStreamClient, error)
}

type client_VersionService struct {
	c    ClientConn
	defn ServiceDefn
}

func (c *client_VersionService) Version(ctx context.Context, in *VersionRequest, out *VersionResponse) error {
	const method = "Version"
	return c.defn.Methods[method].ClientHandler(c.c, ctx, in, out)
}

type VersionService_KeepaliveStreamClient interface {
	Recv(*KeepaliveEvent) error
}

func (c *client_VersionService) KeepaliveStream(ctx context.Context, in *KeepaliveStreamRequest) (VersionService_KeepaliveStreamClient, error) {
	const method = "KeepaliveStream"
	inner, err := c.defn.Methods[method].ClientStreamHandler(c.c, ctx, in)
	if err != nil {
		return nil, err
	}
	return streamerImpl[*KeepaliveEvent]{c: inner}, nil
}

func NewVersionServiceClient(c ClientConn) VersionServiceClient {
	return &client_VersionService{c: c, defn: VersionServiceDefn()}
}

// VersionServiceServer is the server API for VersionService service.
type VersionServiceServer interface {
	// Version returns version information about the server.
	Version(context.Context, *VersionRequest, *VersionResponse) error
	// KeepaliveStream returns a stream where the server continuously writes
	// keepalive events.
	//
	// The stream only terminates if the client requests it or the connection to
	// the server is closed.
	KeepaliveStream(context.Context, *KeepaliveStreamRequest, VersionService_KeepaliveStreamServer) error
}

type VersionService_KeepaliveStreamServer interface {
	Send(m *KeepaliveEvent) error
}

func VersionServiceDefn() ServiceDefn {
	return ServiceDefn{
		Name: "VersionService",
		Methods: map[string]MethodDefn{
			"Version": {
				IsStreaming:  false,
				NewRequest:   func() proto.Message { return new(VersionRequest) },
				NewResponse:  func() proto.Message { return new(VersionResponse) },
				RequestDefn:  func() protoreflect.MessageDescriptor { return new(VersionRequest).ProtoReflect().Descriptor() },
				ResponseDefn: func() protoreflect.MessageDescriptor { return new(VersionResponse).ProtoReflect().Descriptor() },
				Help:         "Version returns version information about the server.",
				ServerHandler: func(x interface{}, ctx context.Context, request, response proto.Message) error {
					return x.(VersionServiceServer).Version(ctx, request.(*VersionRequest), response.(*VersionResponse))
				},
				ClientHandler: func(conn ClientConn, ctx context.Context, request, response proto.Message) error {
					method := "VersionService.Version"
					return conn.Request(ctx, method, request, response)
				},
			},
			"KeepaliveStream": {
				IsStreaming:  true,
				NewRequest:   func() proto.Message { return new(KeepaliveStreamRequest) },
				NewResponse:  func() proto.Message { return new(KeepaliveEvent) },
				RequestDefn:  func() protoreflect.MessageDescriptor { return new(KeepaliveStreamRequest).ProtoReflect().Descriptor() },
				ResponseDefn: func() protoreflect.MessageDescriptor { return new(KeepaliveEvent).ProtoReflect().Descriptor() },
				Help: "KeepaliveStream returns a stream where the server continuously writes keepalive events.\n" +
					"The stream only terminates if the client requests it or the connection to the server is closed.",
				ServerStreamHandler: func(x interface{}, ctx context.Context, request proto.Message, stream ServerStream) error {
					return x.(VersionServiceServer).KeepaliveStream(ctx, request.(*KeepaliveStreamRequest), streamerImpl[*KeepaliveEvent]{s: stream})
				},
				ClientStreamHandler: func(conn ClientConn, ctx context.Context, request proto.Message) (ClientStream, error) {
					method := "VersionService.KeepaliveStream"
					return conn.Stream(ctx, method, request)
				},
			},
		},
	}
}

// ChatServiceClient is the client API for ChatService service.
type ChatServiceClient interface {
	// PM sends a private message to a user of the client.
	PM(ctx context.Context, in *PMRequest, out *PMResponse) error
	// PMStream returns a stream that gets PMs received by the client.
	PMStream(ctx context.Context, in *PMStreamRequest) (ChatService_PMStreamClient, error)
	// AckReceivedPM acks to the server that PMs up to a sequence ID have been
	// processed.
	AckReceivedPM(ctx context.Context, in *AckRequest, out *AckResponse) error
	// GCM sends a message in a GC.
	GCM(ctx context.Context, in *GCMRequest, out *GCMResponse) error
	// GCMStream returns a stream that gets GC messages received by the client.
	GCMStream(ctx context.Context, in *GCMStreamRequest) (ChatService_GCMStreamClient, error)
	// AckReceivedGCM acks to the server that GCMs up to a sequence ID have been
	// processed.
	AckReceivedGCM(ctx context.Context, in *AckRequest, out *AckResponse) error
}

type client_ChatService struct {
	c    ClientConn
	defn ServiceDefn
}

func (c *client_ChatService) PM(ctx context.Context, in *PMRequest, out *PMResponse) error {
	const method = "PM"
	return c.defn.Methods[method].ClientHandler(c.c, ctx, in, out)
}

type ChatService_PMStreamClient interface {
	Recv(*ReceivedPM) error
}

func (c *client_ChatService) PMStream(ctx context.Context, in *PMStreamRequest) (ChatService_PMStreamClient, error) {
	const method = "PMStream"
	inner, err := c.defn.Methods[method].ClientStreamHandler(c.c, ctx, in)
	if err != nil {
		return nil, err
	}
	return streamerImpl[*ReceivedPM]{c: inner}, nil
}

func (c *client_ChatService) AckReceivedPM(ctx context.Context, in *AckRequest, out *AckResponse) error {
	const method = "AckReceivedPM"
	return c.defn.Methods[method].ClientHandler(c.c, ctx, in, out)
}

func (c *client_ChatService) GCM(ctx context.Context, in *GCMRequest, out *GCMResponse) error {
	const method = "GCM"
	return c.defn.Methods[method].ClientHandler(c.c, ctx, in, out)
}

type ChatService_GCMStreamClient interface {
	Recv(*GCReceivedMsg) error
}

func (c *client_ChatService) GCMStream(ctx context.Context, in *GCMStreamRequest) (ChatService_GCMStreamClient, error) {
	const method = "GCMStream"
	inner, err := c.defn.Methods[method].ClientStreamHandler(c.c, ctx, in)
	if err != nil {
		return nil, err
	}
	return streamerImpl[*GCReceivedMsg]{c: inner}, nil
}

func (c *client_ChatService) AckReceivedGCM(ctx context.Context, in *AckRequest, out *AckResponse) error {
	const method = "AckReceivedGCM"
	return c.defn.Methods[method].ClientHandler(c.c, ctx, in, out)
}

func NewChatServiceClient(c ClientConn) ChatServiceClient {
	return &client_ChatService{c: c, defn: ChatServiceDefn()}
}

// ChatServiceServer is the server API for ChatService service.
type ChatServiceServer interface {
	// PM sends a private message to a user of the client.
	PM(context.Context, *PMRequest, *PMResponse) error
	// PMStream returns a stream that gets PMs received by the client.
	PMStream(context.Context, *PMStreamRequest, ChatService_PMStreamServer) error
	// AckReceivedPM acks to the server that PMs up to a sequence ID have been
	// processed.
	AckReceivedPM(context.Context, *AckRequest, *AckResponse) error
	// GCM sends a message in a GC.
	GCM(context.Context, *GCMRequest, *GCMResponse) error
	// GCMStream returns a stream that gets GC messages received by the client.
	GCMStream(context.Context, *GCMStreamRequest, ChatService_GCMStreamServer) error
	// AckReceivedGCM acks to the server that GCMs up to a sequence ID have been
	// processed.
	AckReceivedGCM(context.Context, *AckRequest, *AckResponse) error
}

type ChatService_PMStreamServer interface {
	Send(m *ReceivedPM) error
}

type ChatService_GCMStreamServer interface {
	Send(m *GCReceivedMsg) error
}

func ChatServiceDefn() ServiceDefn {
	return ServiceDefn{
		Name: "ChatService",
		Methods: map[string]MethodDefn{
			"PM": {
				IsStreaming:  false,
				NewRequest:   func() proto.Message { return new(PMRequest) },
				NewResponse:  func() proto.Message { return new(PMResponse) },
				RequestDefn:  func() protoreflect.MessageDescriptor { return new(PMRequest).ProtoReflect().Descriptor() },
				ResponseDefn: func() protoreflect.MessageDescriptor { return new(PMResponse).ProtoReflect().Descriptor() },
				Help:         "PM sends a private message to a user of the client.",
				ServerHandler: func(x interface{}, ctx context.Context, request, response proto.Message) error {
					return x.(ChatServiceServer).PM(ctx, request.(*PMRequest), response.(*PMResponse))
				},
				ClientHandler: func(conn ClientConn, ctx context.Context, request, response proto.Message) error {
					method := "ChatService.PM"
					return conn.Request(ctx, method, request, response)
				},
			},
			"PMStream": {
				IsStreaming:  true,
				NewRequest:   func() proto.Message { return new(PMStreamRequest) },
				NewResponse:  func() proto.Message { return new(ReceivedPM) },
				RequestDefn:  func() protoreflect.MessageDescriptor { return new(PMStreamRequest).ProtoReflect().Descriptor() },
				ResponseDefn: func() protoreflect.MessageDescriptor { return new(ReceivedPM).ProtoReflect().Descriptor() },
				Help:         "PMStream returns a stream that gets PMs received by the client.",
				ServerStreamHandler: func(x interface{}, ctx context.Context, request proto.Message, stream ServerStream) error {
					return x.(ChatServiceServer).PMStream(ctx, request.(*PMStreamRequest), streamerImpl[*ReceivedPM]{s: stream})
				},
				ClientStreamHandler: func(conn ClientConn, ctx context.Context, request proto.Message) (ClientStream, error) {
					method := "ChatService.PMStream"
					return conn.Stream(ctx, method, request)
				},
			},
			"AckReceivedPM": {
				IsStreaming:  false,
				NewRequest:   func() proto.Message { return new(AckRequest) },
				NewResponse:  func() proto.Message { return new(AckResponse) },
				RequestDefn:  func() protoreflect.MessageDescriptor { return new(AckRequest).ProtoReflect().Descriptor() },
				ResponseDefn: func() protoreflect.MessageDescriptor { return new(AckResponse).ProtoReflect().Descriptor() },
				Help:         "AckReceivedPM acks to the server that PMs up to a sequence ID have been processed.",
				ServerHandler: func(x interface{}, ctx context.Context, request, response proto.Message) error {
					return x.(ChatServiceServer).AckReceivedPM(ctx, request.(*AckRequest), response.(*AckResponse))
				},
				ClientHandler: func(conn ClientConn, ctx context.Context, request, response proto.Message) error {
					method := "ChatService.AckReceivedPM"
					return conn.Request(ctx, method, request, response)
				},
			},
			"GCM": {
				IsStreaming:  false,
				NewRequest:   func() proto.Message { return new(GCMRequest) },
				NewResponse:  func() proto.Message { return new(GCMResponse) },
				RequestDefn:  func() protoreflect.MessageDescriptor { return new(GCMRequest).ProtoReflect().Descriptor() },
				ResponseDefn: func() protoreflect.MessageDescriptor { return new(GCMResponse).ProtoReflect().Descriptor() },
				Help:         "GCM sends a message in a GC.",
				ServerHandler: func(x interface{}, ctx context.Context, request, response proto.Message) error {
					return x.(ChatServiceServer).GCM(ctx, request.(*GCMRequest), response.(*GCMResponse))
				},
				ClientHandler: func(conn ClientConn, ctx context.Context, request, response proto.Message) error {
					method := "ChatService.GCM"
					return conn.Request(ctx, method, request, response)
				},
			},
			"GCMStream": {
				IsStreaming:  true,
				NewRequest:   func() proto.Message { return new(GCMStreamRequest) },
				NewResponse:  func() proto.Message { return new(GCReceivedMsg) },
				RequestDefn:  func() protoreflect.MessageDescriptor { return new(GCMStreamRequest).ProtoReflect().Descriptor() },
				ResponseDefn: func() protoreflect.MessageDescriptor { return new(GCReceivedMsg).ProtoReflect().Descriptor() },
				Help:         "GCMStream returns a stream that gets GC messages received by the client.",
				ServerStreamHandler: func(x interface{}, ctx context.Context, request proto.Message, stream ServerStream) error {
					return x.(ChatServiceServer).GCMStream(ctx, request.(*GCMStreamRequest), streamerImpl[*GCReceivedMsg]{s: stream})
				},
				ClientStreamHandler: func(conn ClientConn, ctx context.Context, request proto.Message) (ClientStream, error) {
					method := "ChatService.GCMStream"
					return conn.Stream(ctx, method, request)
				},
			},
			"AckReceivedGCM": {
				IsStreaming:  false,
				NewRequest:   func() proto.Message { return new(AckRequest) },
				NewResponse:  func() proto.Message { return new(AckResponse) },
				RequestDefn:  func() protoreflect.MessageDescriptor { return new(AckRequest).ProtoReflect().Descriptor() },
				ResponseDefn: func() protoreflect.MessageDescriptor { return new(AckResponse).ProtoReflect().Descriptor() },
				Help:         "AckReceivedGCM acks to the server that GCMs up to a sequence ID have been processed.",
				ServerHandler: func(x interface{}, ctx context.Context, request, response proto.Message) error {
					return x.(ChatServiceServer).AckReceivedGCM(ctx, request.(*AckRequest), response.(*AckResponse))
				},
				ClientHandler: func(conn ClientConn, ctx context.Context, request, response proto.Message) error {
					method := "ChatService.AckReceivedGCM"
					return conn.Request(ctx, method, request, response)
				},
			},
		},
	}
}

// PostsServiceClient is the client API for PostsService service.
type PostsServiceClient interface {
	// SubscribeToPosts makes the local client subscribe to a remote user's posts.
	SubscribeToPosts(ctx context.Context, in *SubscribeToPostsRequest, out *SubscribeToPostsResponse) error
	// UnsubscribeToPosts makes the local client unsubscribe from a remote user's posts.
	UnsubscribeToPosts(ctx context.Context, in *UnsubscribeToPostsRequest, out *UnsubscribeToPostsResponse) error
	// PostsStream creates a stream that receives updates about posts received
	// from remote users the local client is subscribed to.
	PostsStream(ctx context.Context, in *PostsStreamRequest) (PostsService_PostsStreamClient, error)
	// AckReceivedPost acknowledges posts received up to a given sequence_id have
	// been processed.
	AckReceivedPost(ctx context.Context, in *AckRequest, out *AckResponse) error
	// PostsStatusStream creates a stream that receives updates about post status
	// events (comments, replies, etc).
	PostsStatusStream(ctx context.Context, in *PostsStatusStreamRequest) (PostsService_PostsStatusStreamClient, error)
	// AckReceivedPostStatus acknowledges post status received up to a given
	// sequence_id have been processed.
	AckReceivedPostStatus(ctx context.Context, in *AckRequest, out *AckResponse) error
}

type client_PostsService struct {
	c    ClientConn
	defn ServiceDefn
}

func (c *client_PostsService) SubscribeToPosts(ctx context.Context, in *SubscribeToPostsRequest, out *SubscribeToPostsResponse) error {
	const method = "SubscribeToPosts"
	return c.defn.Methods[method].ClientHandler(c.c, ctx, in, out)
}

func (c *client_PostsService) UnsubscribeToPosts(ctx context.Context, in *UnsubscribeToPostsRequest, out *UnsubscribeToPostsResponse) error {
	const method = "UnsubscribeToPosts"
	return c.defn.Methods[method].ClientHandler(c.c, ctx, in, out)
}

type PostsService_PostsStreamClient interface {
	Recv(*ReceivedPost) error
}

func (c *client_PostsService) PostsStream(ctx context.Context, in *PostsStreamRequest) (PostsService_PostsStreamClient, error) {
	const method = "PostsStream"
	inner, err := c.defn.Methods[method].ClientStreamHandler(c.c, ctx, in)
	if err != nil {
		return nil, err
	}
	return streamerImpl[*ReceivedPost]{c: inner}, nil
}

func (c *client_PostsService) AckReceivedPost(ctx context.Context, in *AckRequest, out *AckResponse) error {
	const method = "AckReceivedPost"
	return c.defn.Methods[method].ClientHandler(c.c, ctx, in, out)
}

type PostsService_PostsStatusStreamClient interface {
	Recv(*ReceivedPostStatus) error
}

func (c *client_PostsService) PostsStatusStream(ctx context.Context, in *PostsStatusStreamRequest) (PostsService_PostsStatusStreamClient, error) {
	const method = "PostsStatusStream"
	inner, err := c.defn.Methods[method].ClientStreamHandler(c.c, ctx, in)
	if err != nil {
		return nil, err
	}
	return streamerImpl[*ReceivedPostStatus]{c: inner}, nil
}

func (c *client_PostsService) AckReceivedPostStatus(ctx context.Context, in *AckRequest, out *AckResponse) error {
	const method = "AckReceivedPostStatus"
	return c.defn.Methods[method].ClientHandler(c.c, ctx, in, out)
}

func NewPostsServiceClient(c ClientConn) PostsServiceClient {
	return &client_PostsService{c: c, defn: PostsServiceDefn()}
}

// PostsServiceServer is the server API for PostsService service.
type PostsServiceServer interface {
	// SubscribeToPosts makes the local client subscribe to a remote user's posts.
	SubscribeToPosts(context.Context, *SubscribeToPostsRequest, *SubscribeToPostsResponse) error
	// UnsubscribeToPosts makes the local client unsubscribe from a remote user's posts.
	UnsubscribeToPosts(context.Context, *UnsubscribeToPostsRequest, *UnsubscribeToPostsResponse) error
	// PostsStream creates a stream that receives updates about posts received
	// from remote users the local client is subscribed to.
	PostsStream(context.Context, *PostsStreamRequest, PostsService_PostsStreamServer) error
	// AckReceivedPost acknowledges posts received up to a given sequence_id have
	// been processed.
	AckReceivedPost(context.Context, *AckRequest, *AckResponse) error
	// PostsStatusStream creates a stream that receives updates about post status
	// events (comments, replies, etc).
	PostsStatusStream(context.Context, *PostsStatusStreamRequest, PostsService_PostsStatusStreamServer) error
	// AckReceivedPostStatus acknowledges post status received up to a given
	// sequence_id have been processed.
	AckReceivedPostStatus(context.Context, *AckRequest, *AckResponse) error
}

type PostsService_PostsStreamServer interface {
	Send(m *ReceivedPost) error
}

type PostsService_PostsStatusStreamServer interface {
	Send(m *ReceivedPostStatus) error
}

func PostsServiceDefn() ServiceDefn {
	return ServiceDefn{
		Name: "PostsService",
		Methods: map[string]MethodDefn{
			"SubscribeToPosts": {
				IsStreaming: false,
				NewRequest:  func() proto.Message { return new(SubscribeToPostsRequest) },
				NewResponse: func() proto.Message { return new(SubscribeToPostsResponse) },
				RequestDefn: func() protoreflect.MessageDescriptor { return new(SubscribeToPostsRequest).ProtoReflect().Descriptor() },
				ResponseDefn: func() protoreflect.MessageDescriptor {
					return new(SubscribeToPostsResponse).ProtoReflect().Descriptor()
				},
				Help: "SubscribeToPosts makes the local client subscribe to a remote user's posts.",
				ServerHandler: func(x interface{}, ctx context.Context, request, response proto.Message) error {
					return x.(PostsServiceServer).SubscribeToPosts(ctx, request.(*SubscribeToPostsRequest), response.(*SubscribeToPostsResponse))
				},
				ClientHandler: func(conn ClientConn, ctx context.Context, request, response proto.Message) error {
					method := "PostsService.SubscribeToPosts"
					return conn.Request(ctx, method, request, response)
				},
			},
			"UnsubscribeToPosts": {
				IsStreaming: false,
				NewRequest:  func() proto.Message { return new(UnsubscribeToPostsRequest) },
				NewResponse: func() proto.Message { return new(UnsubscribeToPostsResponse) },
				RequestDefn: func() protoreflect.MessageDescriptor {
					return new(UnsubscribeToPostsRequest).ProtoReflect().Descriptor()
				},
				ResponseDefn: func() protoreflect.MessageDescriptor {
					return new(UnsubscribeToPostsResponse).ProtoReflect().Descriptor()
				},
				Help: "UnsubscribeToPosts makes the local client unsubscribe from a remote user's posts.",
				ServerHandler: func(x interface{}, ctx context.Context, request, response proto.Message) error {
					return x.(PostsServiceServer).UnsubscribeToPosts(ctx, request.(*UnsubscribeToPostsRequest), response.(*UnsubscribeToPostsResponse))
				},
				ClientHandler: func(conn ClientConn, ctx context.Context, request, response proto.Message) error {
					method := "PostsService.UnsubscribeToPosts"
					return conn.Request(ctx, method, request, response)
				},
			},
			"PostsStream": {
				IsStreaming:  true,
				NewRequest:   func() proto.Message { return new(PostsStreamRequest) },
				NewResponse:  func() proto.Message { return new(ReceivedPost) },
				RequestDefn:  func() protoreflect.MessageDescriptor { return new(PostsStreamRequest).ProtoReflect().Descriptor() },
				ResponseDefn: func() protoreflect.MessageDescriptor { return new(ReceivedPost).ProtoReflect().Descriptor() },
				Help:         "PostsStream creates a stream that receives updates about posts received from remote users the local client is subscribed to.",
				ServerStreamHandler: func(x interface{}, ctx context.Context, request proto.Message, stream ServerStream) error {
					return x.(PostsServiceServer).PostsStream(ctx, request.(*PostsStreamRequest), streamerImpl[*ReceivedPost]{s: stream})
				},
				ClientStreamHandler: func(conn ClientConn, ctx context.Context, request proto.Message) (ClientStream, error) {
					method := "PostsService.PostsStream"
					return conn.Stream(ctx, method, request)
				},
			},
			"AckReceivedPost": {
				IsStreaming:  false,
				NewRequest:   func() proto.Message { return new(AckRequest) },
				NewResponse:  func() proto.Message { return new(AckResponse) },
				RequestDefn:  func() protoreflect.MessageDescriptor { return new(AckRequest).ProtoReflect().Descriptor() },
				ResponseDefn: func() protoreflect.MessageDescriptor { return new(AckResponse).ProtoReflect().Descriptor() },
				Help:         "AckReceivedPost acknowledges posts received up to a given sequence_id have been processed.",
				ServerHandler: func(x interface{}, ctx context.Context, request, response proto.Message) error {
					return x.(PostsServiceServer).AckReceivedPost(ctx, request.(*AckRequest), response.(*AckResponse))
				},
				ClientHandler: func(conn ClientConn, ctx context.Context, request, response proto.Message) error {
					method := "PostsService.AckReceivedPost"
					return conn.Request(ctx, method, request, response)
				},
			},
			"PostsStatusStream": {
				IsStreaming: true,
				NewRequest:  func() proto.Message { return new(PostsStatusStreamRequest) },
				NewResponse: func() proto.Message { return new(ReceivedPostStatus) },
				RequestDefn: func() protoreflect.MessageDescriptor {
					return new(PostsStatusStreamRequest).ProtoReflect().Descriptor()
				},
				ResponseDefn: func() protoreflect.MessageDescriptor { return new(ReceivedPostStatus).ProtoReflect().Descriptor() },
				Help:         "PostsStatusStream creates a stream that receives updates about post status events (comments, replies, etc).",
				ServerStreamHandler: func(x interface{}, ctx context.Context, request proto.Message, stream ServerStream) error {
					return x.(PostsServiceServer).PostsStatusStream(ctx, request.(*PostsStatusStreamRequest), streamerImpl[*ReceivedPostStatus]{s: stream})
				},
				ClientStreamHandler: func(conn ClientConn, ctx context.Context, request proto.Message) (ClientStream, error) {
					method := "PostsService.PostsStatusStream"
					return conn.Stream(ctx, method, request)
				},
			},
			"AckReceivedPostStatus": {
				IsStreaming:  false,
				NewRequest:   func() proto.Message { return new(AckRequest) },
				NewResponse:  func() proto.Message { return new(AckResponse) },
				RequestDefn:  func() protoreflect.MessageDescriptor { return new(AckRequest).ProtoReflect().Descriptor() },
				ResponseDefn: func() protoreflect.MessageDescriptor { return new(AckResponse).ProtoReflect().Descriptor() },
				Help:         "AckReceivedPostStatus acknowledges post status received up to a given sequence_id have been processed.",
				ServerHandler: func(x interface{}, ctx context.Context, request, response proto.Message) error {
					return x.(PostsServiceServer).AckReceivedPostStatus(ctx, request.(*AckRequest), response.(*AckResponse))
				},
				ClientHandler: func(conn ClientConn, ctx context.Context, request, response proto.Message) error {
					method := "PostsService.AckReceivedPostStatus"
					return conn.Request(ctx, method, request, response)
				},
			},
		},
	}
}

// PaymentsServiceClient is the client API for PaymentsService service.
type PaymentsServiceClient interface {
	// TipUser attempts to send a tip to a user. The user must be or come online
	// for this to complete.
	TipUser(ctx context.Context, in *TipUserRequest, out *TipUserResponse) error
}

type client_PaymentsService struct {
	c    ClientConn
	defn ServiceDefn
}

func (c *client_PaymentsService) TipUser(ctx context.Context, in *TipUserRequest, out *TipUserResponse) error {
	const method = "TipUser"
	return c.defn.Methods[method].ClientHandler(c.c, ctx, in, out)
}

func NewPaymentsServiceClient(c ClientConn) PaymentsServiceClient {
	return &client_PaymentsService{c: c, defn: PaymentsServiceDefn()}
}

// PaymentsServiceServer is the server API for PaymentsService service.
type PaymentsServiceServer interface {
	// TipUser attempts to send a tip to a user. The user must be or come online
	// for this to complete.
	TipUser(context.Context, *TipUserRequest, *TipUserResponse) error
}

func PaymentsServiceDefn() ServiceDefn {
	return ServiceDefn{
		Name: "PaymentsService",
		Methods: map[string]MethodDefn{
			"TipUser": {
				IsStreaming:  false,
				NewRequest:   func() proto.Message { return new(TipUserRequest) },
				NewResponse:  func() proto.Message { return new(TipUserResponse) },
				RequestDefn:  func() protoreflect.MessageDescriptor { return new(TipUserRequest).ProtoReflect().Descriptor() },
				ResponseDefn: func() protoreflect.MessageDescriptor { return new(TipUserResponse).ProtoReflect().Descriptor() },
				Help:         "TipUser attempts to send a tip to a user. The user must be or come online for this to complete.",
				ServerHandler: func(x interface{}, ctx context.Context, request, response proto.Message) error {
					return x.(PaymentsServiceServer).TipUser(ctx, request.(*TipUserRequest), response.(*TipUserResponse))
				},
				ClientHandler: func(conn ClientConn, ctx context.Context, request, response proto.Message) error {
					method := "PaymentsService.TipUser"
					return conn.Request(ctx, method, request, response)
				},
			},
		},
	}
}

var help_messages = map[string]map[string]string{
	"VersionRequest": {
		"@": "",
	},
	"VersionResponse": {
		"@":           "VersionResponse is the information about the running RPC server.",
		"app_version": "app_version is the version of the application.",
		"go_runtime":  "go_runtime is the Go version the server was compiled with.",
		"app_name":    "app_name is the name of the underlying app running the server.",
	},
	"KeepaliveStreamRequest": {
		"@": "KeepaliveStreamRequest is the request for a new keepalive stream.",
		"interval": "interval is how often to send the keepalive (in milliseconds).\n" +
			"A minimum of 1 second is imposed, regardless of the value passed as interval.",
	},
	"KeepaliveEvent": {
		"@":         "KeepaliveEvent is a single keepalive event.",
		"timestamp": "timestamp is the unix timestamp on the server, with second precision.",
	},
	"AckRequest": {
		"@":           "AckRequest is a request to ack that a type of message up to a sequence ID has been processed.",
		"sequence_id": "sequence_id is the ID up to which messages have been processed.",
	},
	"AckResponse": {
		"@": "AckResponse is the response to an ack request.",
	},
	"PMRequest": {
		"@":    "PMRequest is a request to send a new private message.",
		"user": "user is either the nick, alias or an hex-encoded user ID of the destination.",
		"msg":  "msg is the message to be sent.",
	},
	"PMResponse": {
		"@": "PMResponse is the response of the client for a new message.",
	},
	"PMStreamRequest": {
		"@":            "PMStreamRequest is the request for a new private message reception stream.",
		"unacked_from": "unacked_from specifies to the server the sequence_id of the last processed PM. PMs received by the server that have a higher sequence_id will be streamed back to the client.",
	},
	"ReceivedPM": {
		"@":            "ReceivedPM is a private message received by the client.",
		"uid":          "uid is the source user ID in raw format.",
		"nick":         "nick is the source's nick or alias.",
		"msg":          "msg is the received message payload.",
		"timestamp_ms": "timestamp_ms is the timestamp from unix epoch with millisecond precision.",
		"sequence_id":  "sequence_id is an opaque sequential ID.",
	},
	"GCMRequest": {
		"@":   "GCMRequest is a request to send a GC message.",
		"gc":  "gc is either an hex-encoded GCID or a GC alias.",
		"msg": "msg is the text payload of the message.",
	},
	"GCMResponse": {
		"@": "GCMResponse is the response to sending a GC message.",
	},
	"GCMStreamRequest": {
		"@":            "GCMStreamRequest is a request to a stream of received GC messages.",
		"unacked_from": "unacked_from specifies to the server the sequence_id of the last processed GCM. GCMs received by the server that have a higher sequence_id will be streamed back to the client.",
	},
	"GCReceivedMsg": {
		"@":            "GCReceivedMsg is a GC message received from a remote user.",
		"uid":          "uid is the source user ID.",
		"nick":         "nick is the source user nick/alias.",
		"gc_alias":     "gc_alias is the local alias of the GC where the message was sent.",
		"msg":          "msg is the received message.",
		"timestamp_ms": "timestamp_ms is the server timestamp of the message with millisecond precision.",
		"sequence_id":  "sequence_id is an opaque sequential ID.",
	},
	"SubscribeToPostsRequest": {
		"@":    "SubscribeToPostsRequest is a request to subscribe to a remote user's posts.",
		"user": "user is the nick or hex-encoded ID of the user to subscribe to.",
	},
	"SubscribeToPostsResponse": {
		"@": "SubscribeToPostsResponse is the response to subscribing to a remote user's posts.",
	},
	"UnsubscribeToPostsRequest": {
		"@":    "UnsubscribeToPostsRequest is a request to unsubscribe from a remote user's posts.",
		"user": "user is the nick or hex-encoded ID of the user to unsubscribe from.",
	},
	"UnsubscribeToPostsResponse": {
		"@": "UnsubscribeToPostsResponse is the response to an unsubscribe request.",
	},
	"PostSummary": {
		"@":              "PostSummary is the summary information about a post.",
		"id":             "id is the post ID (hash of the post metadata).",
		"from":           "from is the id of the relayer of the post (who the local client received the post from).",
		"author_id":      "author_id is the id of the author of the post.",
		"author_nick":    "author_nick is the reported nick of the author of the post.",
		"date":           "date is the unix timestamp of the post.",
		"last_status_ts": "last_status_ts is the timestamp of the last recorded status update of the post.",
		"title":          "title is either the included or suggested title of the post.",
	},
	"PostsStreamRequest": {
		"@":            "PostsStreamRequest is the request to establish a stream of received post events.",
		"unacked_from": "unacked_from specifies to the server the sequence_id of the last processed post. Posts received by the server that have a higher sequence_id will be streamed back to the client.",
	},
	"ReceivedPost": {
		"@":           "ReceivedPost is a post received by the local client.",
		"sequence_id": "sequence_id is an opaque sequential ID.",
		"relayer_id":  "relayer_id is the id of the user we received the post from (may not be the same as the author).",
		"summary":     "summary is the summary information about the post.",
		"post":        "post is the full post data.",
	},
	"PostsStatusStreamRequest": {
		"@":            "PostsStatusStreamRequest is a request to establish a stream that receives post status updates received by the local client.",
		"unacked_from": "unacked_from specifies to the server the sequence_id of the last processed Post Status. Post Status received by the server that have a higher sequence_id will be streamed back to the client.",
	},
	"ReceivedPostStatus": {
		"@":           "ReceivedPostStatus is a post status update received by the local client.",
		"sequence_id": "sequence_id is an opaque sequential ID.",
		"relayer_id":  "relayer_id is the id of the sender of the client that sent the update.",
		"post_id":     "post_id is the id of the corresponding post.",
		"status_from": "status_from is the original author of the status.",
		"status":      "status is the full status data.",
	},
	"TipUserRequest": {
		"@":          "TipUserRequest is a request to tip a remote user.",
		"user":       "user is the remote user nick or hex-encoded ID.",
		"dcr_amount": "dcr_amount is the DCR amount to send as tip.",
	},
	"TipUserResponse": {
		"@": "TipUserResponse is the response to a tip user request.",
	},
	"RMPrivateMessage": {
		"@":       "RMPrivateMessage is the network-level routed private message.",
		"message": "message is the private message payload.",
		"mode":    "mode is the message mode.",
	},
	"RMGroupMessage": {
		"@":          "RMGroupMessage is the network-level routed group message.",
		"id":         "id is the group chat id where the message was sent.",
		"generation": "generation is the internal generation of the group chat metadata when the sender sent this message.",
		"message":    "message is the textual content.",
		"mode":       "mode is the mode of the message.",
	},
	"PostMetadata": {
		"@":          "PostMetadata is the network-level post data.",
		"version":    "version defines the available fields within attributes.",
		"attributes": "attributes defines the available post attributes.",
	},
	"PostMetadataStatus": {
		"@":          "PostMetadataStatus is the network-level post status update data.",
		"version":    "version defines the available fields within attributes.",
		"from":       "from is the UID of the original status creator.",
		"link":       "link is the ID of the post.",
		"attributes": "attributes is the list of post update attributes.",
	},
}
