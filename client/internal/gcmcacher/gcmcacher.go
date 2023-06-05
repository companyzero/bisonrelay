package gcmcacher

// The GC Message cacher is meant to improve the UX for GC messages received
// just after connecting to the server.
//
// The challenge for this scenario is that there may be a number of messages
// from different users, for different GCs, sent at different times and the
// client only fetches messages synchronously per user.
//
// So the client may fetch message gc01-user01-ts1000 before it fetches message
// gc01-user02-ts900.
//
// This package implements a memory cacher for these received messages, such
// that the initial GC messages are fetched and cached until we can be certain
// no earlier messages will be received from any user.
//
// The following assumptions are made:
//
//   - The RV Manager starts fetching messages as soon as a new connection is
//   made.
//   - The server will send all messages it has, largely concurrently.
//   - Subscriptions for newer messages (for one user) are only made after an
//   older message is received, therefore messages for individual users are
//   received with ascending timestamps.
//
// The strategy for caching is the following: after connecting to the server,
// track most recent RM timestamp and store GCMs for all users, up to the
// configured Cacher.initialDelay duration.
//
// GCMs are sorted by timestamp (older first) and users are sorted by last
// received RM timestamp (older first).
//
// The oldest tracked GCM may be sent to the handler if there are no actively
// tracked users for which their last RM timestamp is still older than the GCM
// timestamp (which means there is no user which could still send a GCM that
// would appear with a newer timestamp).
//
// The cacher considers a user as "inactive" if it has not received any new RMs
// within the configured Cacher.maxLifetime duration. This check is performed
// on a Cacher.updateDelay basis.
//
// Once the initial Cacher.initialDelay has passed and all cached messages
// have been sent to the handler, the cacher enters a "direct push" mode, where
// no caching is done for newly received GCMs.

import (
	"container/heap"
	"context"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/decred/slog"
)

// gcmq is a priority queue for GCMessages. Sorted by timestamp.
type gcmq struct {
	msgs []clientintf.ReceivedGCMsg
}

func (gc *gcmq) Len() int {
	return len(gc.msgs)
}

func (gc *gcmq) Less(i, j int) bool {
	return gc.msgs[i].TS.Before(gc.msgs[j].TS)
}

func (gc *gcmq) Swap(i, j int) {
	gc.msgs[i], gc.msgs[j] = gc.msgs[j], gc.msgs[i]
}

func (gc *gcmq) Push(v any) {
	gc.msgs = append(gc.msgs, v.(clientintf.ReceivedGCMsg))
}

func (gc *gcmq) Pop() any {
	l := len(gc.msgs)
	r := gc.msgs[l-1]
	gc.msgs = gc.msgs[:l-1]
	return r
}

func (gc *gcmq) nextGCM() clientintf.ReceivedGCMsg {
	return heap.Pop(gc).(clientintf.ReceivedGCMsg)
}

type rmMsg struct {
	uid clientintf.UserID
	ts  time.Time
}

type remoteUser struct {
	uid    clientintf.UserID
	rmts   time.Time // last RM timestamp
	updtts time.Time // when the RM was locally received timestamp
}

// ruq is the remote user priority queue. It's sorted by the RM ts timestamp.
type ruq struct {
	users []remoteUser
}

func (r *ruq) Len() int {
	return len(r.users)
}

func (r *ruq) Less(i int, j int) bool {
	return r.users[i].rmts.Before(r.users[j].rmts)
}

func (r *ruq) Swap(i int, j int) {
	r.users[i], r.users[j] = r.users[j], r.users[i]
}

func (r *ruq) Push(v any) {
	r.users = append(r.users, v.(remoteUser))
}

func (r *ruq) Pop() any {
	l := len(r.users)
	res := r.users[l-1]
	r.users = r.users[:l-1]
	return res
}

func (r *ruq) update(uid clientintf.UserID, rmts time.Time) {
	for i := range r.users {
		if r.users[i].uid == uid {
			r.users[i].rmts = rmts
			r.users[i].updtts = time.Now()
			heap.Fix(r, i)
			return
		}
	}

	nu := remoteUser{uid: uid, rmts: rmts, updtts: time.Now()}
	heap.Push(r, nu)
}

func (r *ruq) dropStale(maxLifetime time.Duration) int {
	var nb int
	deadline := time.Now().Add(-maxLifetime)
	for i := 0; i < len(r.users); {
		if r.users[i].updtts.Before(deadline) {
			heap.Remove(r, i)
			nb += 1
		} else {
			i += 1
		}
	}
	return nb
}

func (r *ruq) canEmitGCM(gcmTS time.Time) bool {
	if len(r.users) == 0 {
		return true
	}

	return !r.users[0].rmts.Before(gcmTS)
}

// Cacher caches GC messages fetched immediately after going online to improve
// UX of notifications of new messages.
type Cacher struct {
	maxLifetime  time.Duration
	updateDelay  time.Duration
	initialDelay time.Duration

	handler func(clientintf.ReceivedGCMsg)
	log     slog.Logger

	quit          chan struct{}
	msgChan       chan clientintf.ReceivedGCMsg
	rmChan        chan rmMsg
	reloadChan    chan []clientintf.ReceivedGCMsg
	connectedChan chan bool
}

