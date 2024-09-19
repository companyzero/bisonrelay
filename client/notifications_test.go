package client

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// TestNotificationManager tests the behavior of the NotificationManager.
func TestNotificationManager(t *testing.T) {
	t.Parallel()

	nmgr := NewNotificationManager()
	user := newTestRemoteUser(t, clientintf.UserID{}, "remote")

	var called bool
	var calledChan = make(chan struct{})
	fSync := func() {
		called = true
	}
	fAsync := func() {
		calledChan <- struct{}{}
	}

	assertCalledSync := func(want bool) {
		t.Helper()
		if want != called {
			t.Fatalf("unexpected called sync: got %v, want %v",
				called, want)
		}
		called = false
	}
	assertCalledAsync := func(want bool) {
		t.Helper()
		if want {
			select {
			case <-calledChan:
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for calledChan")
			}
		} else {
			select {
			case <-calledChan:
				t.Fatal("unexpected write to calledChan")
			case <-time.After(time.Millisecond * 100):
			}

		}
	}
	assertUnregister := func(reg NotificationRegistration, want bool) {
		t.Helper()
		got := reg.Unregister()
		if got != want {
			t.Fatalf("unexpected unregister() result: got %v, want %v",
				got, want)
		}
	}

	// No one registered  yet.
	nmgr.notifyOnPM(user, rpc.RMPrivateMessage{}, time.Now())
	assertCalledSync(false)
	assertCalledAsync(false)

	// Register one sync and one async calls.
	regSync := nmgr.RegisterSync(onTestNtfn(fSync))
	regAsync := nmgr.Register(onTestNtfn(fAsync))

	// Both called after registration.
	nmgr.notifyTest()
	assertCalledSync(true)
	assertCalledAsync(true)

	// Unregister only sync and call.
	assertUnregister(regSync, true)
	nmgr.notifyTest()
	assertCalledSync(false)
	assertCalledAsync(true)

	// Unregister async and call.
	assertUnregister(regAsync, true)
	nmgr.notifyTest()
	assertCalledSync(false)
	assertCalledAsync(false)

	// Both already unregistered.
	assertUnregister(regSync, false)
	assertUnregister(regAsync, false)
}

// TestUINotificationBatching tests that UI notifications get batched.
func TestUINotificationBatching(t *testing.T) {
	t.Parallel()

	fromPM := zkidentity.ShortID{31: 0x01}
	fromGC := zkidentity.ShortID{31: 0x02}
	fromMention := zkidentity.ShortID{31: 0x03}
	user := newTestRemoteUser(t, fromPM, "remote")

	nmgr := NewNotificationManager()
	cfg := UINotificationsConfig{
		PMs:           true,
		GCMs:          true,
		GCMMentions:   true,
		MentionRegexp: regexp.MustCompile(`@mynick`),
		EmitInterval:  time.Second,
	}
	nmgr.UpdateUIConfig(cfg)

	uiNtnfChan := make(chan UINotification, 5)
	nmgr.RegisterSync(OnUINotification(func(n UINotification) {
		uiNtnfChan <- n
	}))

	// Send 3 notifications (one of each type).
	nmgr.notifyOnPM(user, rpc.RMPrivateMessage{}, time.Now())
	nmgr.notifyOnGCM(user, rpc.RMGroupMessage{ID: fromGC}, "gc01", time.Now())
	nmgr.notifyOnGCM(user, rpc.RMGroupMessage{ID: fromMention, Message: "@mynick"},
		"gc02", time.Now())

	// Assert chan not written yet (waiting for EmitInterval to elapse).
	assert.ChanNotWritten(t, uiNtnfChan, cfg.EmitInterval/2)

	// Assert only one notification is sent batching 3 messages.
	got := assert.ChanWritten(t, uiNtnfChan)
	assert.DeepEqual(t, got.Count, 3)

	// Assert no other messages sent.
	assert.ChanNotWritten(t, uiNtnfChan, cfg.EmitInterval+time.Millisecond*100)
}

// TestUINotificationCancelEmission tests that UI notifications emission can be
// canceled.
func TestUINotificationCancelEmission(t *testing.T) {
	t.Parallel()

	fromPM := zkidentity.ShortID{31: 0x01}
	user := newTestRemoteUser(t, fromPM, "remote")

	// Use a canceled context that will discard emissions.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	nmgr := NewNotificationManager()
	cfg := UINotificationsConfig{
		PMs:                   true,
		EmitInterval:          time.Second,
		CancelEmissionChannel: ctx.Done(),
	}
	nmgr.UpdateUIConfig(cfg)

	uiNtnfChan := make(chan UINotification, 5)
	nmgr.RegisterSync(OnUINotification(func(n UINotification) {
		uiNtnfChan <- n
	}))

	nmgr.notifyOnPM(user, rpc.RMPrivateMessage{}, time.Now())
	nmgr.notifyOnPM(user, rpc.RMPrivateMessage{}, time.Now())
	nmgr.notifyOnPM(user, rpc.RMPrivateMessage{}, time.Now())

	// No emission of UI notifications.
	assert.ChanNotWritten(t, uiNtnfChan, cfg.EmitInterval+time.Millisecond*100)
}
