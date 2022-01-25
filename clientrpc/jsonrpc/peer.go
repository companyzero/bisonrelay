package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
)

// waitingRequest is a request made on the client that is still waiting for a
// reply on the server.
type waitingRequest struct {
	id      uint32
	method  string
	resChan chan interface{}
	ctx     context.Context
}

var errRunDone = errors.New("run done")

// peer is a bidirectional JSON-RPC peer. It can send requests and receive
// responses and notifications (in the form of streams).
//
// It is a generic peer implementation that can work as long as its nextDecoder,
// nextEncoder and flushLastWrite functions are provided.
type peer struct {
	services *types.ServersMap
	log      slog.Logger

	nextDecoder    func() (*json.Decoder, error)
	nextEncoder    func() (*json.Encoder, error)
	flushLastWrite func() error

	id       uint32 // Atomic
	runDone  chan struct{}
	readEOFd chan struct{}
	reqsSema requestsSemaphore

	outQ          chan outboundMsg
	waitReplyChan chan waitingRequest
	replyRcvdChan chan inboundMsg

	mtx      sync.Mutex
	waiting  map[uint32]waitingRequest
	streamID uint64
	streams  map[string]*responseStream
}

func (p *peer) requestStream(ctx context.Context, method string, params proto.Message) (*responseStream, error) {
	stream := newResponseStream(p, method)
	p.mtx.Lock()

	// Append stream ID to method name.
	id := p.streamID
	p.streamID += 1
	method = fmt.Sprintf("%s[%.8x]", method, id)
	if _, ok := p.streams[method]; ok {
		p.mtx.Unlock()
		return nil, fmt.Errorf("already have stream to method %s", method)
	}
	p.streams[method] = stream
	p.mtx.Unlock()

	go func() {
		err := p.request(ctx, method, params, nil)
		p.mtx.Lock()
		delete(p.streams, method)
		p.mtx.Unlock()
		stream.push(err, true)
	}()
	return stream, nil
}

func (p *peer) request(ctx context.Context, method string, params, resp proto.Message) error {
	id := atomic.AddUint32(&p.id, 1)
	out := outboundMsg{
		Version: version,
		ID:      id,
		Params:  &protoPayload{params},
		Method:  &method,
	}

	// Setup the reply waiter.
	w := waitingRequest{
		ctx:     ctx,
		id:      id,
		method:  method,
		resChan: make(chan interface{}),
	}
	p.mtx.Lock()
	p.waiting[id] = w
	p.mtx.Unlock()

	select {
	case <-p.runDone:
		return errRunDone
	case p.outQ <- out:
	case <-ctx.Done():
		p.mtx.Lock()
		delete(p.waiting, id)
		p.mtx.Unlock()
		return ctx.Err()
	}

	select {
	case res := <-w.resChan:
		switch res := res.(type) {
		case error:
			return res
		case inboundMsg:
			if resp != nil && res.Result != nil {
				err := unmarshalOpts.Unmarshal(res.Result, resp)
				if err != nil {
					err = fmt.Errorf("unable to unmarshal result: %v", err)
				}
				return err
			}
			return nil
		default:
			panic("unhandled case in <-w.resChan")
		}
	case <-p.runDone:
		return errRunDone
	case <-ctx.Done():
		p.mtx.Lock()
		delete(p.waiting, id)
		p.mtx.Unlock()
		return ctx.Err()
	}
}

func (p *peer) handleResponse(ctx context.Context, in inboundMsg) {
	id64, ok := in.ID.(float64)
	if !ok {
		// Log wrong type of ID.
		p.log.Warnf("Received message with non-number ID %v", in.ID)
		return
	}
	id := uint32(id64)

	p.mtx.Lock()
	w, ok := p.waiting[id]
	delete(p.waiting, id)
	p.mtx.Unlock()

	if !ok {
		p.log.Warnf("Received response without prior request with ID %d", id)
		return
	}

	var res interface{}
	if in.Error != nil {
		res = error(in.Error)
	} else {
		res = in
	}

	select {
	case w.resChan <- res:
	case <-w.ctx.Done():
	case <-ctx.Done():
	}
}

