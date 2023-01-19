package lowlevel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
)

type wireMsg struct {
	msg            rpc.Message
	payload        interface{}
	writeReplyChan chan error
	replyChan      chan<- interface{}
}

func (wm *wireMsg) writeReply(err error) {
	go func() { wm.writeReplyChan <- err }()
}

type pushedMsg struct {
	tag     uint32
	payload interface{}
}

type tagCB struct {
	tag       uint32
	replyChan chan<- interface{}
}

func (tcb tagCB) returnTag(tagStack chan uint32) {
	go func() { tagStack <- tcb.tag }()
}

func (tcb tagCB) reply(r interface{}) {
	if tcb.replyChan == nil {
		return
	}
	go func() { tcb.replyChan <- r }()
}

// msgReaderWriter is the minimum interface for a kx'd session that is able to
// transport individual msgs.
type msgReaderWriter interface {
	Read() ([]byte, error)
	Write([]byte) error
}

// serverSession stores session state relative to a specific network connection
// to the server.
//
// It also maintains invariants expected from a continuous network connection to
// the server. In particular, it ensures at most `tagStackDepth` messages are
// kept in flight at any time, that replies are directed to the correct channel
// and that errors cascade to trigger a run loop closure and eventual
// disconnection from this session.
type serverSession struct {
	// The following fields should only be set during setup of the session
	// and are not safe for concurrent modification.

	conn         clientintf.Conn
	kx           msgReaderWriter
	log          slog.Logger
	pc           clientintf.PaymentClient
	pingInterval time.Duration
	lnNode       string

	tagStackDepth  int64  // max nb of inflight requests
	payScheme      string // negotiated between server and client on welcome
	pushPayRate    uint64 // Push Payment rate in MAtoms/byte
	subPayRate     uint64 // Sub payment rate in MAtoms/byte
	logPings       bool   // Whether to log ping/pong messages.
	expirationDays int    // After When data is purged from server

	policy clientintf.ServerPolicy

	// Handler for pushed routed messages.
	//
	// If the handler returns an error that unwraps into an AckError
	// instance, then the given error code and messages are returned. In
	// that case, the NonFatal flag of the error can be set to indicate the
	// recvLoop should _not_ be canceled with an error (and thus the server
	// connection is not unmade).
	//
	// All other instances of errors returned by the handler cause the
	// recvLoop to terminate with an error.
	pushedRoutedMsgsHandler func(msg *rpc.PushRoutedMessage) error

	// The following fields are initialized by newServerSession().

	ctx                context.Context
	cancel             func()
	sendChan           chan wireMsg
	sendChanClosed     chan struct{}
	sendAckChan        chan wireMsg
	recvTagsChan       chan tagCB
	taggedSendChan     chan wireMsg
	tagStack           chan uint32
	pongChan           chan struct{}
	closeRequestedChan chan error
}

var (
	errSendLoopExiting = fmt.Errorf("send loop: %w", clientintf.ErrSubsysExiting)
	errRecvLoopExiting = fmt.Errorf("receive loop: %w", clientintf.ErrSubsysExiting)
)

func (sess *serverSession) String() string {
	return sess.conn.RemoteAddr().String()
}

func (sess *serverSession) PayClient() clientintf.PaymentClient {
	return sess.pc
}

func (sess *serverSession) LNNode() string {
	return sess.lnNode
}

func (sess *serverSession) PaymentRates() (uint64, uint64) {
	return sess.pushPayRate, sess.subPayRate
}

func (sess *serverSession) ExpirationDays() int {
	return sess.expirationDays
}

func (sess *serverSession) Policy() clientintf.ServerPolicy {
	return sess.policy
}

// SendPRPC sends the given msg and payload to the server. This returns when
// the msg has been sent with any errors generated during the send process.
//
// The cb channel will written to when the server sends a reply to the given
// message or if the session with the server has been closed while waiting for
// the reply. If the reply chan is nil, the reply is discarded.
func (sess *serverSession) SendPRPC(msg rpc.Message, payload interface{},
	reply chan<- interface{}) error {

	m := wireMsg{
		msg:            msg,
		payload:        payload,
		writeReplyChan: make(chan error),
		replyChan:      reply,
	}

	select {
	case sess.sendChan <- m:
	case <-sess.sendChanClosed:
		return errSendLoopExiting
	}
	return <-m.writeReplyChan
}

// Context returns a context that is closed once this server finishes running.
func (sess *serverSession) Context() context.Context {
	return sess.ctx
}

