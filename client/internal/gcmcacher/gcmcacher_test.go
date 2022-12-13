package gcmcacher

import (
	"fmt"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/slog"
	"golang.org/x/net/context"
)

const (
	testDelay    = 10 * time.Millisecond
	maxTestDelay = 10 * testDelay
)

// testCacher creates a test gcm cacher.
func testCacher(t testing.TB) (*Cacher, chan []Msg) {
	ch := make(chan []Msg)
	handler := func(msgs []Msg) {
		ch <- msgs
	}
	log := slog.Disabled //testutils.TestLoggerSys(t, "GCMC")
	c := New(testDelay, maxTestDelay, log, handler)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { c.Run(ctx) }()

	return c, ch
}

// TestGCMSortsMessages asserts that messages are reordered based on their
// timestamp.
func TestGCMSortsMessages(t *testing.T) {
	c, ch := testCacher(t)

	// Send 5 messages in reverse order.
	c.SessionChanged(true)
	uid := clientintf.UserID{}
	nbMsgs := 5
	ts := time.Now()
	for i := 0; i < nbMsgs; i++ {
		gcm := rpc.RMGroupMessage{Message: fmt.Sprintf("%d", i)}
		c.GCMessageReceived(uid, gcm, ts)
		ts = ts.Add(-time.Second)
	}

	// Assert messages were reordered.
	msgs := assert.ChanWritten(t, ch)
	if len(msgs) != nbMsgs {
		t.Fatalf("unexpected nb of messages: got %d, want %d", len(msgs), nbMsgs)
	}
	for i := 0; i < nbMsgs; i++ {
		wantMsg := fmt.Sprintf("%d", nbMsgs-i-1)
		assert.DeepEqual(t, msgs[i].GCM.Message, wantMsg)
	}
}

// TestGCMMaxDelay tests the cacher only delays up to the max delay time, even
// if multiple messages are still being received.
func TestGCMMaxDelay(t *testing.T) {
	c, ch := testCacher(t)

	// Keep sending messages until the test ends, faster than the
	// inter-message delay.
	c.SessionChanged(true)
	uid := clientintf.UserID{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			select {
			case <-ctx.Done():
			case <-time.After(testDelay / 2):
				c.GCMessageReceived(uid, rpc.RMGroupMessage{}, time.Now())
			}
		}
	}()

	// We expect to still receive callback after maxDelay.
	assert.ChanWritten(t, ch)
}

// TestGCMCMessagesOffline asserts that the handler callback is called even if
// the cacher goes offline after receiving some messages.
func TestGCMCMessagesOffline(t *testing.T) {
	c, ch := testCacher(t)
	uid := clientintf.UserID{}

	// Receive a message and immediately go offline.
	c.SessionChanged(true)
	c.GCMessageReceived(uid, rpc.RMGroupMessage{}, time.Now())
	c.SessionChanged(false)

	// We expect to still receive callback after some delay.
	assert.ChanWritten(t, ch)

}
