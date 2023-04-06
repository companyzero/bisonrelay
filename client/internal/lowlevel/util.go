package lowlevel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
)

// canceled returns true if the given context is done.
func canceled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

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
		return nil, makeUnmarshalError(message.Command, err)
	}

	return p, err
}

// blockingMsgReaderWriter is a msgReaderWriter that blocks any operations
// until the underlying context is canceled.
type blockingMsgReaderWriter struct {
	ctx context.Context
}

func (rw blockingMsgReaderWriter) Read() ([]byte, error) {
	<-rw.ctx.Done()
	return nil, rw.ctx.Err()
}

func (rw blockingMsgReaderWriter) Write([]byte) error {
	<-rw.ctx.Done()
	return rw.ctx.Err()
}

func joinRVList(rvs []ratchet.RVPoint) string {
	b := new(strings.Builder)
	for i, rv := range rvs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(rv.String())
	}
	return b.String()
}

// rvMapKeys returns all the keys in the given rv map, split into 2 groups:
// keys where onlyMarkPaid is false and ones where it is true.
func rvMapKeys(m map[RVID]rdzvSub) ([]ratchet.RVPoint, []ratchet.RVPoint) {
	toAdd := make([]ratchet.RVPoint, 0, len(m))
	var toMark []ratchet.RVPoint
	for id, sub := range m {
		if sub.onlyMarkPaid {
			toMark = append(toMark, id)
		} else {
			toAdd = append(toAdd, id)
		}
	}
	return toAdd, toMark
}

// removePrepaidSubs removes RVs from needsPay where the corresponding sub is
// marked as prepaid.
func removePrepaidSubs(needsPay []ratchet.RVPoint, subs map[RVID]rdzvSub) []ratchet.RVPoint {
	for i := 0; i < len(needsPay); {
		if !subs[needsPay[i]].prepaid {
			i += 1
			continue
		}

		needsPay[i] = needsPay[len(needsPay)-1]
		needsPay = needsPay[:len(needsPay)-1]
	}
	return needsPay
}

// multiCtx returns a context that is canceled once any one of the passed
// contexts are cancelled.
//
// The returned Cancel() function MUST be called, otherwise this leaks
// goroutines.
func multiCtx(ctxs ...context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	for _, c := range ctxs {
		c := c
		go func() {
			select {
			case <-c.Done():
				cancel()
			case <-ctx.Done():
			}
		}()
	}
	return ctx, cancel
}
