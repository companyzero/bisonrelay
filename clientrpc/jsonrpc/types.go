package jsonrpc

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const version string = "2.0"

var (
	unmarshalOpts = protojson.UnmarshalOptions{}
	marshalOpts   = protojson.MarshalOptions{
		UseProtoNames: false,
	}
)

// inboundMsg is a JSON-RPC message decoded from a reader. It supports both
// requests and responses.
type inboundMsg struct {
	Version string          `json:"jsonrpc,"`
	ID      interface{}     `json:"id,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
	Method  *string         `json:"method,omitempty"`
}

type protoPayload struct {
	payload proto.Message
}

func (pr protoPayload) MarshalJSON() ([]byte, error) {
	return marshalOpts.Marshal(pr.payload)
}

// outboundMsg is a message that is going to be written on a writer. It supports
// both requests and responses.
type outboundMsg struct {
	Version string        `json:"jsonrpc,"`
	ID      interface{}   `json:"id,omitempty"`
	Params  *protoPayload `json:"params,omitempty"`
	Result  *protoPayload `json:"result,omitempty"`
	Error   *Error        `json:"error,omitempty"`
	Method  *string       `json:"method,omitempty"`

	sentChan chan struct{}
}
