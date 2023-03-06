package rpcserver

import (
	"context"
	"sync"

	"github.com/companyzero/bisonrelay/client/internal/replaymsglog"
	"github.com/decred/slog"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// clientrpcMsg abstracts message types.
type clientrpcMsg interface {
	ProtoReflect() protoreflect.Message
	Reset()
}

// setSequenceIDField sets a field named sequence_id (uint64) to the specified
// value.
func setSequenceIDField(ntfn clientrpcMsg, val uint64) {
	ref := ntfn.ProtoReflect()
	seqIdField := ref.Descriptor().Fields().ByName("sequence_id")
	ref.Set(seqIdField, protoreflect.ValueOfUint64(val))
}

// serverStreamFor abstracts a server-side stream for a particular message type.
type serverStreamFor[T clientrpcMsg] interface {
	Send(T) error
}

// serverStream is a cancellable server stream.
type serverStream[T clientrpcMsg] struct {
	cancel func()
	stream serverStreamFor[T]
}

// serverStreams tracks all the streams for a given server stream type.
type serverStreams[T clientrpcMsg] struct {
	mtx       sync.Mutex
	replayLog *replaymsglog.Log
	nextID    int32
	m         map[int32]serverStream[T]
	prefix    string
	log       slog.Logger
}

// runStream runs the stream, sending unacked messages from the replayLog
// starting at unackedFrom.
func (cs *serverStreams[T]) runStream(ctx context.Context, unackedFrom uint64, stream serverStreamFor[T]) error {
	cs.mtx.Lock()

	// Read unacked messages, starting at unackedFrom.
	var ntfn T
	ntfn = ntfn.ProtoReflect().New().Interface().(T)
	id := replaymsglog.ID(unackedFrom)
	err := cs.replayLog.ReadAfter(id, ntfn, func(id replaymsglog.ID) error {
		cs.log.Infof("XXXXXXXXXXXX read %s", id)
		setSequenceIDField(ntfn, uint64(id))
		err := stream.Send(ntfn)
		if err != nil {
			return err
		}
		ntfn.Reset()
		return nil
	})
	if err != nil {
		cs.mtx.Unlock()
		return err
	}

	// Register this new stream.
	ctx, cancel := context.WithCancel(ctx)
	sid := cs.nextID + 1
	cs.nextID += 1
	cs.m[sid] = serverStream[T]{cancel: cancel, stream: stream}
	cs.mtx.Unlock()

	cs.log.Tracef("Started running stream %d for %s", sid, cs.prefix)

	// Wait until the stream is done.
	<-ctx.Done()

	// Unregister the stream.
	cs.mtx.Lock()
	delete(cs.m, sid)
	cs.mtx.Unlock()

	return ctx.Err()
}

// send the notification on all running streams.
func (cs *serverStreams[T]) send(ntfn T) {
	cs.mtx.Lock()

	// Save in replay file.
	replayID, err := cs.replayLog.Store(ntfn)
	if err != nil {
		cs.mtx.Unlock()
		cs.log.Errorf("Unable to store %s in replay log: %v", cs.prefix, err)
		return
	}
	setSequenceIDField(ntfn, uint64(replayID))

	// Copy list of streams to send to.
	streams := make(map[int32]serverStream[T], len(cs.m))
	for id, s := range cs.m {
		streams[id] = s
	}
	cs.mtx.Unlock()

	// Send on each stream.
	for id, s := range streams {
		err := s.stream.Send(ntfn)
		if err != nil {
			cs.log.Errorf("Unable to send %s notification to stream %d: %v",
				cs.prefix, id, err)
			s.cancel()
		}
	}
}

// ack clears the replay log up to the specified id.
func (cs *serverStreams[T]) ack(upTo uint64) error {
	id := replaymsglog.ID(upTo)
	err := cs.replayLog.ClearUpTo(id)
	if err != nil {
		cs.log.Errorf("Unable to clear %s log up to id %s: %v",
			cs.prefix, id, err)
	}
	return err

}

// newServerStreams creates a new set of server streams for a given message type.
func newServerStreams[T clientrpcMsg](rootDir, prefix string, log slog.Logger) (*serverStreams[T], error) {
	rpl, err := replaymsglog.New(replaymsglog.Config{
		Log:     log,
		Prefix:  prefix,
		RootDir: rootDir,
		MaxSize: 1 << 23, // 8MiB
	})
	if err != nil {
		return nil, err
	}

	return &serverStreams[T]{
		log:       log,
		replayLog: rpl,
		prefix:    prefix,
		m:         make(map[int32]serverStream[T]),
	}, nil
}
