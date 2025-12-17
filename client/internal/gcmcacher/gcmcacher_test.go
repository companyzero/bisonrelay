package gcmcacher

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
)

const (
	testUpdateDelay  = 10 * time.Millisecond
	testMaxLifetime  = 10 * testUpdateDelay
	testInitialDelay = 10 * testMaxLifetime
)

// testCacher creates a test gcm cacher.
func testCacher(t testing.TB) (*Cacher, chan clientintf.ReceivedGCMsg) {
	ch := make(chan clientintf.ReceivedGCMsg, 10)
	handler := func(msg clientintf.ReceivedGCMsg) {
		ch <- msg
	}
	log := slog.Disabled
	c := New(testMaxLifetime, testUpdateDelay, testInitialDelay, log, handler)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { c.Run(ctx) }()

	return c, ch
}

// TestGCMSortsMessages asserts that messages are reordered based on their
// timestamp.
func TestGCMSortsMessages(t *testing.T) {
	t.Parallel()
	c, ch := testCacher(t)

	// Send 5 messages in reverse order.
	c.SessionChanged(true)
	uid := clientintf.UserID{}
	nbMsgs := 5
	ts := time.Now()
	for i := 0; i < nbMsgs; i++ {
		gcm := rpc.RMGroupMessage{Message: fmt.Sprintf("%d", i)}
		rgcm := clientintf.ReceivedGCMsg{UID: uid, GCM: gcm, TS: ts}
		c.GCMessageReceived(rgcm)
		ts = ts.Add(-time.Second)
	}

	// Assert messages were reordered.
	for i := 0; i < nbMsgs; i++ {
		gotMsg := assert.ChanWritten(t, ch)
		wantMsg := fmt.Sprintf("%d", nbMsgs-i-1)
		assert.DeepEqual(t, gotMsg.GCM.Message, wantMsg)
	}
}

// TestGCMCMessagesOffline asserts that the handler callback is called even if
// the cacher goes offline after receiving some messages.
func TestGCMCMessagesOffline(t *testing.T) {
	t.Parallel()
	c, ch := testCacher(t)

	// Receive a message and immediately go offline.
	c.SessionChanged(true)
	c.GCMessageReceived(clientintf.ReceivedGCMsg{TS: time.Now()})
	c.SessionChanged(false)

	// We expect to still receive callback after some delay.
	assert.ChanWritten(t, ch)
}

// TestSlowServerConn tests a scenario where the connection to the server is
// slow enough that we're still fetching messages even after the initial
// delay has elapsed, and thus need to reorder based on the rate of received
// messages.
func TestSlowServerConn(t *testing.T) {
	t.Parallel()
	c, ch := testCacher(t)
	uid1 := clientintf.UserID{0: 1}
	uid2 := clientintf.UserID{0: 2}
	uid3 := clientintf.UserID{0: 3}

	basetime := time.Unix(1577851261, 0)
	testTime := func() time.Time {
		basetime = basetime.Add(time.Second)
		return basetime
	}

	// Connected to session. Client starts receiving the initial RMs and
	// GCMs from the server.
	c.SessionChanged(true)
	c.RMReceived(uid1, testTime())
	c.RMReceived(uid3, testTime())
	oldTime := basetime
	c.RMReceived(uid3, testTime())
	midTime := basetime
	c.RMReceived(uid3, testTime())
	c.RMReceived(uid2, testTime())
	wantMsg03 := "msg03"
	c.GCMessageReceived(clientintf.ReceivedGCMsg{UID: uid2, GCM: rpc.RMGroupMessage{Message: wantMsg03}, TS: basetime})

	// Wait until just before the initial delay is about to elapse.
	time.Sleep(testInitialDelay - testMaxLifetime)

	// The server is still sending old RMs, at a rate just slower than
	// MaxLifetime, meaning we might still fetch a message that is newer
	// than oldTime and older than the time for msg03.
	for i := 0; i < 5; i++ {
		c.RMReceived(uid1, oldTime)
		time.Sleep(testMaxLifetime - testUpdateDelay*2)
	}

	// msg01 appears, which is older than msg02.
	wantMsg01 := "msg01"
	c.RMReceived(uid1, oldTime)
	c.GCMessageReceived(clientintf.ReceivedGCMsg{UID: uid1, GCM: rpc.RMGroupMessage{Message: wantMsg01}, TS: oldTime})

	// msg01 is immediately dispatched (because we can receive no message
	// older than that).
	gotMsg01 := assert.ChanWritten(t, ch)
	assert.DeepEqual(t, gotMsg01.GCM.Message, wantMsg01)

	// Test that delaying still doesn't cause msgs to be dispatched.
	for i := 0; i < 5; i++ {
		c.RMReceived(uid1, midTime)
		time.Sleep(testMaxLifetime - testUpdateDelay*2)
	}

	// msg02 appears, which is older than msg03 but newer than msg01.
	wantMsg02 := "msg02"
	c.RMReceived(uid1, midTime)
	c.GCMessageReceived(clientintf.ReceivedGCMsg{UID: uid1, GCM: rpc.RMGroupMessage{Message: wantMsg02}, TS: midTime})

	// Wait for the timeouts to elapse and verify correct ordering.
	gotMsg02 := assert.ChanWritten(t, ch)
	assert.DeepEqual(t, gotMsg02.GCM.Message, wantMsg02)
	gotMsg03 := assert.ChanWritten(t, ch)
	assert.DeepEqual(t, gotMsg03.GCM.Message, wantMsg03)
}

// TestEmitsReloadedWhileOffline asserts that messages reloaded while the
// cacher was offline are eventually emitted.
func TestEmitsReloadedWhileOffline(t *testing.T) {
	t.Parallel()
	c, ch := testCacher(t)

	// Reload a cached message before the session is online.
	rgcm := clientintf.ReceivedGCMsg{
		MsgID: zkidentity.ShortID{0: 0x01},
		UID:   clientintf.UserID{0: 0x02},
		GCM:   rpc.RMGroupMessage{Message: "test"},
		TS:    time.Now(),
	}
	c.ReloadCachedMessages([]clientintf.ReceivedGCMsg{rgcm})

	// Session goes online.
	c.SessionChanged(true)

	// We expect to still receive the callback after some delay.
	assert.ChanWrittenWithVal(t, ch, rgcm)
}

// TestEmitsReloadedWhenOnline asserts that messages reloaded while the
// cacher was online are eventually emitted.
func TestEmitsReloadedWhenOnline(t *testing.T) {
	t.Parallel()
	c, ch := testCacher(t)

	// Session goes online.
	c.SessionChanged(true)
	time.Sleep(testInitialDelay + time.Millisecond*10)

	// Reload a cached message before the session is online.
	rgcm := clientintf.ReceivedGCMsg{
		MsgID: zkidentity.ShortID{0: 0x01},
		UID:   clientintf.UserID{0: 0x02},
		GCM:   rpc.RMGroupMessage{Message: "test"},
		TS:    time.Now(),
	}
	c.ReloadCachedMessages([]clientintf.ReceivedGCMsg{rgcm})

	// We expect to still receive the callback after some delay.
	assert.ChanWrittenWithVal(t, ch, rgcm)
}