func (p *peer) queueResponse(ctx context.Context, id interface{}, result proto.Message, err error) {
	var out outboundMsg
	if err != nil {
		out = outboundFromError(id, err)
	} else {
		out = outboundMsg{
			Version: version,
			Result:  &protoPayload{payload: result},
			ID:      id,
		}
	}
	select {
	case <-ctx.Done():
	case p.outQ <- out:
	}
}

func (p *peer) queueNotification(method string, payload proto.Message) error {
	sentChan := make(chan struct{})
	out := outboundMsg{
		Version:  version,
		Params:   &protoPayload{payload: payload},
		Method:   &method,
		sentChan: sentChan,
	}
	select {
	case <-p.runDone:
		return errRunDone
	case p.outQ <- out:
	}

	// Wait until ntfn is sent in the wire.
	select {
	case <-p.runDone:
		return errRunDone
	case <-sentChan:
		return nil
	}
}

func (p *peer) handleNotfication(ctx context.Context, in inboundMsg) {
	p.mtx.Lock()
	stream, ok := p.streams[*in.Method]
	p.mtx.Unlock()
	if !ok {
		// Notification without a corresponding stream.
		p.log.Warnf("Received unexpected notification for method %q", *in.Method)
		return
	}

	stream.push(in, false)
}

var streamIDRegexp = regexp.MustCompile(`(\w+\.\w+)\[(\d+)\]`)

func extractStreamID(method string) (string, string) {
	matches := streamIDRegexp.FindStringSubmatch(method)
	if len(matches) == 3 {
		return matches[1], matches[2]
	}
	return method, ""
}

func (p *peer) handleRequest(ctx context.Context, in inboundMsg) {
	var err error
	var res proto.Message
	var protoReq proto.Message

	// Determine the service.
	fullMethod := *in.Method
	method, _ := extractStreamID(*in.Method)
	_, svc, methodDefn, err := p.services.SvcForMethod(method)
	if err != nil {
		p.queueResponse(ctx, in.ID, nil, err)
		p.log.Debugf("Error calling SvcForMethod %s: %v", method, err)
		return
	}

	// Decode the params as the correct request type.
	protoReq = methodDefn.NewRequest()
	if err := unmarshalOpts.Unmarshal(in.Params, protoReq); err != nil {
		err = newError(ErrInvalidParams, fmt.Sprintf("unable to decode params: %v", err))
		p.queueResponse(ctx, in.ID, nil, err)
		p.log.Debugf("Error unmarshalling request for %T: %v", protoReq, err)
		return
	}

	// Call the handler.
	if methodDefn.IsStreaming {
		stream := &requestStream{p: p, method: fullMethod}
		err = methodDefn.ServerStreamHandler(svc, ctx, protoReq, stream)

		// Send final EOF.
		if err == nil {
			err = io.EOF
		}
	} else {
		res = methodDefn.NewResponse()
		err = methodDefn.ServerHandler(svc, ctx, protoReq, res)
		if err != nil {
			p.log.Debugf("Error handling request %s: %v", method, err)
		}
	}

	// Send reply.
	p.queueResponse(ctx, in.ID, res, err)
}