// close this session's conn (if it exists) and log any errors to the session
// logger. This is callable even when the session or the conn is nil.
func (sess *serverSession) close() {
	if sess == nil {
		return
	}
	if sess.conn == nil {
		return
	}
	err := sess.conn.Close()
	if err == nil {
		return
	}

	// No need to log context.Canceled errors.
	if errors.Is(err, context.Canceled) {
		return
	}
	log := sess.log
	if log == nil {
		log = slog.Disabled
	}
	log.Errorf("Error while closing connection: %v", err)
}

// RequestClose requests that this session be closed for the given reason.
func (sess *serverSession) RequestClose(err error) {
	if err == nil {
		return
	}

	select {
	case sess.closeRequestedChan <- err:
	case <-sess.ctx.Done():
	}
}

// handlePushedMsg calls the handler for the given pushed message and sends the
// ack result to sendAckChan. Fatal errors during processing are sent to
// handlerErrChan.
func (sess *serverSession) handlePushedMsg(ctx context.Context, pm pushedMsg,
	sendAckChan chan wireMsg, handlerErrChan chan error) {

	// Determine which handler to use to process the pushed message.
	handler := func() error { return nil }

	sess.log.Tracef("Handling pushed msg of type %T", pm.payload)

	switch payload := pm.payload.(type) {
	case *rpc.PushRoutedMessage:
		if sess.pushedRoutedMsgsHandler != nil {
			handler = func() error {
				return sess.pushedRoutedMsgsHandler(payload)
			}
		} else {
			sess.log.Warnf("Empty pushedRouterMsgsHandler")
		}

	default:
		handlerErrChan <- fmt.Errorf("unknown pushed msg type %t", pm.payload)
		return
	}

	// Call the handler and figure out the result of the error.
	err := handler()

	// Assemble the ack to send as response to the server.
	var ack rpc.Acknowledge
	var errAck AckError
	var nonFatal bool // Whether to cancel recvLoop.
	if errors.As(err, &errAck) {
		errAck.ToAck(&ack)
		nonFatal = errAck.NonFatal
	} else if err != nil {
		ack.Error = err.Error()
	}

	// Send the ack result.
	wm := wireMsg{
		msg: rpc.Message{
			Command: rpc.TaggedCmdAcknowledge,
			Tag:     pm.tag,
		},
		payload:        ack,
		writeReplyChan: make(chan error),
	}
	select {
	case sendAckChan <- wm:
	case <-ctx.Done():
		return
	}

	// Wait until the server received the error'd ack reply.
	select {
	case <-wm.writeReplyChan:
	case <-ctx.Done():
		return
	}

	if err == nil || nonFatal {
		return
	}

	// Cancel recvLoop due to fatal error.
	select {
	case handlerErrChan <- err:
	case <-ctx.Done():
	}
}

// readAndDecodeMessage reads and decodes the next message in the kx wire. It
// returns the payload of the message.
func (sess *serverSession) readAndDecodeMessage(msg *rpc.Message) (interface{}, error) {
	rawMsg, err := sess.kx.Read()
	if err != nil {
		return nil, err
	}

	br := bytes.NewReader(rawMsg)
	dec := json.NewDecoder(br)
	err = dec.Decode(&msg)
	if err != nil {
		return nil, makeUnmarshalError("header", err)
	}

	payload, err := decodeRPCPayload(msg, dec)
	if err != nil {
		return nil, err
	}

	if msg.Command != rpc.TaggedCmdPong || sess.logPings {
		sess.log.Tracef("Decoded msg %q, tag %d, %d bytes", msg.Command,
			msg.Tag, len(rawMsg))
	}

	return payload, nil
}

