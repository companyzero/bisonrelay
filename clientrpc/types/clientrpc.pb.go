// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.19.0
// source: clientrpc.proto

package types

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type MessageMode int32

const (
	// MESSAGE_MODE_NORMAL is the standard mode for messages.
	MessageMode_MESSAGE_MODE_NORMAL MessageMode = 0
	// MESSAGE_MODE_ME are messages sent in the passive voice (i.e. with /me).
	MessageMode_MESSAGE_MODE_ME MessageMode = 1
)

// Enum value maps for MessageMode.
var (
	MessageMode_name = map[int32]string{
		0: "MESSAGE_MODE_NORMAL",
		1: "MESSAGE_MODE_ME",
	}
	MessageMode_value = map[string]int32{
		"MESSAGE_MODE_NORMAL": 0,
		"MESSAGE_MODE_ME":     1,
	}
)

func (x MessageMode) Enum() *MessageMode {
	p := new(MessageMode)
	*p = x
	return p
}

func (x MessageMode) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (MessageMode) Descriptor() protoreflect.EnumDescriptor {
	return file_clientrpc_proto_enumTypes[0].Descriptor()
}

func (MessageMode) Type() protoreflect.EnumType {
	return &file_clientrpc_proto_enumTypes[0]
}

func (x MessageMode) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use MessageMode.Descriptor instead.
func (MessageMode) EnumDescriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{0}
}

type VersionRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *VersionRequest) Reset() {
	*x = VersionRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VersionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VersionRequest) ProtoMessage() {}

func (x *VersionRequest) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VersionRequest.ProtoReflect.Descriptor instead.
func (*VersionRequest) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{0}
}

// VersionResponse is the information about the running RPC server.
type VersionResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// app_version is the version of the application.
	AppVersion string `protobuf:"bytes,1,opt,name=app_version,json=appVersion,proto3" json:"app_version,omitempty"`
	// go_runtime is the Go version the server was compiled with.
	GoRuntime string `protobuf:"bytes,2,opt,name=go_runtime,json=goRuntime,proto3" json:"go_runtime,omitempty"`
	// app_name is the name of the underlying app running the server.
	AppName string `protobuf:"bytes,3,opt,name=app_name,json=appName,proto3" json:"app_name,omitempty"`
}

func (x *VersionResponse) Reset() {
	*x = VersionResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VersionResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VersionResponse) ProtoMessage() {}

func (x *VersionResponse) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VersionResponse.ProtoReflect.Descriptor instead.
func (*VersionResponse) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{1}
}

func (x *VersionResponse) GetAppVersion() string {
	if x != nil {
		return x.AppVersion
	}
	return ""
}

func (x *VersionResponse) GetGoRuntime() string {
	if x != nil {
		return x.GoRuntime
	}
	return ""
}

func (x *VersionResponse) GetAppName() string {
	if x != nil {
		return x.AppName
	}
	return ""
}

// KeepaliveStreamRequest is the request for a new keepalive stream.
type KeepaliveStreamRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// interval is how often to send the keepalive (in milliseconds).
	//
	// A minimum of 1 second is imposed, regardless of the value passed as
	// interval.
	Interval int64 `protobuf:"varint,1,opt,name=interval,proto3" json:"interval,omitempty"`
}

func (x *KeepaliveStreamRequest) Reset() {
	*x = KeepaliveStreamRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *KeepaliveStreamRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*KeepaliveStreamRequest) ProtoMessage() {}

func (x *KeepaliveStreamRequest) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use KeepaliveStreamRequest.ProtoReflect.Descriptor instead.
func (*KeepaliveStreamRequest) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{2}
}

func (x *KeepaliveStreamRequest) GetInterval() int64 {
	if x != nil {
		return x.Interval
	}
	return 0
}

