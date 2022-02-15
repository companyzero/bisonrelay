package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/server/internal/tagstack"
	"github.com/companyzero/bisonrelay/session"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
)

type sessionContext struct {
	writer   chan *RPCWrapper
	kx       *session.KX
	conn     net.Conn
	tagStack *tagstack.TagStack
	log      slog.Logger

	// subscriptions
	msgC    chan ratchet.RVPoint
	msgSetC chan rpc.SubscribeRoutedMessages
	msgAckC chan ratchet.RVPoint

	// protected
	sync.Mutex
	tagMessage      []*RPCWrapper
	lnPayReqHashSub []byte
	lnPushHashes    map[[32]byte]time.Time
}

func (z *ZKS) sessionWriter(ctx context.Context, sc *sessionContext) error {

	sc.log.Tracef("sessionWriter starting")
	defer func() {
		sc.log.Tracef("sessionWriter quit")
	}()

	for {
		var (
			msg *RPCWrapper
			ok  bool
		)

		select {
		case <-ctx.Done():
			return ctx.Err()

		case msg, ok = <-sc.writer:
			if !ok {
				sc.log.Tracef("sessionWriter sc.writer")
				return fmt.Errorf("sessionWriter sc.writer closed")
			}

			if msg.Message.Command != rpc.TaggedCmdPong || z.logPings {
				sc.log.Tracef("sessionWriter write %v %v",
					msg.Message.Command,
					msg.Message.Tag)
			}

			err := z.writeMessage(sc.kx, msg)
			if err != nil {
				sc.log.Errorf("sessionWriter write failed: %v",
					err)
				return err
			}

			if msg.CloseAfterWritingErr != nil {
				return fmt.Errorf("sessionWriter closed after writing: %v",
					msg.CloseAfterWritingErr)
			}
		}
	}
}

func (z *ZKS) sessionSubscribe(ctx context.Context, sc *sessionContext) error {
	sc.log.Tracef("subscribers: %v", "not yet set")

	// Track subscribers for this session.
	//
	// TODO: avoid having to track the RV in two places (ZKS.subscribers
	// and here) to reduce memory per RV. The current way trades speed of
	// operations and lock contention for memory consumption.
	sessSubs := make(map[ratchet.RVPoint]struct{})

	defer func() {
		// Remove all of this session's subscriptions.
		z.Lock()
		for rv := range sessSubs {
			delete(z.subscribers, rv)
		}
		z.Unlock()
		sc.log.Tracef("subscribers quit: %v", sessSubs)
	}()

loop:
	for {
		var rvsToCheck []ratchet.RVPoint

		select {
		case <-ctx.Done():
			break loop

		case s := <-sc.msgSetC:
			rvsToCheck = s.AddRendezvous

			z.Lock()
			// Remove subscriptions that were deleted.
			for _, rv := range s.DelRendezvous {
				if _, ok := sessSubs[rv]; !ok {
					continue
				}
				delete(z.subscribers, rv)
				delete(sessSubs, rv)
				z.stats.activeSubs.add(-1)
			}

			// Add new subscriptions.
			for i := 0; i < len(rvsToCheck); i++ {
				rv := rvsToCheck[i]
				if other, ok := z.subscribers[rv]; ok && sc != other {
					// Someone tried to subscribe to an RV
					// that another session was already
					// subscribed to. Skip this RV.
					copy(rvsToCheck[i:], rvsToCheck[i+1:])
					rvsToCheck = rvsToCheck[:len(rvsToCheck)-1]
					continue
				}
				z.subscribers[rv] = sc
				sessSubs[rv] = struct{}{}
				z.stats.subsRecv.add(1)
				z.stats.activeSubs.add(1)
			}
			z.Unlock()

			sc.log.Tracef("subscribers added %v deleted %v",
				s.AddRendezvous, s.DelRendezvous)

			// Fallthrough

		case rv := <-sc.msgC:
			sc.log.Tracef("subscribers read: %v", rv)
			rvsToCheck = []ratchet.RVPoint{rv}

		case rv := <-sc.msgAckC:
			sc.log.Tracef("subscribers ackd: %v", rv)

			// Ackd rv. Delete from db.
			err := z.db.RemovePayload(z.dbCtx, rv)
			if err != nil {
				sc.log.Errorf("unable to delete ackd rv: %v", err)
				return err
			}

			continue loop
		}

		// Among the new RVs added, see if any are already stored.
		for _, rv := range rvsToCheck {
			msgPayload, err := z.db.FetchPayload(z.dbCtx, rv)
			if err != nil {
				sc.log.Errorf("subscribers FetchContent: %v", err)
				continue
			}
			if msgPayload == nil || msgPayload.Payload == nil {
				continue
			}
			sc.log.Tracef("subscribers: trying to push %s",
				rv)

			// obtain tag
			tag, err := sc.tagStack.Pop()
			if err != nil {
				// this probably should be debug
				sc.log.Errorf("could not obtain tag: %v", err)
				continue
			}
			reply := RPCWrapper{
				Message: rpc.Message{
					Command: rpc.TaggedCmdPushRoutedMessage,
					Tag:     tag,
				},
				Payload: rpc.PushRoutedMessage{
					Payload:   msgPayload.Payload,
					RV:        rv,
					Timestamp: msgPayload.InsertTime.Unix(),
				},
			}

			sc.Lock()
			if sc.tagMessage[tag] != nil {
				sc.Unlock()
				sc.log.Errorf("write duplicate tag: %v", tag)
				continue
			}
			sc.tagMessage[tag] = &reply
			sc.Unlock()

			// And send
			sc.log.Debugf("Pushing %d bytes to client at RV %s",
				len(msgPayload.Payload), rv)
			z.stats.rmsSent.add(1)

			sc.writer <- &reply
		}
	}

	// Remove session subscriptions.
	z.Lock()
	for rv := range sessSubs {
		delete(z.subscribers, rv)
		z.stats.activeSubs.add(-1)
	}
	z.Unlock()

	return ctx.Err()
}

