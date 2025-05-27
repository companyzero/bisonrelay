package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/companyzero/bisonrelay/rpc"
)

var errUnknownRPCCommand = errors.New("unknown rpc command")

func payloadForCmd(cmd string) (interface{}, error) {
	var p interface{}

	switch cmd {
	case rpc.TaggedCmdPing:
		p = new(rpc.Ping)
	case rpc.TaggedCmdPong:
		p = new(rpc.Pong)
	case rpc.TaggedCmdAcknowledge:
		p = new(rpc.Acknowledge)
	case rpc.TaggedCmdRouteMessage:
		p = new(rpc.RouteMessage)
	case rpc.TaggedCmdRouteMessageReply:
		p = new(rpc.RouteMessageReply)
	case rpc.TaggedCmdSubscribeRoutedMessagesReply:
		p = new(rpc.SubscribeRoutedMessagesReply)
	case rpc.TaggedCmdPushRoutedMessage:
		p = new(rpc.PushRoutedMessage)
	case rpc.TaggedCmdGetInvoiceReply:
		p = new(rpc.GetInvoiceReply)
	default:
		return nil, errUnknownRPCCommand
	}

	return p, nil
}

func decodeRPCPayload(message *rpc.Message, dec *json.Decoder) (interface{}, error) {
	p, err := payloadForCmd(message.Command)
	if err != nil {
		return nil, err
	}

	err = dec.Decode(&p)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal %q: %v", message.Command, err)
	}

	return p, err
}

func fingerprintDER(c tls.Certificate) string {
	if len(c.Certificate) != 1 {
		return "unexpected chained certificate"
	}

	d := sha256.New()
	d.Write(c.Certificate[0])
	digest := d.Sum(nil)
	return hex.EncodeToString(digest)
}

func randomUint64() uint64 {
	var b [8]byte
	rand.Read(b[:])
	return binary.BigEndian.Uint64(b[:])
}