// KeepaliveEvent is a single keepalive event.
type KeepaliveEvent struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// timestamp is the unix timestamp on the server, with second precision.
	Timestamp int64 `protobuf:"varint,1,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
}

func (x *KeepaliveEvent) Reset() {
	*x = KeepaliveEvent{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *KeepaliveEvent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*KeepaliveEvent) ProtoMessage() {}

func (x *KeepaliveEvent) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use KeepaliveEvent.ProtoReflect.Descriptor instead.
func (*KeepaliveEvent) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{3}
}

func (x *KeepaliveEvent) GetTimestamp() int64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}

// PMRequest is a request to send a new private message.
type PMRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// user is either the nick, alias or an hex-encoded user ID of the destination.
	User string `protobuf:"bytes,1,opt,name=user,proto3" json:"user,omitempty"`
	// msg is the message to be sent.
	Msg *RMPrivateMessage `protobuf:"bytes,2,opt,name=msg,proto3" json:"msg,omitempty"`
}

func (x *PMRequest) Reset() {
	*x = PMRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PMRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PMRequest) ProtoMessage() {}

func (x *PMRequest) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PMRequest.ProtoReflect.Descriptor instead.
func (*PMRequest) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{4}
}

func (x *PMRequest) GetUser() string {
	if x != nil {
		return x.User
	}
	return ""
}

func (x *PMRequest) GetMsg() *RMPrivateMessage {
	if x != nil {
		return x.Msg
	}
	return nil
}

// PMResponse is the response of the client for a new message.
type PMResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *PMResponse) Reset() {
	*x = PMResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PMResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PMResponse) ProtoMessage() {}

func (x *PMResponse) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PMResponse.ProtoReflect.Descriptor instead.
func (*PMResponse) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{5}
}

// PMStreamRequest is the request for a new private message reception stream.
type PMStreamRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *PMStreamRequest) Reset() {
	*x = PMStreamRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PMStreamRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PMStreamRequest) ProtoMessage() {}

func (x *PMStreamRequest) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PMStreamRequest.ProtoReflect.Descriptor instead.
func (*PMStreamRequest) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{6}
}

// ReceivedPM is a private message received by the client.
type ReceivedPM struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// uid is the source user ID in raw format.
	Uid []byte `protobuf:"bytes,1,opt,name=uid,proto3" json:"uid,omitempty"`
	// nick is the source's nick or alias.
	Nick string `protobuf:"bytes,2,opt,name=nick,proto3" json:"nick,omitempty"`
	// msg is the received message payload.
	Msg *RMPrivateMessage `protobuf:"bytes,3,opt,name=msg,proto3" json:"msg,omitempty"`
	// timestamp_ms is the timestamp from unix epoch with millisecond precision.
	TimestampMs int64 `protobuf:"varint,4,opt,name=timestamp_ms,json=timestampMs,proto3" json:"timestamp_ms,omitempty"`
}

func (x *ReceivedPM) Reset() {
	*x = ReceivedPM{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ReceivedPM) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ReceivedPM) ProtoMessage() {}

func (x *ReceivedPM) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ReceivedPM.ProtoReflect.Descriptor instead.
func (*ReceivedPM) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{7}
}

func (x *ReceivedPM) GetUid() []byte {
	if x != nil {
		return x.Uid
	}
	return nil
}

func (x *ReceivedPM) GetNick() string {
	if x != nil {
		return x.Nick
	}
	return ""
}

func (x *ReceivedPM) GetMsg() *RMPrivateMessage {
	if x != nil {
		return x.Msg
	}
	return nil
}

func (x *ReceivedPM) GetTimestampMs() int64 {
	if x != nil {
		return x.TimestampMs
	}
	return 0
}

// GCMRequest is a request to send a GC message.
type GCMRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// gc is either an hex-encoded GCID or a GC alias.
	Gc string `protobuf:"bytes,1,opt,name=gc,proto3" json:"gc,omitempty"`
	// msg is the text payload of the message.
	Msg string `protobuf:"bytes,2,opt,name=msg,proto3" json:"msg,omitempty"`
}

func (x *GCMRequest) Reset() {
	*x = GCMRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GCMRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GCMRequest) ProtoMessage() {}

func (x *GCMRequest) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GCMRequest.ProtoReflect.Descriptor instead.
func (*GCMRequest) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{8}
}

func (x *GCMRequest) GetGc() string {
	if x != nil {
		return x.Gc
	}
	return ""
}

func (x *GCMRequest) GetMsg() string {
	if x != nil {
		return x.Msg
	}
	return ""
}

// GCMResponse is the response to sending a GC message.
type GCMResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GCMResponse) Reset() {
	*x = GCMResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GCMResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GCMResponse) ProtoMessage() {}

func (x *GCMResponse) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GCMResponse.ProtoReflect.Descriptor instead.
func (*GCMResponse) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{9}
}

// GCMStreamRequest is a request to a stream of received GC messages.
type GCMStreamRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GCMStreamRequest) Reset() {
	*x = GCMStreamRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GCMStreamRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GCMStreamRequest) ProtoMessage() {}

func (x *GCMStreamRequest) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GCMStreamRequest.ProtoReflect.Descriptor instead.
func (*GCMStreamRequest) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{10}
}

// GCReceivedMsg is a GC message received from a remote user.
type GCReceivedMsg struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// uid is the source user ID.
	Uid []byte `protobuf:"bytes,1,opt,name=uid,proto3" json:"uid,omitempty"`
	// nick is the source user nick/alias.
	Nick string `protobuf:"bytes,2,opt,name=nick,proto3" json:"nick,omitempty"`
	// gc_alias is the local alias of the GC where the message was sent.
	GcAlias string `protobuf:"bytes,3,opt,name=gc_alias,json=gcAlias,proto3" json:"gc_alias,omitempty"`
	// msg is the received message.
	Msg *RMGroupMessage `protobuf:"bytes,4,opt,name=msg,proto3" json:"msg,omitempty"`
	// timestamp_ms is the server timestamp of the message with millisecond precision.
	TimestampMs int64 `protobuf:"varint,5,opt,name=timestamp_ms,json=timestampMs,proto3" json:"timestamp_ms,omitempty"`
}

func (x *GCReceivedMsg) Reset() {
	*x = GCReceivedMsg{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[11]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GCReceivedMsg) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GCReceivedMsg) ProtoMessage() {}

func (x *GCReceivedMsg) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[11]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GCReceivedMsg.ProtoReflect.Descriptor instead.
func (*GCReceivedMsg) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{11}
}

func (x *GCReceivedMsg) GetUid() []byte {
	if x != nil {
		return x.Uid
	}
	return nil
}

func (x *GCReceivedMsg) GetNick() string {
	if x != nil {
		return x.Nick
	}
	return ""
}

func (x *GCReceivedMsg) GetGcAlias() string {
	if x != nil {
		return x.GcAlias
	}
	return ""
}

func (x *GCReceivedMsg) GetMsg() *RMGroupMessage {
	if x != nil {
		return x.Msg
	}
	return nil
}

func (x *GCReceivedMsg) GetTimestampMs() int64 {
	if x != nil {
		return x.TimestampMs
	}
	return 0
}

// RMPrivateMessage is the network-level routed private message.
type RMPrivateMessage struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// message is the private message payload.
	Message string `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	// mode is the message mode.
	Mode MessageMode `protobuf:"varint,2,opt,name=mode,proto3,enum=MessageMode" json:"mode,omitempty"`
}