// sessionReader deals with incoming RPC calls.  For now treat all errors as
// critical and return which in turns shuts down the connection.
func (z *ZKS) sessionReader(ctx context.Context, sc *sessionContext) error {
	sc.log.Tracef("sessionReader: starting")
	defer func() {
		sc.log.Tracef("sessionReader: quit")
	}()

	// Helper to read next message from the session kx.
	nextMsgChan := make(chan interface{})
	readNextMsg := func() {
		cmd, err := sc.kx.Read()
		var v interface{}
		if err != nil {
			v = err
		} else {
			v = cmd
		}
		select {
		case nextMsgChan <- v:
		case <-ctx.Done():
		}
	}

	tagBitmap := make([]bool, tagDepth) // see if there is a duplicate tag
	for {
		var message rpc.Message

		// OpenBSD does not support per socket TCP KEEPALIVES So
		// for now we ping on the client every 10 seconds and we
		// try to read those aggresively.  We'll cope in the
		// client with aggressive reconnects.  This really is
		// ugly as sin.
		//
		// Ideally this crap goes away and we use proper TCP for
		// this.
		sc.conn.SetReadDeadline(time.Now().Add(z.pingLimit))

		// Read next message asynchronously (since .Read() blocks).
		go readNextMsg()

		var cmd []byte
		select {
		case v := <-nextMsgChan:
			switch v := v.(type) {
			case error:
				return v
			case []byte:
				cmd = v
			default:
				panic("should not happen")
			}

		case <-ctx.Done():
			return ctx.Err()
		}

		z.stats.bytesRecv.add(int64(len(cmd)))

		// unmarshal header
		br := bytes.NewReader(cmd)
		dec := json.NewDecoder(br)
		err := z.unmarshal(dec, &message)
		if err != nil {
			return fmt.Errorf("unmarshal header failed")
		}

		if message.Tag > tagDepth {
			return fmt.Errorf("invalid tag received %v", message.Tag)
		}

		if tagBitmap[message.Tag] {
			return fmt.Errorf("read duplicate tag: %v", message.Tag)
		}
		tagBitmap[message.Tag] = true

		if message.Command != rpc.TaggedCmdPing {
			sc.log.Tracef("handleSession: %v %v",
				message.Command,
				message.Tag)
		}

		// unmarshal payload
		switch message.Command {
		case rpc.TaggedCmdPing:
			var p rpc.Ping
			err = z.unmarshal(dec, &p)
			if err != nil {
				return fmt.Errorf("unmarshal Ping failed")
			}

			if z.logPings {
				sc.log.Tracef("handleSession: got ping tag %v",
					message.Tag)
			}

			sc.writer <- &RPCWrapper{
				Message: rpc.Message{
					Command: rpc.TaggedCmdPong,
					Tag:     message.Tag,
				},
				Payload: rpc.Pong{},
			}

		case rpc.TaggedCmdAcknowledge:
			sc.Lock()
			m := sc.tagMessage[message.Tag]
			sc.Unlock()

			sc.log.Tracef("handleSession: got ack tag %v",
				message.Tag)

			// sanity
			if m != nil && m.Message.Tag != message.Tag {
				return fmt.Errorf("acknowledge tag doesn't "+
					"match: %v %v",
					m.Message.Tag,
					message.Tag)
			}

			// mark free
			sc.Lock()
			sc.tagMessage[message.Tag] = nil
			sc.Unlock()

			// just push tag for now
			err = sc.tagStack.Push(message.Tag)
			if err != nil {
				return fmt.Errorf("Acknowledge can't push tag: %v",
					message.Tag)
			}

			// If the message being ack'd is a PushRM, the client
			// has processed the message. Delete from disk.
			if prm, ok := m.Payload.(rpc.PushRoutedMessage); ok {
				go func() {
					select {
					case sc.msgAckC <- prm.RV:
					case <-ctx.Done():
					}
				}()
			}

			sc.log.Tracef("handleSession: ack tag %v",
				message.Tag)

		case rpc.TaggedCmdRouteMessage:
			sc.log.Tracef("TaggedCmdRouteMessage")

			var r rpc.RouteMessage
			err = z.unmarshal(dec, &r)
			if err != nil {
				return fmt.Errorf("unmarshal RouteMessage failed")
			}
			err = z.handleRouteMessage(ctx, sc.writer, message, r, sc)
			if err != nil {
				return fmt.Errorf("handleRouteMessage: %v", err)
			}

		case rpc.TaggedCmdSubscribeRoutedMessages:
			sc.log.Tracef("TaggedCmdSubscribeRoutedMessages")

			var r rpc.SubscribeRoutedMessages
			err = z.unmarshal(dec, &r)
			if err != nil {
				return fmt.Errorf("unmarshal "+
					"SubscribeRoutedMessages failed: %v", err)
			}

			err = z.handleSubscribeRoutedMessages(ctx, message, r, sc)
			if err != nil {
				return fmt.Errorf("handleSubscribeRoutedMessages: %v", err)
			}

		case rpc.TaggedCmdGetInvoice:
			sc.log.Tracef("TaggedCmdGetInvoice")

			var r rpc.GetInvoice
			err = z.unmarshal(dec, &r)
			if err != nil {
				return fmt.Errorf("unmarshal RouteMessage failed")
			}
			err = z.handleGetInvoice(ctx, sc, message, r)
			if err != nil {
				return fmt.Errorf("handleGetInvoice %q: %v",
					r.Action, err)
			}

		default:
			return fmt.Errorf("invalid message: %v", message)
		}

		tagBitmap[message.Tag] = false
	}
}

