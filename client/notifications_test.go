package client

import (
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
)

// TestNotificationManager tests the behavior of the NotificationManager.
func TestNotificationManager(t *testing.T) {
	nmgr := NewNotificationManager()

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
	nmgr.notifyOnPM(nil, rpc.RMPrivateMessage{}, time.Now())
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
