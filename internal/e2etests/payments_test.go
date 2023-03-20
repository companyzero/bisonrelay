package e2etests

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
)

// TestTipUser asserts that attempting to tip an user works when payments go
// through.
func TestTipUser(t *testing.T) {
	t.Parallel()
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	// Setup tip progress handler.
	progressErrChan := make(chan error, 1)
	alice.handle(client.OnTipAttemptProgressNtfn(func(ru *client.RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
		progressErrChan <- attemptErr
	}))

	// Generate a custom test invoice.
	payMAtoms := int64(4321000)
	bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
		if amt == payMAtoms {
			return "custom invoice", nil
		}
		return fmt.Sprintf("invoice for %d", amt), nil
	})
	alice.mpc.HookDecodeInvoice(func(invoice string) (clientintf.DecodedInvoice, error) {
		if invoice == "custom invoice" {
			inv, _ := alice.mpc.DefaultDecodeInvoice(invoice)
			inv.MAtoms = payMAtoms
			return inv, nil
		}
		return alice.mpc.DefaultDecodeInvoice(invoice)
	})

	// Send a tip from Alice to Bob. Sending should succeed and we should
	// get a completed progress event.
	const maxAttempts = 1
	err := alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)
	assert.NilErrFromChan(t, progressErrChan)
}

// TestTipUserRejectsWrongInvoiceAmount asserts that if the remote client sends
// an invoice for an amount different than the expected one, payment is not
// attempted.
func TestTipUserRejectsWrongInvoiceAmount(t *testing.T) {
	t.Parallel()
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	// Setup tip progress handler.
	progressErrChan := make(chan error, 1)
	alice.handle(client.OnTipAttemptProgressNtfn(func(ru *client.RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
		progressErrChan <- attemptErr
	}))

	// Generate a custom test invoice.
	var mtx sync.Mutex
	payMAtoms := int64(4321000)
	payMAtomsDelta := int64(-1)
	bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
		if amt == payMAtoms {
			return "custom invoice", nil
		}
		return fmt.Sprintf("invoice for %d", amt), nil
	})
	alice.mpc.HookDecodeInvoice(func(invoice string) (clientintf.DecodedInvoice, error) {
		mtx.Lock()
		defer mtx.Unlock()
		if invoice == "custom invoice" {
			inv, _ := alice.mpc.DefaultDecodeInvoice(invoice)
			inv.MAtoms = payMAtoms + payMAtomsDelta
			return inv, nil
		}
		return alice.mpc.DefaultDecodeInvoice(invoice)
	})
	payInvoiceChan := make(chan struct{}, 10)
	alice.mpc.HookPayInvoice(func(string) (int64, error) {
		payInvoiceChan <- struct{}{}
		return 0, nil
	})

	// Helper to test error received in progressErrChan. Assertion is made
	// by string because the error travels through the server.
	assertAliceProgrChanErrored := func(wantErr error) {
		t.Helper()
		err := assert.ChanWritten(t, progressErrChan)
		if err == nil || err.Error() != wantErr.Error() {
			t.Fatalf("Unexpected error: got %v, want %v", err, wantErr)
		}
	}

	// Send a tip from Alice to Bob. Sending should fail due to invoice
	// amount less than the expected.
	const maxAttempts = 1
	err := alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)
	assertAliceProgrChanErrored(errors.New("milliatoms requested in " +
		"invoice (4320999) different than milliatoms originally requested (4321000)"))
	assert.ChanNotWritten(t, payInvoiceChan, time.Millisecond*500)

	// Attempt tip again, this time with an invoice that is larger than the
	// expected.
	mtx.Lock()
	payMAtomsDelta = +1
	mtx.Unlock()
	err = alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)
	assertAliceProgrChanErrored(errors.New("milliatoms requested in " +
		"invoice (4321001) different than milliatoms originally requested (4321000)"))

	assert.ChanNotWritten(t, payInvoiceChan, time.Millisecond*500)
}

