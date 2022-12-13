package gcmcacher

import (
	"context"
	"sort"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
)

// Msg is an individual message stored by the GC Message cacher.
type Msg struct {
	UID clientintf.UserID
	GCM rpc.RMGroupMessage
	TS  time.Time
}

type gcmq struct {
	msgs []Msg
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

// Cacher caches GC messages fetched immediately after going online to improve
// UX of notifications of new messages.
type Cacher struct {
	delayTimeout    time.Duration
	maxDelayTimeout time.Duration
	handler         func(msgs []Msg)
	log             slog.Logger

	quit          chan struct{}
	msgChan       chan Msg
	connectedChan chan bool
}

// New creates a new GC message cacher. delayTimeout is the time between
// messages which causes the cacher to delay delivering messages. maxDelayTimeout
// is a max delay after which messages will be delivered, regardless of being
// received within delayDuration of each other.
func New(delayTimeout, maxDelayTimeout time.Duration,
	log slog.Logger, handler func([]Msg)) *Cacher {

	c := &Cacher{
		delayTimeout:    delayTimeout,
		maxDelayTimeout: maxDelayTimeout,
		handler:         handler,
		log:             log,

		quit:          make(chan struct{}),
		msgChan:       make(chan Msg),
		connectedChan: make(chan bool),
	}
	return c
}

// GCMessageReceived should be called whenever a new GC message is externally
// received.
func (c *Cacher) GCMessageReceived(uid clientintf.UserID, gcm rpc.RMGroupMessage, ts time.Time) {
	select {
	case c.msgChan <- Msg{UID: uid, GCM: gcm, TS: ts}:
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
	gcs := make(map[zkidentity.ShortID]*gcmq)
	var online bool
	var delayDeadline time.Time
	var timerChan <-chan time.Time

loop:
	for {
		select {
		case msg := <-c.msgChan:
			if timerChan == nil {
				c.log.Tracef("Pushing message from %s in gc %s with ts %s",
					msg.UID, msg.GCM.ID, msg.TS)

				// Initial caching delay elapsed, call
				// message handler directly.
				if c.handler != nil {
					c.handler([]Msg{msg})
				}
				continue
			}

			c.log.Tracef("Delaying message from %s in gc %s with ts %s",
				msg.UID, msg.GCM.ID, msg.TS)

			// We'll need to delay this message. Store in message
			// queue, sorted by timestamp.
			gc := gcs[msg.GCM.ID]
			if gc == nil {
				gc = &gcmq{}
				gcs[msg.GCM.ID] = gc
			}
			gc.msgs = append(gc.msgs, msg)

			// Reset timer chan if the max delay deadline has not
			// passed.
			if time.Now().Before(delayDeadline) {
				timerChan = time.After(c.delayTimeout)
			}

		case online = <-c.connectedChan:
			if !online {
				c.log.Tracef("Gone offline")
			} else {
				delayDeadline = time.Now().Add(c.maxDelayTimeout)
				timerChan = time.After(c.delayTimeout)
				c.log.Tracef("Gone online with delay deadline %s",
					delayDeadline)
			}

		case <-timerChan:
			// Max delay elapsed. Trigger handlers and switch to
			// immediate msg mode.
			c.log.Tracef("Timer triggered with %d gcs with messages",
				len(gcs))
			timerChan = nil
			if c.handler != nil {
				for _, gc := range gcs {
					sort.Sort(gc)
					c.handler(gc.msgs)
				}
			}
			gcs = make(map[zkidentity.ShortID]*gcmq)

		case <-ctx.Done():
			break loop
		}
	}

	close(c.quit)
	return ctx.Err()
}