func (sess *serverSession) recvLoop(ctx context.Context) error {
	sendAckChan := sess.sendAckChan
	recvTagsChan := sess.recvTagsChan
	tagStack := sess.tagStack
	pongChan := sess.pongChan

	msgChan := make(chan interface{})
	errChan := make(chan error)
	pmErrChan := make(chan error)
	tags := make(map[uint32]*tagCB, sess.tagStackDepth)

	var message rpc.Message
	readNextMsg := func() {
		msg, err := sess.readAndDecodeMessage(&message)
		if err != nil {
			errChan <- err
		} else {
			msgChan <- msg
		}
	}
	drainNextMsg := func() {
		select {
		case <-msgChan:
		case <-errChan:
		}
	}

	// Start reading messages.
	go readNextMsg()

	var err error

nextMsg:
	for {
		select {
		case tcb := <-recvTagsChan:
			// We expect to receive an ack for the given sent tag.
			tags[tcb.tag] = &tcb
		case msg := <-msgChan:
			// Handle the msg.
			switch msg := msg.(type) {
			case *rpc.Pong:
				// Alert we received a pong.
				go func() {
					select {
					case pongChan <- struct{}{}:
					case <-ctx.Done():
					}
				}()
			case *rpc.PushRoutedMessage:
				// Pushed messages are sent to a handler.
				pm := pushedMsg{tag: message.Tag, payload: msg}
				go sess.handlePushedMsg(ctx, pm, sendAckChan, pmErrChan)
			default:
				// Replies are sent to the original caller of
				// the sendPRPC(). The tag is returned to the
				// taggingLoop.
				tcb, ok := tags[message.Tag]
				if !ok {
					err = makeInvalidRecvTagError(message.Command,
						message.Tag)
					break nextMsg
				}

				// TODO: ensure the reply cmd is appropriate
				// for the original cmd.
				delete(tags, message.Tag)
				tcb.returnTag(tagStack)
				tcb.reply(msg)
			}

			// Start reading the next msg.
			go readNextMsg()
		case err = <-errChan:
			// Read error (EOF, unmarshal, etc). Stop the loop.
			break nextMsg
		case err = <-pmErrChan:
			// Fatal error processing a pushed msg. Stop the loop.
			go drainNextMsg()
			break nextMsg
		case <-ctx.Done():
			// Exiting. Stop the loop.
			go drainNextMsg()
			err = ctx.Err()
			break nextMsg
		}
	}

	// Alert all outstanding sendPRPC() callers that they won't get a
	// reply.
	for _, tcb := range tags {
		tcb.returnTag(tagStack)
		tcb.reply(errRecvLoopExiting)
	}

	return err
}

// taggingLoop fetches messages from the sendChan and tags from the tagstack
// and combines them so we can send messages that need remote acks.
func (sess *serverSession) taggingLoop(ctx context.Context) error {
	sendChan := sess.sendChan
	taggedSendChan := sess.taggedSendChan
	recvTagsChan := sess.recvTagsChan
	tagStack := sess.tagStack

	var err error
	var tag uint32
	var wm wireMsg

nextMsg:
	for {
		// Fetch the next available tag.
		select {
		case tag = <-tagStack:
		case <-time.After(time.Second):
			sess.log.Debugf("tagstack exhausted")
			continue nextMsg
		case <-ctx.Done():
			err = ctx.Err()
			break nextMsg
		}

		// Fetch the next msg to send.
		select {
		case wm = <-sendChan:
		case <-ctx.Done():
			err = ctx.Err()
			break nextMsg
		}

		// Prepare the receiver loop to receive a response for this tag
		// (so it knows where to send the ack reply to).
		select {
		case recvTagsChan <- tagCB{tag, wm.replyChan}:
		case <-ctx.Done():
			// Let caller of the this message know we're shutting
			// down.
			err = ctx.Err()
			wm.writeReply(errSendLoopExiting)
			break nextMsg
		}

		wm.msg.Tag = tag

		// Direct it to the send queue.
		select {
		case taggedSendChan <- wm:
		case <-ctx.Done():
			err = ctx.Err()
			wm.writeReply(errSendLoopExiting)
			break nextMsg
		}
	}

	// Mark the send loop as exiting to avoid deadlocking calls to
	// sendPRPC().
	close(sess.sendChanClosed)

	return err
}

// writeMessage encodes and sends the given message and payload to the kx wire.
func (sess *serverSession) writeMessage(msg *rpc.Message, payload interface{}) error {
	msg.TimeStamp = time.Now().Unix() // set timestamp

	var bb bytes.Buffer
	enc := json.NewEncoder(&bb)
	err := enc.Encode(msg)
	if err != nil {
		return fmt.Errorf("could not marshal message '%v': %w", msg.Command, err)
	}
	err = enc.Encode(payload)
	if err != nil {
		return fmt.Errorf("could not marshal payload '%v': %w", msg.Command, err)
	}

	b := bb.Bytes()
	err = sess.kx.Write(b)
	if err != nil {
		return fmt.Errorf("could not write '%v': %w",
			msg.Command, err)
	}

	if msg.Command != rpc.TaggedCmdPing || sess.logPings {
		sess.log.Tracef("Wrote msg %q, tag %v, %d bytes", msg.Command, msg.Tag,
			len(b))
	}

	return nil
}