func (x *RMPrivateMessage) Reset() {
	*x = RMPrivateMessage{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[12]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RMPrivateMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RMPrivateMessage) ProtoMessage() {}

func (x *RMPrivateMessage) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[12]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RMPrivateMessage.ProtoReflect.Descriptor instead.
func (*RMPrivateMessage) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{12}
}

func (x *RMPrivateMessage) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *RMPrivateMessage) GetMode() MessageMode {
	if x != nil {
		return x.Mode
	}
	return MessageMode_MESSAGE_MODE_NORMAL
}

// RMGroupMessage is the network-level routed group message.
type RMGroupMessage struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// id is the group chat id where the message was sent.
	Id []byte `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// generation is the internal generation of the group chat metadata when the
	// sender sent this message.
	Generation uint64 `protobuf:"varint,2,opt,name=generation,proto3" json:"generation,omitempty"`
	// message is the textual content.
	Message string `protobuf:"bytes,3,opt,name=message,proto3" json:"message,omitempty"`
	// mode is the mode of the message.
	Mode MessageMode `protobuf:"varint,4,opt,name=mode,proto3,enum=MessageMode" json:"mode,omitempty"`
}

func (x *RMGroupMessage) Reset() {
	*x = RMGroupMessage{}
	if protoimpl.UnsafeEnabled {
		mi := &file_clientrpc_proto_msgTypes[13]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RMGroupMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RMGroupMessage) ProtoMessage() {}

func (x *RMGroupMessage) ProtoReflect() protoreflect.Message {
	mi := &file_clientrpc_proto_msgTypes[13]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RMGroupMessage.ProtoReflect.Descriptor instead.
func (*RMGroupMessage) Descriptor() ([]byte, []int) {
	return file_clientrpc_proto_rawDescGZIP(), []int{13}
}

func (x *RMGroupMessage) GetId() []byte {
	if x != nil {
		return x.Id
	}
	return nil
}

func (x *RMGroupMessage) GetGeneration() uint64 {
	if x != nil {
		return x.Generation
	}
	return 0
}

func (x *RMGroupMessage) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *RMGroupMessage) GetMode() MessageMode {
	if x != nil {
		return x.Mode
	}
	return MessageMode_MESSAGE_MODE_NORMAL
}

var File_clientrpc_proto protoreflect.FileDescriptor

var file_clientrpc_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x72, 0x70, 0x63, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0x10, 0x0a, 0x0e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x22, 0x6c, 0x0a, 0x0f, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x61, 0x70, 0x70, 0x5f, 0x76, 0x65,
	0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x61, 0x70, 0x70,
	0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x1d, 0x0a, 0x0a, 0x67, 0x6f, 0x5f, 0x72, 0x75,
	0x6e, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x67, 0x6f, 0x52,
	0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x61, 0x70, 0x70, 0x5f, 0x6e, 0x61,
	0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x70, 0x70, 0x4e, 0x61, 0x6d,
	0x65, 0x22, 0x34, 0x0a, 0x16, 0x4b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65, 0x53, 0x74,
	0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x22, 0x2e, 0x0a, 0x0e, 0x4b, 0x65, 0x65, 0x70, 0x61,
	0x6c, 0x69, 0x76, 0x65, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x74, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x74, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x22, 0x44, 0x0a, 0x09, 0x50, 0x4d, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x75, 0x73, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x75, 0x73, 0x65, 0x72, 0x12, 0x23, 0x0a, 0x03, 0x6d, 0x73, 0x67, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x52, 0x4d, 0x50, 0x72, 0x69, 0x76, 0x61, 0x74,
	0x65, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x03, 0x6d, 0x73, 0x67, 0x22, 0x0c, 0x0a,
	0x0a, 0x50, 0x4d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x11, 0x0a, 0x0f, 0x50,
	0x4d, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x7a,
	0x0a, 0x0a, 0x52, 0x65, 0x63, 0x65, 0x69, 0x76, 0x65, 0x64, 0x50, 0x4d, 0x12, 0x10, 0x0a, 0x03,
	0x75, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x03, 0x75, 0x69, 0x64, 0x12, 0x12,
	0x0a, 0x04, 0x6e, 0x69, 0x63, 0x6b, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x69,
	0x63, 0x6b, 0x12, 0x23, 0x0a, 0x03, 0x6d, 0x73, 0x67, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x11, 0x2e, 0x52, 0x4d, 0x50, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x4d, 0x65, 0x73, 0x73, 0x61,
	0x67, 0x65, 0x52, 0x03, 0x6d, 0x73, 0x67, 0x12, 0x21, 0x0a, 0x0c, 0x74, 0x69, 0x6d, 0x65, 0x73,
	0x74, 0x61, 0x6d, 0x70, 0x5f, 0x6d, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0b, 0x74,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x4d, 0x73, 0x22, 0x2e, 0x0a, 0x0a, 0x47, 0x43,
	0x4d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x67, 0x63, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x67, 0x63, 0x12, 0x10, 0x0a, 0x03, 0x6d, 0x73, 0x67, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6d, 0x73, 0x67, 0x22, 0x0d, 0x0a, 0x0b, 0x47, 0x43,
	0x4d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x12, 0x0a, 0x10, 0x47, 0x43, 0x4d,
	0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x96, 0x01,
	0x0a, 0x0d, 0x47, 0x43, 0x52, 0x65, 0x63, 0x65, 0x69, 0x76, 0x65, 0x64, 0x4d, 0x73, 0x67, 0x12,
	0x10, 0x0a, 0x03, 0x75, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x03, 0x75, 0x69,
	0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x69, 0x63, 0x6b, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x6e, 0x69, 0x63, 0x6b, 0x12, 0x19, 0x0a, 0x08, 0x67, 0x63, 0x5f, 0x61, 0x6c, 0x69, 0x61,
	0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x67, 0x63, 0x41, 0x6c, 0x69, 0x61, 0x73,
	0x12, 0x21, 0x0a, 0x03, 0x6d, 0x73, 0x67, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e,
	0x52, 0x4d, 0x47, 0x72, 0x6f, 0x75, 0x70, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x03,
	0x6d, 0x73, 0x67, 0x12, 0x21, 0x0a, 0x0c, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70,
	0x5f, 0x6d, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0b, 0x74, 0x69, 0x6d, 0x65, 0x73,
	0x74, 0x61, 0x6d, 0x70, 0x4d, 0x73, 0x22, 0x4e, 0x0a, 0x10, 0x52, 0x4d, 0x50, 0x72, 0x69, 0x76,
	0x61, 0x74, 0x65, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x12, 0x20, 0x0a, 0x04, 0x6d, 0x6f, 0x64, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x0c, 0x2e, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x4d, 0x6f, 0x64, 0x65,
	0x52, 0x04, 0x6d, 0x6f, 0x64, 0x65, 0x22, 0x7c, 0x0a, 0x0e, 0x52, 0x4d, 0x47, 0x72, 0x6f, 0x75,
	0x70, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0c, 0x52, 0x02, 0x69, 0x64, 0x12, 0x1e, 0x0a, 0x0a, 0x67, 0x65, 0x6e, 0x65,
	0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0a, 0x67, 0x65,
	0x6e, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61,
	0x67, 0x65, 0x12, 0x20, 0x0a, 0x04, 0x6d, 0x6f, 0x64, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x0c, 0x2e, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x4d, 0x6f, 0x64, 0x65, 0x52, 0x04,
	0x6d, 0x6f, 0x64, 0x65, 0x2a, 0x3b, 0x0a, 0x0b, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x4d,
	0x6f, 0x64, 0x65, 0x12, 0x17, 0x0a, 0x13, 0x4d, 0x45, 0x53, 0x53, 0x41, 0x47, 0x45, 0x5f, 0x4d,
	0x4f, 0x44, 0x45, 0x5f, 0x4e, 0x4f, 0x52, 0x4d, 0x41, 0x4c, 0x10, 0x00, 0x12, 0x13, 0x0a, 0x0f,
	0x4d, 0x45, 0x53, 0x53, 0x41, 0x47, 0x45, 0x5f, 0x4d, 0x4f, 0x44, 0x45, 0x5f, 0x4d, 0x45, 0x10,
	0x01, 0x32, 0x7d, 0x0a, 0x0e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x53, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x12, 0x2c, 0x0a, 0x07, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x0f,
	0x2e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x10, 0x2e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x3d, 0x0a, 0x0f, 0x4b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65, 0x53, 0x74,
	0x72, 0x65, 0x61, 0x6d, 0x12, 0x17, 0x2e, 0x4b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65,
	0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0f, 0x2e,
	0x4b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x30, 0x01,
	0x32, 0xad, 0x01, 0x0a, 0x0b, 0x43, 0x68, 0x61, 0x74, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x12, 0x1d, 0x0a, 0x02, 0x50, 0x4d, 0x12, 0x0a, 0x2e, 0x50, 0x4d, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x0b, 0x2e, 0x50, 0x4d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x2b, 0x0a, 0x08, 0x50, 0x4d, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x10, 0x2e, 0x50, 0x4d,
	0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0b, 0x2e,
	0x52, 0x65, 0x63, 0x65, 0x69, 0x76, 0x65, 0x64, 0x50, 0x4d, 0x30, 0x01, 0x12, 0x20, 0x0a, 0x03,
	0x47, 0x43, 0x4d, 0x12, 0x0b, 0x2e, 0x47, 0x43, 0x4d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x0c, 0x2e, 0x47, 0x43, 0x4d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x30,
	0x0a, 0x09, 0x47, 0x43, 0x4d, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x11, 0x2e, 0x47, 0x43,
	0x4d, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0e,
	0x2e, 0x47, 0x43, 0x52, 0x65, 0x63, 0x65, 0x69, 0x76, 0x65, 0x64, 0x4d, 0x73, 0x67, 0x30, 0x01,
	0x42, 0x34, 0x5a, 0x32, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x63,
	0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79, 0x7a, 0x65, 0x72, 0x6f, 0x2f, 0x62, 0x69, 0x73, 0x63, 0x6f,
	0x6e, 0x72, 0x65, 0x6c, 0x61, 0x79, 0x2f, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x72, 0x70, 0x63,
	0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_clientrpc_proto_rawDescOnce sync.Once
	file_clientrpc_proto_rawDescData = file_clientrpc_proto_rawDesc
)

func file_clientrpc_proto_rawDescGZIP() []byte {
	file_clientrpc_proto_rawDescOnce.Do(func() {
		file_clientrpc_proto_rawDescData = protoimpl.X.CompressGZIP(file_clientrpc_proto_rawDescData)
	})
	return file_clientrpc_proto_rawDescData
}

var file_clientrpc_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_clientrpc_proto_msgTypes = make([]protoimpl.MessageInfo, 14)
var file_clientrpc_proto_goTypes = []interface{}{
	(MessageMode)(0),               // 0: MessageMode
	(*VersionRequest)(nil),         // 1: VersionRequest
	(*VersionResponse)(nil),        // 2: VersionResponse
	(*KeepaliveStreamRequest)(nil), // 3: KeepaliveStreamRequest
	(*KeepaliveEvent)(nil),         // 4: KeepaliveEvent
	(*PMRequest)(nil),              // 5: PMRequest
	(*PMResponse)(nil),             // 6: PMResponse
	(*PMStreamRequest)(nil),        // 7: PMStreamRequest
	(*ReceivedPM)(nil),             // 8: ReceivedPM
	(*GCMRequest)(nil),             // 9: GCMRequest
	(*GCMResponse)(nil),            // 10: GCMResponse
	(*GCMStreamRequest)(nil),       // 11: GCMStreamRequest
	(*GCReceivedMsg)(nil),          // 12: GCReceivedMsg
	(*RMPrivateMessage)(nil),       // 13: RMPrivateMessage
	(*RMGroupMessage)(nil),         // 14: RMGroupMessage
}
var file_clientrpc_proto_depIdxs = []int32{
	13, // 0: PMRequest.msg:type_name -> RMPrivateMessage
	13, // 1: ReceivedPM.msg:type_name -> RMPrivateMessage
	14, // 2: GCReceivedMsg.msg:type_name -> RMGroupMessage
	0,  // 3: RMPrivateMessage.mode:type_name -> MessageMode
	0,  // 4: RMGroupMessage.mode:type_name -> MessageMode
	1,  // 5: VersionService.Version:input_type -> VersionRequest
	3,  // 6: VersionService.KeepaliveStream:input_type -> KeepaliveStreamRequest
	5,  // 7: ChatService.PM:input_type -> PMRequest
	7,  // 8: ChatService.PMStream:input_type -> PMStreamRequest
	9,  // 9: ChatService.GCM:input_type -> GCMRequest
	11, // 10: ChatService.GCMStream:input_type -> GCMStreamRequest
	2,  // 11: VersionService.Version:output_type -> VersionResponse
	4,  // 12: VersionService.KeepaliveStream:output_type -> KeepaliveEvent
	6,  // 13: ChatService.PM:output_type -> PMResponse
	8,  // 14: ChatService.PMStream:output_type -> ReceivedPM
	10, // 15: ChatService.GCM:output_type -> GCMResponse
	12, // 16: ChatService.GCMStream:output_type -> GCReceivedMsg
	11, // [11:17] is the sub-list for method output_type
	5,  // [5:11] is the sub-list for method input_type
	5,  // [5:5] is the sub-list for extension type_name
	5,  // [5:5] is the sub-list for extension extendee
	0,  // [0:5] is the sub-list for field type_name
}

func init() { file_clientrpc_proto_init() }
func file_clientrpc_proto_init() {
	if File_clientrpc_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_clientrpc_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*VersionRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_clientrpc_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*VersionResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_clientrpc_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*KeepaliveStreamRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_clientrpc_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*KeepaliveEvent); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_clientrpc_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PMRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_clientrpc_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PMResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_clientrpc_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PMStreamRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_clientrpc_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ReceivedPM); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_clientrpc_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GCMRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_clientrpc_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GCMResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_clientrpc_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GCMStreamRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_clientrpc_proto_msgTypes[11].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GCReceivedMsg); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_clientrpc_proto_msgTypes[12].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RMPrivateMessage); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_clientrpc_proto_msgTypes[13].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RMGroupMessage); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_clientrpc_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   14,
			NumExtensions: 0,
			NumServices:   2,
		},
		GoTypes:           file_clientrpc_proto_goTypes,
		DependencyIndexes: file_clientrpc_proto_depIdxs,
		EnumInfos:         file_clientrpc_proto_enumTypes,
		MessageInfos:      file_clientrpc_proto_msgTypes,
	}.Build()
	File_clientrpc_proto = out.File
	file_clientrpc_proto_rawDesc = nil
	file_clientrpc_proto_goTypes = nil
	file_clientrpc_proto_depIdxs = nil
}