// TestTipUserMultipleAttempts asserts that tipping works when multiple attempts
// are needed between online clients.
func TestTipUserMultipleAttempts(t *testing.T) {
	t.Parallel()
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	const maxAttempts = 3
	var mtx sync.Mutex
	payMAtoms := int64(4321000)

	// Setup Bob hook helper func.
	getInvoiceErr := errors.New("failed to get invoice")
	bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
		mtx.Lock()
		defer mtx.Unlock()
		if amt == payMAtoms {
			return "custom invoice", getInvoiceErr
		}
		return fmt.Sprintf("invoice for %d", amt), nil
	})

	// Setup Alice hook helper func.
	progressErrChan := make(chan error, 10)
	payInvoiceErr := errors.New("failed to pay invoice")
	alice.handleSync(client.OnTipAttemptProgressNtfn(func(ru *client.RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
		if completed && attemptErr != nil {
			attemptErr = fmt.Errorf("got completed flag with err %v", attemptErr)
		}
		if amtMAtoms != payMAtoms {
			attemptErr = fmt.Errorf("wrong amount of MAtoms: got %v, want %v",
				amtMAtoms, payMAtoms)
		}
		if attempt == maxAttempts && willRetry {
			attemptErr = errors.New("got attempts == maxAttempts with will flag")
		}
		if !completed && attempt < maxAttempts && !willRetry {
			attemptErr = errors.New("got attempts < maxAttempts without willRetry flag")
		}

		progressErrChan <- attemptErr
	}))

	alice.mpc.HookPayInvoice(func(invoice string) (int64, error) {
		mtx.Lock()
		defer mtx.Unlock()
		if invoice == "custom invoice" {
			return 0, payInvoiceErr
		}
		return 0, nil
	})
	alice.mpc.HookIsPayCompleted(func(invoice string) (int64, error) {
		mtx.Lock()
		defer mtx.Unlock()
		if invoice == "custom invoice" {
			return 0, payInvoiceErr
		}
		return 0, nil
	})
	alice.mpc.HookDecodeInvoice(func(invoice string) (clientintf.DecodedInvoice, error) {
		if invoice == "custom invoice" {
			inv, _ := alice.mpc.DefaultDecodeInvoice(invoice)
			inv.MAtoms = payMAtoms
			return inv, nil
		}
		return alice.mpc.DefaultDecodeInvoice(invoice)
	})

	// The first test will make multiple attempts, but will never receive a
	// valid invoice.
	err := alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)
	for i := 0; i < maxAttempts; i++ {
		err := assert.ChanWritten(t, progressErrChan)
		if err == nil || err.Error() != rpc.ErrUnableToGenerateInvoice.Error() {
			t.Fatalf("Unexpected error: got %v, want %v", err, rpc.ErrUnableToGenerateInvoice)
		}
	}

	// After maxAttempts, there should not be an additional attempt.
	assert.ChanNotWritten(t, progressErrChan, alice.cfg.TipUserReRequestInvoiceDelay*3)

	// Same test, but this time failing payment instead of invoice
	// generation.
	mtx.Lock()
	getInvoiceErr = nil
	mtx.Unlock()
	err = alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)
	for i := 0; i < maxAttempts; i++ {
		err := assert.ChanWritten(t, progressErrChan)
		if err == nil || err.Error() != payInvoiceErr.Error() {
			t.Fatalf("Unexpected error: got %v, want %v", err, rpc.ErrUnableToGenerateInvoice)
		}
	}
	assert.ChanNotWritten(t, progressErrChan, alice.cfg.TipUserReRequestInvoiceDelay*3)

	// Final test: make the payment succeed.
	mtx.Lock()
	payInvoiceErr = nil
	mtx.Unlock()
	err = alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)
	assert.NilErrFromChan(t, progressErrChan)
}