func (z *ZKS) runNewSession(ctx context.Context, conn net.Conn, kx *session.KX) {
	var rid zkidentity.ShortID
	rand.Read(rid[:])
	log := z.logBknd.untrackedLogger(fmt.Sprintf("SESS %s", rid.ShortLogID()))

	// create session context
	sc := sessionContext{
		writer:     make(chan *RPCWrapper, tagDepth),
		kx:         kx,
		conn:       conn,
		tagStack:   tagstack.NewBlocking(tagDepth),
		tagMessage: make([]*RPCWrapper, tagDepth),
		log:        log,

		// routed messages
		msgSetC: make(chan rpc.SubscribeRoutedMessages),
		msgC:    make(chan ratchet.RVPoint, 2), // To allow write in go func itself
		msgAckC: make(chan ratchet.RVPoint),

		lnPushHashes: make(map[[32]byte]time.Time),
	}

	// Mark session online.
	z.logConn.Debugf("handleSession online: from %s id %s", conn.RemoteAddr(), rid)

	z.stats.connections.add(1)

	// Start subroutines.
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return z.sessionWriter(gctx, &sc) })
	g.Go(func() error { return z.sessionSubscribe(gctx, &sc) })
	g.Go(func() error { return z.sessionReader(gctx, &sc) })

	// Wait until something errors.
	err := g.Wait()

	// Ensure connection is closed.
	conn.Close()

	// Mark session offline.
	if !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
		z.logConn.Errorf("handleSession offline: %v", err)
	} else {
		z.logConn.Debugf("handleSession offline: %v", err)
	}

	// Cancel any outstanding invoices the session had that have not yet
	// been redeemed.
	for hash := range sc.lnPushHashes {
		err := z.cancelLNInvoice(ctx, hash[:])
		if err != nil {
			z.logConn.Warnf("handleSession: unable to cancel push "+
				"invoice hash %x", hash)
		} else {
			z.logConn.Debugf("handleSession: canceled push invoice %x", hash)
		}
	}
	if sc.lnPayReqHashSub != nil {
		err := z.cancelLNInvoice(ctx, sc.lnPayReqHashSub)
		if err != nil {
			z.logConn.Warnf("handleSession: unable to cancel sub invoice "+
				"hash %x", sc.lnPayReqHashSub)
		} else {
			z.logConn.Debugf("handleSession: canceled sub invoice %x",
				sc.lnPayReqHashSub)
		}

	}

	z.stats.disconnections.add(1)
}