func (p *peer) readLoop(ctx context.Context) error {
	var loopErr error

	chanDec := make(chan *json.Decoder, 1)
	chanErr := make(chan error, 1)
	nextDecoder := func() {
		dec, err := p.nextDecoder()
		if err != nil {
			chanErr <- err
		} else {
			chanDec <- dec
		}
	}

	var dec *json.Decoder
loop:
	for {
		go nextDecoder()
		select {
		case dec = <-chanDec:
		case loopErr = <-chanErr:
			break loop
		case <-ctx.Done():
			loopErr = ctx.Err()
			break loop
		}

		// Decode JSON-RPC request.
		var in inboundMsg
		if err := dec.Decode(&in); err != nil {
			loopErr = err
			break loop
		}

		if in.Version != version {
			loopErr = MakeError(ErrInvalidRequest,
				"unsupported JSON-RPC version")
			break loop
		}

		nilID := in.ID == nil
		nilError := in.Error == nil
		nilResult := in.Result == nil
		nilMethod := in.Method == nil
		nilParams := in.Params == nil

		// Determine the type of message.
		switch {
		case nilID && !nilError && nilResult && nilMethod && nilParams:
			// Response with error decoding ID.
			// Log error as there's nothing to do.
			p.log.Debugf("Received error with nil ID: %v", in.Error)

		case !nilID && nilError != nilResult && nilMethod && nilParams:
			// Valid response with result or error payload
			go p.handleResponse(ctx, in)

		case nilID && nilError && nilResult && !nilMethod:
			// Valid notification or standard.
			// This isn't called as a goroutine to ensure ordering
			// in the stream.
			p.handleNotfication(ctx, in)

		case !nilID && nilError && nilResult && !nilMethod:
			// Valid request.
			if !p.reqsSema.acquire(ctx) {
				loopErr = ctx.Err()
				break loop
			}
			go func() {
				p.handleRequest(ctx, in)
				p.reqsSema.release()
			}()

		default:
			// Unrecognized message. Log error.
			p.log.Debugf("Received unrecognized message: %v", in)

		}
	}

	if !errors.Is(loopErr, context.Canceled) {
		p.log.Debugf("readLoop exiting due to unexpected error: %v", loopErr)
	}

	// Wait until all outstanding requests have been processed before
	// terminating the peer.
	p.reqsSema.drain(ctx)
	return loopErr
}

func (p *peer) writeLoop(ctx context.Context) error {

	chanEnc := make(chan *json.Encoder, 1)
	chanErr := make(chan error, 1)
	nextEncoder := func() {
		dec, err := p.nextEncoder()
		if err != nil {
			chanErr <- err
		} else {
			chanEnc <- dec
		}
	}

	var enc *json.Encoder
	var loopErr error
loop:
	for {
		var msg outboundMsg
		select {
		case msg = <-p.outQ:
		case <-ctx.Done():
			loopErr = ctx.Err()
			break loop
		}

		go nextEncoder()
		select {
		case enc = <-chanEnc:
		case err := <-chanErr:
			loopErr = fmt.Errorf("error obtaining next encoder to write: %w", err)
			break loop
		}

		if err := enc.Encode(msg); err != nil {
			loopErr = fmt.Errorf("error writing encoded msg: %w", err)
			break loop
		}
		if err := p.flushLastWrite(); err != nil {
			loopErr = fmt.Errorf("error flushing encoded msg: %w", err)
			break loop
		}

		if msg.sentChan != nil {
			close(msg.sentChan)
		}
	}

	if !errors.Is(loopErr, context.Canceled) {
		p.log.Debugf("Exiting readLoop due to unexpected error: %v", loopErr)
	}
	return loopErr
}

func (p *peer) run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return p.readLoop(gctx) })
	g.Go(func() error { return p.writeLoop(gctx) })
	err := g.Wait()
	close(p.runDone)
	return err
}

func newPeer(services *types.ServersMap, log slog.Logger, nextDecoder func() (*json.Decoder, error),
	nextEncoder func() (*json.Encoder, error), flushLastWrite func() error) *peer {

	// Number of max concurrent inflight requests on the server (including
	// streams).
	const maxConcurrentRequests = 16

	return &peer{
		services:       services,
		log:            log,
		nextDecoder:    nextDecoder,
		nextEncoder:    nextEncoder,
		flushLastWrite: flushLastWrite,

		runDone:       make(chan struct{}),
		readEOFd:      make(chan struct{}),
		outQ:          make(chan outboundMsg),
		waitReplyChan: make(chan waitingRequest),
		replyRcvdChan: make(chan inboundMsg),
		reqsSema:      makeRequestsSemaphore(maxConcurrentRequests),

		waiting: make(map[uint32]waitingRequest),
		streams: make(map[string]*responseStream),
	}
}