// TestTipUserWithRestarts asserts that tipping works even when multiple
// attempts are needed and the clients are restarted.
func TestTipUserWithRestarts(t *testing.T) {
	t.Parallel()
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	const maxAttempts = 3
	var mtx sync.Mutex
	payMAtoms := int64(4321000)

	// Setup Bob hook helper func.
	getInvoiceErr := errors.New("failed to get invoice")
	bobSentInvoice := make(chan struct{}, 10)
	hookBob := func() {
		bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
			mtx.Lock()
			defer mtx.Unlock()
			if amt == payMAtoms {
				if bobSentInvoice != nil {
					bobSentInvoice <- struct{}{}
				}
				return "custom invoice", getInvoiceErr
			}
			return fmt.Sprintf("invoice for %d", amt), nil
		})
	}

	// Setup Alice hook helper func.
	progressErrChan := make(chan error, 10)
	payInvoiceErr := errors.New("failed to pay invoice")
	hookAlice := func() {
		alice.handleSync(client.OnTipAttemptProgressNtfn(func(ru *client.RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
			progressErrChan <- attemptErr
		}))

		alice.mpc.HookPayInvoice(func(invoice string) (int64, error) {
			mtx.Lock()
			defer mtx.Unlock()
			if invoice == "custom invoice" {
				return 0, payInvoiceErr
			}
			return 0, nil
		})
		alice.mpc.HookIsPayCompleted(func(invoice string) (int64, error) {
			mtx.Lock()
			defer mtx.Unlock()
			if invoice == "custom invoice" {
				return 0, payInvoiceErr
			}
			return 0, nil
		})
		alice.mpc.HookDecodeInvoice(func(invoice string) (clientintf.DecodedInvoice, error) {
			if invoice == "custom invoice" {
				inv, _ := alice.mpc.DefaultDecodeInvoice(invoice)
				inv.MAtoms = payMAtoms
				return inv, nil
			}
			return alice.mpc.DefaultDecodeInvoice(invoice)
		})
	}

	// Helper to test error received in progressErrChan. Assertion is made
	// by string because the error travels through the server.
	assertAliceProgrChanErrored := func(wantErr error) {
		err := assert.ChanWritten(t, progressErrChan)
		if err == nil || err.Error() != wantErr.Error() {
			t.Fatalf("Unexpected error: got %v, want %v", err, wantErr)
		}
	}

	// Test restart scenarios now.
	//   1. Bob goes offline
	//   2. Alice asks for invoice. Alice goes offline.
	//   3. Bob comes online. Fails invoice generation. Goes offline.
	//   4. Alice comes online. Requests invoice again. Goes offline.
	//   5. Bob comes online. Sends invoice. Goes offline.
	//   6. Alice comes online. Invoice payment fail. Requests new invoice. Goes offline.
	//   7. Bob comes online. Sends invoice. Goes offline.
	//   9. Alice successfully pays for invoice.
	mtx.Lock()
	getInvoiceErr = errors.New("failed to get invoice")
	mtx.Unlock()

	// (1)
	ts.stopClient(bob)

	// (2)
	err := alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)
	ts.stopClient(alice)

	// (3)
	bob = ts.recreateStoppedClient(bob)
	hookBob()
	assert.ChanWritten(t, bobSentInvoice)
	time.Sleep(time.Millisecond * 250) // Wait msg to be sent
	ts.stopClient(bob)

	// (4)
	alice = ts.recreateStoppedClient(alice)
	hookAlice()
	assertAliceProgrChanErrored(rpc.ErrUnableToGenerateInvoice)
	time.Sleep(alice.cfg.TipUserReRequestInvoiceDelay + 250*time.Millisecond) // Wait to ask for new invoice
	ts.stopClient(alice)

	// (5)
	mtx.Lock()
	getInvoiceErr = nil
	mtx.Unlock()
	bob = ts.recreateStoppedClient(bob)
	hookBob()
	assert.ChanWritten(t, bobSentInvoice)
	time.Sleep(time.Millisecond * 250) // Wait msg to be sent
	ts.stopClient(bob)

	// (6)
	alice = ts.recreateStoppedClient(alice)
	hookAlice()
	assertAliceProgrChanErrored(payInvoiceErr)
	time.Sleep(alice.cfg.TipUserReRequestInvoiceDelay + 250*time.Millisecond) // Wait to ask for new invoice
	ts.stopClient(alice)

	// (7)
	bob = ts.recreateStoppedClient(bob)
	hookBob()
	assert.ChanWritten(t, bobSentInvoice)
	time.Sleep(time.Millisecond * 250) // Wait msg to be sent
	ts.stopClient(bob)

	// (8)
	mtx.Lock()
	payInvoiceErr = nil
	mtx.Unlock()
	alice = ts.recreateStoppedClient(alice)
	hookAlice()
	assert.NilErrFromChan(t, progressErrChan)
	time.Sleep(alice.cfg.TipUserReRequestInvoiceDelay + 250*time.Millisecond) // Wait to ask for new invoice
}

