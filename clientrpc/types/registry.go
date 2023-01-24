package types

import (
	context "context"
	"fmt"
	"strings"
	sync "sync"

	proto "google.golang.org/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

// ClientConn abstracts the necessary functions from an underlying transport
// of messages.
type ClientConn interface {
	Request(ctx context.Context, method string, request, response proto.Message) error
	Stream(ctx context.Context, method string, request proto.Message) (ClientStream, error)
}

// ServerStream abstracts the necessary functions of a server-side stream.
type ServerStream interface {
	Send(proto.Message) error
}

// ClientStream abstracts the necessary functions of a client-side stream.
type ClientStream interface {
	Recv(proto.Message) error
}

// MethodDefn tracks information about an RPC method.
type MethodDefn struct {
	NewRequest   func() proto.Message
	NewResponse  func() proto.Message
	RequestDefn  func() protoreflect.MessageDescriptor
	ResponseDefn func() protoreflect.MessageDescriptor

	ServerHandler       func(x interface{}, ctx context.Context, request, response proto.Message) error
	ServerStreamHandler func(x interface{}, ctx context.Context, request proto.Message, stream ServerStream) error
	ClientHandler       func(conn ClientConn, ctx context.Context, request, response proto.Message) error
	ClientStreamHandler func(conn ClientConn, ctx context.Context, request proto.Message) (ClientStream, error)

	Help        string
	IsStreaming bool
}

// ServiceDefn tracks information about a service.
type ServiceDefn struct {
	Name    string
	Methods map[string]MethodDefn
}

// service is a single service implementation within the ServersMap.
type service struct {
	defn ServiceDefn
	impl interface{}
}

// ServersMap is a concurent-safe map for services.
type ServersMap struct {
	mtx sync.Mutex
	m   map[string]service
}

// Bind the service server to the specified name.
func (sm *ServersMap) Bind(name string, defn ServiceDefn, impl interface{}) {
	if sm == nil {
		return
	}

	sm.mtx.Lock()
	if sm.m == nil {
		sm.m = make(map[string]service, 1)
	}
	sm.m[name] = service{defn: defn, impl: impl}
	sm.mtx.Unlock()
}

// SvcForMethod return the service and method name given a Service.Method
// name.
func (sm *ServersMap) SvcForMethod(method string) (*ServiceDefn, interface{}, *MethodDefn, error) {
	if sm == nil {
		return nil, nil, nil, fmt.Errorf("unknown service")
	}

	splitMethod := strings.SplitN(method, ".", 2)
	if len(splitMethod) != 2 {
		return nil, nil, nil, fmt.Errorf("method is not Service.Method")
	}

	svcName, methodName := splitMethod[0], splitMethod[1]
	sm.mtx.Lock()
	svc, ok := sm.m[svcName]
	sm.mtx.Unlock()
	if !ok {
		return nil, nil, nil, fmt.Errorf("unknown service")
	}

	methodDefn, ok := svc.defn.Methods[methodName]
	if !ok {
		return nil, nil, nil, fmt.Errorf("unknown method")
	}

	return &svc.defn, svc.impl, &methodDefn, nil
}

// streamerImpl is a generic implementation of both a server and client stream
// for some proto.Message.
type streamerImpl[T proto.Message] struct {
	s ServerStream
	c ClientStream
}

func (s streamerImpl[T]) Send(m T) error {
	return s.s.Send(m)
}

func (s streamerImpl[T]) Recv(m T) error {
	return s.c.Recv(m)
}