// sendLoop sends messages as fast as possible to the wire.
func (sess *serverSession) sendLoop(ctx context.Context) error {
	taggedSendChan := sess.taggedSendChan
	sendAckChan := sess.sendAckChan
	pongChan := sess.pongChan

	// Setup ping/pong if requested. We send a ping after pingInterval has
	// elapsed from the last sent message and expect a server pong. Note
	// the tag used equals the tagStackDepth of the session.
	gotPong := true
	var pingChan <-chan time.Time
	var ticker *time.Ticker
	var pingMsg *rpc.Message
	resetPingInterval := func() {}
	if sess.pingInterval > 0 {
		ticker = time.NewTicker(sess.pingInterval)
		pingChan = ticker.C
		defer ticker.Stop()

		pingMsg = &rpc.Message{
			Command: rpc.TaggedCmdPing,
			Tag:     uint32(sess.tagStackDepth),
		}
		resetPingInterval = func() { ticker.Reset(sess.pingInterval) }
	}

	// Helper to write a message and payload.
	lastWriteTime := time.Now().Round(0) // real time
	writeMsg := func(msg *rpc.Message, payload interface{}) error {
		timeSince := time.Since(lastWriteTime)
		if timeSince > rpc.PingLimit {
			// Client took too long to send a message. Close session.
			return fmt.Errorf("sendLoop stalled for %s", timeSince)
		}
		err := sess.writeMessage(msg, payload)
		lastWriteTime = time.Now().Round(0)
		resetPingInterval()
		return err
	}

	var err error

loop:
	for err == nil {
		select {
		case <-pongChan:
			gotPong = true
			continue loop

		case <-pingChan:
			if !gotPong {
				err = errPongTimeout
				break loop
			}
			gotPong = false
			err = writeMsg(pingMsg, rpc.Ping{})

		case wm := <-taggedSendChan:
			err = writeMsg(&wm.msg, wm.payload)
			wm.writeReply(err)

		case wm := <-sendAckChan:
			err = writeMsg(&wm.msg, wm.payload)
			wm.writeReply(err)

		case <-ctx.Done():
			err = ctx.Err()
		}
	}

	return err
}

// Run the internal goroutine loops. The session MUST have been completely
// setup prior to running this.
func (sess *serverSession) Run(ctx context.Context) error {
	sess.log.Tracef("Running session %s", sess.conn.RemoteAddr())

	g, gctx := errgroup.WithContext(ctx)

	// Cancel the session context once we know the session is finished.
	g.Go(func() error {
		<-gctx.Done()
		sess.cancel()
		return nil
	})

	g.Go(func() error {
		err := sess.recvLoop(gctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			sess.log.Debugf("recvLoop errored: %v", err)
			return err
		} else {
			sess.log.Tracef("recvLoop ending with err: %v", err)
		}
		return nil
	})

	g.Go(func() error {
		err := sess.taggingLoop(gctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			sess.log.Debugf("taggingLoop errored: %v", err)
			return err
		} else {
			sess.log.Tracef("taggingLoop ending with err: %v", err)
		}

		return nil
	})

	g.Go(func() error {
		err := sess.sendLoop(gctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			sess.log.Debugf("sendLoop errored: %v", err)
			return err
		} else {
			sess.log.Tracef("sendLoop ending with err: %v", err)
		}

		return nil
	})

	// Cancel the loops if we were requested to.
	g.Go(func() error {
		select {
		case err := <-sess.closeRequestedChan:
			return err
		case <-gctx.Done():
			return nil
		}
	})

	err := g.Wait()
	if err == nil {
		err = ctx.Err()
	}

	// Ensure the connection is closed.
	closeErr := sess.conn.Close()
	if closeErr != nil {
		sess.log.Debugf("Connection close error: %v", closeErr)
	}

	sess.log.Tracef("Finished running session %s with err %v",
		sess.conn.RemoteAddr(), err)
	return err
}

// newServerSession initializes the minimum fields of a new server session.
func newServerSession(conn clientintf.Conn, kx msgReaderWriter,
	tagStackDepth int64, log slog.Logger) *serverSession {

	if log == nil {
		log = slog.Disabled
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Minimum tag depth is 2: 1 tag is reserved for pings, another is for
	// sending msgs.
	if tagStackDepth <= 0 {
		tagStackDepth = 2
	}
	sess := &serverSession{
		conn:          conn,
		kx:            kx,
		log:           log,
		tagStackDepth: tagStackDepth,

		ctx:                ctx,
		cancel:             cancel,
		recvTagsChan:       make(chan tagCB),
		sendChan:           make(chan wireMsg),
		sendAckChan:        make(chan wireMsg),
		taggedSendChan:     make(chan wireMsg),
		sendChanClosed:     make(chan struct{}),
		pongChan:           make(chan struct{}),
		closeRequestedChan: make(chan error),
		tagStack:           make(chan uint32, tagStackDepth),
	}

	for i := uint32(0); i < uint32(sess.tagStackDepth); i++ {
		sess.tagStack <- i
	}

	return sess
}