// TestTipUserRestartNoDoublePay checks that even if multiple attempts to fetch
// an invoice are made, they do not cause a double attempt at paying for the
// same tip.
func TestTipUserRestartNoDoublePay(t *testing.T) {
	t.Parallel()
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	const maxAttempts = 3
	var mtx sync.Mutex
	payMAtoms := int64(4321000)

	// Setup Bob hook helper func.
	bobSentInvoice := make(chan struct{}, 10)
	hookBob := func() {
		bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
			mtx.Lock()
			defer mtx.Unlock()
			if amt == payMAtoms {
				if bobSentInvoice != nil {
					bobSentInvoice <- struct{}{}
				}
				return "custom invoice", nil
			}
			return fmt.Sprintf("invoice for %d", amt), nil
		})
	}

	// Setup Alice hook helper func.
	progressErrChan := make(chan error, 10)
	alicePayChan := make(chan struct{}, 10)
	hookAlice := func() {
		alice.handleSync(client.OnTipAttemptProgressNtfn(func(ru *client.RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
			progressErrChan <- attemptErr
		}))

		alice.mpc.HookPayInvoice(func(invoice string) (int64, error) {
			mtx.Lock()
			defer mtx.Unlock()
			if invoice == "custom invoice" {
				alicePayChan <- struct{}{}
				return 0, nil
			}
			return 0, nil
		})
		alice.mpc.HookIsPayCompleted(func(invoice string) (int64, error) {
			mtx.Lock()
			defer mtx.Unlock()
			if invoice == "custom invoice" {
				return 0, fmt.Errorf("pay is not completed")
			}
			return 0, nil
		})
		alice.mpc.HookDecodeInvoice(func(invoice string) (clientintf.DecodedInvoice, error) {
			if invoice == "custom invoice" {
				inv, _ := alice.mpc.DefaultDecodeInvoice(invoice)
				inv.MAtoms = payMAtoms
				return inv, nil
			}
			return alice.mpc.DefaultDecodeInvoice(invoice)
		})
	}

	resendDelay := alice.cfg.TipUserRestartDelay + alice.cfg.TipUserReRequestInvoiceDelay +
		time.Millisecond*250

	// Test restart scenarios now.
	//   1. Bob goes offline
	//   2. Alice asks for invoice. Alice goes offline.
	//   3. (2x) Alice comes online, sends request for new invoice, goes offline.
	//   4. Bob comes online, sends multiple invoices, goes offline.
	//   5. Alice comes online, only pays a single invoice.

	// (1)
	ts.stopClient(bob)

	// (2)
	err := alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)
	ts.stopClient(alice)

	// (3)
	for i := 0; i < 2; i++ {
		alice = ts.recreateStoppedClient(alice)
		hookAlice()
		time.Sleep(resendDelay) // Wait to ask for new invoice
		ts.stopClient(alice)
	}

	// (4)
	bob = ts.recreateStoppedClient(bob)
	hookBob()
	assert.ChanWritten(t, bobSentInvoice)
	assert.ChanWritten(t, bobSentInvoice)
	assert.ChanWritten(t, bobSentInvoice)
	time.Sleep(time.Millisecond * 250) // Wait msgs to be sent
	ts.stopClient(bob)

	// (5)
	alice = ts.recreateStoppedClient(alice)
	hookAlice()
	assert.ChanWritten(t, alicePayChan)
	assert.ChanNotWritten(t, alicePayChan, resendDelay)
	assert.NilErrFromChan(t, progressErrChan)
	assert.ChanNotWritten(t, progressErrChan, resendDelay)
}

func TestTipUserExceedsLifetime(t *testing.T) {
	t.Parallel()
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	// Setup tip progress handler.
	const maxAttempts = 3
	progressErrChan := make(chan error, 1)
	alice.handle(client.OnTipAttemptProgressNtfn(func(ru *client.RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
		if willRetry {
			progressErrChan <- fmt.Errorf("progress should not have failed with willRetry set")
		} else if attempt != maxAttempts {
			progressErrChan <- fmt.Errorf("attempt %d != max attempts %d",
				attempt, maxAttempts)
		} else {
			progressErrChan <- attemptErr

		}
	}))

	// Generate a custom test invoice.
	var mtx sync.Mutex
	payMAtoms := int64(4321000)
	bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
		// Delay invoice generation until the max lifetime elapses on
		// Alice.
		time.Sleep(alice.cfg.TipUserMaxLifetime)
		if amt == payMAtoms {
			return "custom invoice", nil
		}
		return fmt.Sprintf("invoice for %d", amt), nil
	})
	alice.mpc.HookDecodeInvoice(func(invoice string) (clientintf.DecodedInvoice, error) {
		mtx.Lock()
		defer mtx.Unlock()
		if invoice == "custom invoice" {
			inv, _ := alice.mpc.DefaultDecodeInvoice(invoice)
			inv.MAtoms = payMAtoms
			return inv, nil
		}
		return alice.mpc.DefaultDecodeInvoice(invoice)
	})
	payInvoiceChan := make(chan struct{}, 10)
	alice.mpc.HookPayInvoice(func(string) (int64, error) {
		payInvoiceChan <- struct{}{}
		return 0, nil
	})

	// Send a tip from Alice to Bob. Sending should fail because the invoice
	// will take too long to be generated/received.
	err := alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)

	// Check for correct error. Needs to be done as a string because the
	// message is variable.
	gotErr := assert.ChanWritten(t, progressErrChan)
	if gotErr == nil {
		t.Fatalf("nil error on progressErrChan")
	}
	if !strings.HasSuffix(gotErr.Error(), "which is greater than max lifetime 10s") {
		t.Fatalf("unexpected error: %v", gotErr)
	}
	assert.ChanNotWritten(t, payInvoiceChan, time.Millisecond*500)
}

