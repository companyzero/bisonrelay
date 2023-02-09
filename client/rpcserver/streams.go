package rpcserver

import (
	"context"
	"math/rand"
	"sync"
)

// serverStream is a cancellable server stream.
type serverStream[T any] struct {
	cancel func()
	stream T
}

// serverStreams tracks all the streams for a given server stream type.
type serverStreams[T any] struct {
	mtx sync.Mutex
	m   map[int32]serverStream[T]
}

func (cs *serverStreams[T]) register(ctx context.Context, stream T) (int32, context.Context) {
	cs.mtx.Lock()
	if cs.m == nil {
		cs.m = make(map[int32]serverStream[T], 1)
	}
	ctx, cancel := context.WithCancel(ctx)
	i := rand.Int31()
	_, ok := cs.m[i]
	for ok {
		i = rand.Int31()
		_, ok = cs.m[i]
	}
	cs.m[i] = serverStream[T]{cancel: cancel, stream: stream}
	cs.mtx.Unlock()
	return i, ctx
}

func (cs *serverStreams[T]) unregister(i int32) {
	cs.mtx.Lock()
	delete(cs.m, i)
	cs.mtx.Unlock()
}

func (cs *serverStreams[T]) iterateOver(f func(int32, T)) {
	// Make a copy under the lock so we can call f without the lock being
	// held.
	cs.mtx.Lock()
	streams := make(map[int32]serverStream[T], len(cs.m))
	for id, s := range cs.m {
		streams[id] = s
	}
	cs.mtx.Unlock()
	for id, s := range streams {
		f(id, s.stream)
	}
}