// New creates a new GC message cacher. delayTimeout is the time between
// messages which causes the cacher to delay delivering messages. maxDelayTimeout
// is a max delay after which messages will be delivered, regardless of being
// received within delayDuration of each other.
func New(maxLifetime, updateDelay, initialDelay time.Duration,
	log slog.Logger, handler func(clientintf.ReceivedGCMsg)) *Cacher {

	c := &Cacher{
		maxLifetime:  maxLifetime,
		updateDelay:  updateDelay,
		initialDelay: initialDelay,
		handler:      handler,
		log:          log,

		quit:          make(chan struct{}),
		rmChan:        make(chan rmMsg),
		msgChan:       make(chan clientintf.ReceivedGCMsg),
		reloadChan:    make(chan []clientintf.ReceivedGCMsg),
		connectedChan: make(chan bool),
	}
	return c
}

// RMReceived should be called whenever an RM for a user is received.
func (c *Cacher) RMReceived(uid clientintf.UserID, ts time.Time) {
	select {
	case c.rmChan <- rmMsg{uid: uid, ts: ts}:
	case <-c.quit:
	}
}

// GCMessageReceived should be called whenever a new GC message is externally
// received.
func (c *Cacher) GCMessageReceived(msg clientintf.ReceivedGCMsg) {
	select {
	case c.msgChan <- msg:
	case <-c.quit:
	}
}

// ReloadCachedMessages reloads messages that were previously cached by the
// cacher. This assumes the cacher has been restarted/cleared.
func (c *Cacher) ReloadCachedMessages(rgcms []clientintf.ReceivedGCMsg) {
	select {
	case c.reloadChan <- rgcms:
	case <-c.quit:
	}
}

// SessionChanged should be called whenever the session changes.
func (c *Cacher) SessionChanged(connected bool) {
	select {
	case c.connectedChan <- connected:
	case <-c.quit:
	}
}

// Run runs the cacher operations.
func (c *Cacher) Run(ctx context.Context) error {
	// Working queues.
	msgs := &gcmq{}
	users := &ruq{}

	updtTicker := time.NewTicker(c.updateDelay)
	updtTicker.Stop()
	var initialDelayChan <-chan time.Time

	// doneCaching is set to true once the initial sync is finished and the
	// cacher should switch to relaying messages directly, without caching.
	var doneCaching bool

	var wasOnline bool

loop:
	for {
		select {
		case rm := <-c.rmChan:
			if doneCaching {
				continue loop
			}
			users.update(rm.uid, rm.ts) // Track last RM ts for user.
			if initialDelayChan != nil {
				continue loop
			}

		case msg := <-c.msgChan:
			if doneCaching {
				// Caching already done, call handler without
				// delay.
				c.log.Tracef("Pushing message from %s in gc %s with ts %s",
					msg.UID, msg.GCM.ID, msg.TS)
				if c.handler != nil {
					c.handler(msg)
				}
				continue loop
			}

			c.log.Tracef("Delaying message from %s in gc %s with ts %s",
				msg.UID, msg.GCM.ID, msg.TS)

			// We'll need to delay this message. Store in message
			// queue, sorted by timestamp.
			heap.Push(msgs, msg)

			if initialDelayChan != nil {
				continue loop
			}

		case online := <-c.connectedChan:
			// Skip repeated event.
			if wasOnline == online {
				continue loop
			}
			if !online {
				users.dropStale(0)
				initialDelayChan = nil
				c.log.Tracef("Gone offline")
			} else {
				doneCaching = false
				initialDelayChan = time.After(c.initialDelay)
				c.log.Tracef("Gone online")
			}
			wasOnline = online

			// If offline, continue execution of this iteration to
			// emit currently cached msgs.
			if online {
				continue loop
			}

		case reloadedMsgs := <-c.reloadChan:
			for i := range reloadedMsgs {
				heap.Push(msgs, reloadedMsgs[i])
			}

			c.log.Tracef("Reloaded %d messages into cacher",
				len(reloadedMsgs))

			// Reloading only loads old messages but still waits
			// for the online event to make sure messages were
			// received in order.
			if !doneCaching {
				continue loop
			}

		case <-initialDelayChan:
			initialDelayChan = nil
			updtTicker.Reset(c.updateDelay)
			c.log.Tracef("Initial delay elapsed")

		case <-updtTicker.C:
			if doneCaching {
				continue loop
			}
			nb := users.dropStale(c.maxLifetime)
			c.log.Tracef("Dropped %d stale users due to update ticker", nb)

		case <-ctx.Done():
			break loop
		}

		// Emit all GCMs available.
		for msgs.Len() > 0 && users.canEmitGCM(msgs.msgs[0].TS) {
			msg := msgs.nextGCM()
			c.log.Tracef("Emitting GCM from date %s", msg.TS)
			if c.handler != nil {
				c.handler(msg)
			}
		}
		doneCaching = msgs.Len() == 0
		if doneCaching {
			c.log.Tracef("Done caching")
			updtTicker.Stop()
		}
	}

	close(c.quit)
	return ctx.Err()
}