// TestTipUserPaysOnceWithSlowPayment tests that only a single payment is
// initiated if multiple invoices are received while a payment is already
// in-flight.
func TestTipUserPaysOnceWithSlowPayment(t *testing.T) {
	t.Parallel()
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	const maxAttempts = 3
	var mtx sync.Mutex
	payMAtoms := int64(4321000)

	// Setup Bob hook helper func.
	bobSentInvoice := make(chan struct{}, 10)
	hookBob := func() {
		bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
			mtx.Lock()
			defer mtx.Unlock()
			if amt == payMAtoms {
				if bobSentInvoice != nil {
					bobSentInvoice <- struct{}{}
				}
				return "custom invoice", nil
			}
			return fmt.Sprintf("invoice for %d", amt), nil
		})
	}

	// Setup Alice hook helper func.
	progressErrChan := make(chan error, 10)
	alicePayChan := make(chan struct{}, 10)
	hookAlice := func() {
		alice.handleSync(client.OnTipAttemptProgressNtfn(func(ru *client.RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
			progressErrChan <- attemptErr
		}))

		alice.mpc.HookPayInvoice(func(invoice string) (int64, error) {
			// This payment will take a while to complete. This is
			// timed so that alice will start processing all three
			// RMInvoice requests before the first payment completes,
			// if in fact multiple payments are attempted.
			time.Sleep(time.Millisecond * 500)
			mtx.Lock()
			defer mtx.Unlock()
			if invoice == "custom invoice" {
				alicePayChan <- struct{}{}
				return 0, nil
			}
			return 0, nil
		})
		alice.mpc.HookIsPayCompleted(func(invoice string) (int64, error) {
			mtx.Lock()
			defer mtx.Unlock()
			if invoice == "custom invoice" {
				return 0, fmt.Errorf("pay is not completed")
			}
			return 0, nil
		})
		alice.mpc.HookDecodeInvoice(func(invoice string) (clientintf.DecodedInvoice, error) {
			if invoice == "custom invoice" {
				inv, _ := alice.mpc.DefaultDecodeInvoice(invoice)
				inv.MAtoms = payMAtoms
				return inv, nil
			}
			return alice.mpc.DefaultDecodeInvoice(invoice)
		})
	}

	resendDelay := alice.cfg.TipUserRestartDelay + alice.cfg.TipUserReRequestInvoiceDelay +
		time.Millisecond*250

	// Test restart scenarios now.
	//   1. Bob goes offline
	//   2. Alice asks for invoice. Alice goes offline.
	//   3. (2x) Alice comes online, sends request for new invoice, goes offline.
	//   4. Bob comes online, sends multiple invoices, goes offline.
	//   5. Alice comes online, attempts only a single payment.

	// (1)
	ts.stopClient(bob)

	// (2)
	err := alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)
	ts.stopClient(alice)

	// (3)
	for i := 0; i < 2; i++ {
		alice = ts.recreateStoppedClient(alice)
		hookAlice()
		time.Sleep(resendDelay) // Wait to ask for new invoice
		ts.stopClient(alice)
	}

	// (4)
	bob = ts.recreateStoppedClient(bob)
	hookBob()
	assert.ChanWritten(t, bobSentInvoice)
	assert.ChanWritten(t, bobSentInvoice)
	assert.ChanWritten(t, bobSentInvoice)
	time.Sleep(time.Millisecond * 250) // Wait msgs to be sent
	ts.stopClient(bob)

	// (5)
	alice = ts.recreateStoppedClient(alice)
	hookAlice()
	assert.ChanWritten(t, alicePayChan)                    // Successful payment
	assert.ChanNotWritten(t, alicePayChan, resendDelay)    // No more payments attempted
	assert.NilErrFromChan(t, progressErrChan)              // Success ntfn
	assert.ChanNotWritten(t, progressErrChan, resendDelay) // No more ntfns
}
