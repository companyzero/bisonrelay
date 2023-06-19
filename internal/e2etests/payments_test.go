package e2etests

import (
	"errors"
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/internal/testutils"
	"github.com/companyzero/bisonrelay/rpc"
)

// TestTipUserExceedsLifetime asserts that if the max lifetime of the tip
// expires, the tip attempt is dropped.
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
		} else if attempt != 1 {
			progressErrChan <- fmt.Errorf("attempt %d != %d",
				attempt, 1)
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
	errRe := regexp.MustCompile(`expired [0-9.nmus]+ after creation`)
	if !errRe.MatchString(gotErr.Error()) {
		t.Fatalf("unexpected error: %v", gotErr)
	}
	assert.ChanNotWritten(t, payInvoiceChan, time.Millisecond*500)
}

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
			attemptErr = errors.New("got attempts <= maxAttempts without willRetry flag")
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

	// Test scenario.
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
	time.Sleep(50 * time.Millisecond)
	assertEmptyRMQ(t, alice)
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
	alice.waitTippingSubsysRunning(alice.cfg.TipUserReRequestInvoiceDelay + 250*time.Millisecond) // Wait to ask for new invoice
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
	alice.waitTippingSubsysRunning(alice.cfg.TipUserReRequestInvoiceDelay + 250*time.Millisecond) // Wait to ask for new invoice
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
	var invoiceTag uint32

	// Setup Bob hook helper func.
	bobRecvInvoiceReq, bobSendInvoice := make(chan struct{}, 10), make(chan struct{}, 10)
	bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
		if amt == payMAtoms {
			return "custom invoice", nil
		}
		return fmt.Sprintf("invoice for %d", amt), nil
	})
	bob.handleSync(client.OnTipUserInvoiceGeneratedNtfn(func(ru *client.RemoteUser, tag uint32, inv string) {
		mtx.Lock()
		invoiceTag = tag
		mtx.Unlock()
		bobRecvInvoiceReq <- struct{}{}
		<-bobSendInvoice
	}))

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

	// Test scenario.
	//   1. Alice asks for invoice. Alice goes offline.
	//   2. Bob sends invoice
	//   3. Simulate bob sending multiple invoices (erroneously). Bob goes offline.
	//   4. Alice comes online, only pays a single invoice.

	// (1)
	err := alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)
	aliceID := alice.PublicID()

	// (2)
	assert.ChanWritten(t, bobRecvInvoiceReq)
	ts.stopClient(alice)
	bobSendInvoice <- struct{}{}

	// (3)
	mtx.Lock()
	rm := rpc.RMInvoice{Tag: invoiceTag, Invoice: "custom invoice"}
	mtx.Unlock()
	bobTI := bob.testInterface()
	bobTI.SendUserRM(aliceID, rm)
	bobTI.SendUserRM(aliceID, rm)
	time.Sleep(time.Millisecond * 250) // Wait msgs to be sent
	assertEmptyRMQ(t, bob)
	ts.stopClient(bob)

	// (4)
	alice = ts.recreateStoppedClient(alice)
	hookAlice()
	assert.ChanWritten(t, alicePayChan)
	assert.ChanNotWritten(t, alicePayChan, resendDelay)
	assert.NilErrFromChan(t, progressErrChan)
	assert.ChanNotWritten(t, progressErrChan, resendDelay)
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
	var invoiceTag uint32

	// Setup Bob hook helper func.
	bobSentInvoice := make(chan struct{})
	bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
		if amt == payMAtoms {
			return "custom invoice", nil
		}
		return fmt.Sprintf("invoice for %d", amt), nil
	})
	bob.handleSync(client.OnTipUserInvoiceGeneratedNtfn(func(ru *client.RemoteUser, tag uint32, inv string) {
		mtx.Lock()
		invoiceTag = tag
		mtx.Unlock()
		bobSentInvoice <- struct{}{}
	}))

	// Setup Alice hook helper func.
	progressErrChan := make(chan error, 10)
	alicePayingChan, alicePaidChan := make(chan struct{}, 10), make(chan struct{}, 10)
	alice.handleSync(client.OnTipAttemptProgressNtfn(func(ru *client.RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
		progressErrChan <- attemptErr
	}))

	alice.mpc.HookPayInvoice(func(invoice string) (int64, error) {
		// This payment will take a while to complete. This is
		// timed so that alice will start processing all three
		// RMInvoice requests before the first payment completes,
		// if in fact multiple payments are attempted.
		if invoice == "custom invoice" {
			alicePayingChan <- struct{}{}
		}
		time.Sleep(time.Millisecond * 500)
		if invoice == "custom invoice" {
			alicePaidChan <- struct{}{}
			return 0, nil
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

	resendDelay := alice.cfg.TipUserRestartDelay + alice.cfg.TipUserReRequestInvoiceDelay +
		time.Millisecond*250

	// Test scenario.
	//   1. Alice asks for invoice
	//   2. Bob sends correct invoice
	//   3. Alice accepts and starts payment
	//   4. Bob sends multiple additional invoices
	//   5. Alice completes payment but does not attempt others

	// (1)
	err := alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)

	// (2)
	assert.ChanWritten(t, bobSentInvoice)

	// (3)
	assert.ChanWritten(t, alicePayingChan)

	// (4)
	mtx.Lock()
	rm := rpc.RMInvoice{Tag: invoiceTag, Invoice: "custom invoice"}
	mtx.Unlock()
	bobTI := bob.testInterface()
	bobTI.SendUserRM(alice.PublicID(), rm)
	bobTI.SendUserRM(alice.PublicID(), rm)

	// (5)
	assert.ChanWritten(t, alicePaidChan)                   // Successful payment
	assert.ChanNotWritten(t, alicePaidChan, resendDelay)   // No more payments attempted
	assert.NilErrFromChan(t, progressErrChan)              // Success ntfn
	assert.ChanNotWritten(t, progressErrChan, resendDelay) // No more ntfns
}

// TestTipUserInvoiceAfterPayment asserts that sending and invoice for the same
// tip attempt after the payment completes does not trigger a new payment.
func TestTipUserInvoiceAfterPayment(t *testing.T) {
	t.Parallel()
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	const maxAttempts = 3
	var mtx sync.Mutex
	payMAtoms := int64(4321000)
	var invoiceTag uint32

	// Setup Bob hook helper func.
	bobSentInvoice := make(chan struct{})
	bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
		if amt == payMAtoms {
			return "custom invoice", nil
		}
		return fmt.Sprintf("invoice for %d", amt), nil
	})
	bob.handleSync(client.OnTipUserInvoiceGeneratedNtfn(func(ru *client.RemoteUser, tag uint32, inv string) {
		mtx.Lock()
		invoiceTag = tag
		mtx.Unlock()
		bobSentInvoice <- struct{}{}
	}))

	// Setup Alice hook helper func.
	progressErrChan := make(chan error, 10)
	alicePaidChan := make(chan struct{}, 10)
	alice.handleSync(client.OnTipAttemptProgressNtfn(func(ru *client.RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
		progressErrChan <- attemptErr
	}))

	alice.mpc.HookPayInvoice(func(invoice string) (int64, error) {
		if invoice == "custom invoice" {
			alicePaidChan <- struct{}{}
			return 0, nil
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

	resendDelay := alice.cfg.TipUserRestartDelay + alice.cfg.TipUserReRequestInvoiceDelay +
		time.Millisecond*250

	// Test scenario.
	//   1. Alice asks for invoice
	//   2. Bob sends correct invoice
	//   3. Alice accepts and completes payment
	//   4. Bob sends additional invoice
	//   5. Alice does not attempt second payment

	// (1)
	err := alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)

	// (2)
	assert.ChanWritten(t, bobSentInvoice)

	// (3)
	assert.ChanWritten(t, alicePaidChan)      // Successful payment
	assert.NilErrFromChan(t, progressErrChan) // Success ntfn

	// (4)
	mtx.Lock()
	rm := rpc.RMInvoice{Tag: invoiceTag, Invoice: "custom invoice"}
	mtx.Unlock()
	bobTI := bob.testInterface()
	bobTI.SendUserRM(alice.PublicID(), rm)

	// (5)
	assert.ChanNotWritten(t, alicePaidChan, resendDelay)   // No more payments attempted
	assert.ChanNotWritten(t, progressErrChan, resendDelay) // No more ntfns
}

// TestTipUserSerialSameUserAttempts asserts that sending multiple TipUser
// attempts to the same user happen in series instead of in parallel.
func TestTipUserSerialSameUserAttempts(t *testing.T) {
	t.Parallel()
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	const maxAttempts = 3
	payMAtoms := int64(4321000)

	// Setup Bob hook helper func.
	bobSentInvoice := make(chan struct{})
	bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
		if amt == payMAtoms {
			return "custom invoice", nil
		}
		return fmt.Sprintf("invoice for %d", amt), nil
	})
	bob.handleSync(client.OnTipUserInvoiceGeneratedNtfn(func(ru *client.RemoteUser, tag uint32, inv string) {
		bobSentInvoice <- struct{}{}
	}))

	// Setup Alice hook helper func.
	progressErrChan := make(chan error, 10)
	alicePaidChan := make(chan struct{})
	alice.handleSync(client.OnTipAttemptProgressNtfn(func(ru *client.RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
		progressErrChan <- attemptErr
	}))

	alice.mpc.HookPayInvoice(func(invoice string) (int64, error) {
		if invoice == "custom invoice" {
			alicePaidChan <- struct{}{}
			return 0, nil
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

	// Test scenario.
	//   1. 2x Alice starts tip attempt.
	//   2. Bob sends correct invoice
	//   3. Alice accepts and completes payment
	//   4. Bob sends second invoice
	//   5. Alice accepts and completes payment

	// (1)
	err := alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)
	err = alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)

	// (2) (only one invoice received so far)
	assert.ChanWritten(t, bobSentInvoice)
	assert.ChanNotWritten(t, bobSentInvoice, time.Second)

	// (3)
	assert.ChanWritten(t, alicePaidChan)
	assert.NilErrFromChan(t, progressErrChan) // Success ntfn

	// (4) (second invoice received)
	assert.ChanWritten(t, bobSentInvoice)

	// (5)
	assert.ChanWritten(t, alicePaidChan)      // Successful payment
	assert.NilErrFromChan(t, progressErrChan) // Success ntfn
}

// TestTipUserRetriesRetriablePayFailure asserts that when the error that caused
// a tip failure is retriable, the payment will be retried.
func TestTipUserRetriesRetriablePayFailure(t *testing.T) {
	t.Parallel()
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	const maxAttempts = 1
	var mtx sync.Mutex
	payMAtoms := int64(4321000)

	// Setup Bob hook helper func.
	bobSentInvoice := make(chan struct{}, 10)
	bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
		if amt == payMAtoms {
			return "custom invoice", nil
		}
		return fmt.Sprintf("invoice for %d", amt), nil
	})
	bob.handleSync(client.OnTipUserInvoiceGeneratedNtfn(func(ru *client.RemoteUser, tag uint32, inv string) {
		bobSentInvoice <- struct{}{}
	}))

	// Setup Alice hook helper func.
	var payInvoiceErr error
	progressErrChan := make(chan error, 10)
	alicePaidChan, alicePayFailedChan := make(chan struct{}, 10), make(chan struct{}, 10)
	hookAlice := func() {
		alice.handleSync(client.OnTipAttemptProgressNtfn(func(ru *client.RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
			progressErrChan <- attemptErr
		}))

		alice.mpc.HookPayInvoice(func(invoice string) (int64, error) {
			mtx.Lock()
			defer mtx.Unlock()
			if invoice == "custom invoice" {
				if payInvoiceErr == nil {
					alicePaidChan <- struct{}{}
				} else {
					alicePayFailedChan <- struct{}{}
				}
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

	resendDelay := alice.cfg.TipUserRestartDelay + alice.cfg.TipUserReRequestInvoiceDelay +
		time.Millisecond*250

	// Test scenario.
	//   1. Alice asks for invoice
	//   2. Bob sends correct invoice
	//   3. Alice accepts and starts payment
	//   4. 2x Payment fails with retriable error
	//   5. Alice restarts
	//   6. 2x Payment fails with retriable error
	//   7. Payment succeeds.

	// (1)
	hookAlice()
	mtx.Lock()
	payInvoiceErr = fmt.Errorf("test error: %w", clientintf.ErrRetriablePayment)
	mtx.Unlock()
	err := alice.TipUser(bob.PublicID(), float64(payMAtoms)/1e11, maxAttempts)
	assert.NilErr(t, err)

	// (2)
	assert.ChanWritten(t, bobSentInvoice)

	// (3) (4)
	assert.ChanWritten(t, alicePayFailedChan)
	assert.ChanWritten(t, alicePayFailedChan)

	// (5)
	assert.ChanNotWritten(t, alicePayFailedChan, 100*time.Millisecond)
	ts.stopClient(alice)
	alice = ts.recreateStoppedClient(alice)
	hookAlice()

	// (6)
	assert.ChanWritten(t, alicePayFailedChan)
	assert.ChanWritten(t, alicePayFailedChan)
	assert.ChanNotWritten(t, progressErrChan, time.Second)

	// (7)
	mtx.Lock()
	payInvoiceErr = nil
	mtx.Unlock()
	assert.ChanWritten(t, alicePaidChan)
	assert.ChanNotWritten(t, alicePaidChan, resendDelay)   // No more payments attempted
	assert.NilErrFromChan(t, progressErrChan)              // Success ntfn
	assert.ChanNotWritten(t, progressErrChan, resendDelay) // No more ntfns
}

// TestRecvTipPersistsSuccess asserts that receiving tips are notified across
// client restarts.
func TestRecvTipPersistsSuccess(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	ts.kxUsers(alice, bob)

	payMAtoms, payDcr := int64(1e11), 1.0
	customInvoice := "custom invoice"
	bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
		if amt == payMAtoms {
			return customInvoice, nil
		}
		return fmt.Sprintf("invoice for %d", amt), nil
	})

	// The first execution will not return until bob is done.
	bob.mpc.HookTrackInvoice(func(inv string, _ int64) (int64, error) {
		if inv == customInvoice {
			<-bob.ctx.Done()
		}
		return 0, bob.ctx.Err()
	})

	// The first hook to Bob's OnTipReceivedNtfn should not be triggered.
	bobTipRecvChan := make(chan error, 3)
	bob.handle(client.OnTipReceivedNtfn(func(_ *client.RemoteUser, amt int64) {
		if amt != payMAtoms {
			bobTipRecvChan <- fmt.Errorf("unexpected amount %d", amt)
		} else {
			bobTipRecvChan <- nil
		}
	}))

	assert.NilErr(t, alice.TipUser(bob.PublicID(), payDcr, 1))
	assert.ChanNotWritten(t, bobTipRecvChan, time.Second)

	// Setup next Bob's mpc hooked to track payments so that on restart,
	// Bob will attempt to track the data correctly.
	trackInvoiceChan := make(chan struct{})
	bobPCIniter := func(loggerSubsysIniter) clientintf.PaymentClient {
		mpc := &testutils.MockPayClient{}
		mpc.HookTrackInvoice(func(inv string, amt int64) (int64, error) {
			if inv == customInvoice {
				trackInvoiceChan <- struct{}{}
				return amt, nil
			}
			return 0, nil
		})
		return mpc
	}

	// Shutdown and recreate Bob.
	bob = ts.recreateClient(bob, withPCIniter(bobPCIniter))

	// The hook on Bob's second instance should be triggered after the
	// payment is completed.
	bob.handle(client.OnTipReceivedNtfn(func(_ *client.RemoteUser, amt int64) {
		if amt != payMAtoms {
			bobTipRecvChan <- fmt.Errorf("unexpected amount %d", amt)
		} else {
			bobTipRecvChan <- nil
		}
	}))

	// Bob should be tracking the custom invoice, and after that is
	// acknowledged as paid, Bob should get a notification that tipping
	// completed.
	assert.ChanWritten(t, trackInvoiceChan)
	assert.NilErrFromChan(t, bobTipRecvChan)
}

// TestRecvTipExpired asserts that if a tip payment expires, then it's not
// tracked anymore even after a restart.
func TestRecvTipExpired(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	ts.kxUsers(alice, bob)

	payMAtoms, payDcr := int64(1e11), 1.0
	customInvoice := "custom invoice"
	bob.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
		if amt == payMAtoms {
			return customInvoice, nil
		}
		return fmt.Sprintf("invoice for %d", amt), nil
	})

	// Return an expiration error for the invoice.
	bobExpireInvoiceChan := make(chan struct{})
	bob.mpc.HookTrackInvoice(func(inv string, _ int64) (int64, error) {
		if inv == customInvoice {
			bobExpireInvoiceChan <- struct{}{}
			return 0, clientintf.ErrInvoiceExpired
		}
		return 0, nil
	})

	// The first hook to Bob's OnTipReceivedNtfn should not be triggered.
	bobTipRecvChan := make(chan error, 3)
	bob.handle(client.OnTipReceivedNtfn(func(_ *client.RemoteUser, amt int64) {
		if amt != payMAtoms {
			bobTipRecvChan <- fmt.Errorf("unexpected amount %d", amt)
		} else {
			bobTipRecvChan <- nil
		}
	}))

	assert.NilErr(t, alice.TipUser(bob.PublicID(), payDcr, 1))
	assert.ChanNotWritten(t, bobTipRecvChan, time.Second)

	// Expire invoice.
	assert.ChanWritten(t, bobExpireInvoiceChan)

	// Setup next Bob's mpc hooked to track payments so that on restart,
	// Bob will attempt to track the data correctly.
	trackInvoiceChan := make(chan struct{})
	bobPCIniter := func(loggerSubsysIniter) clientintf.PaymentClient {
		mpc := &testutils.MockPayClient{}
		mpc.HookTrackInvoice(func(inv string, amt int64) (int64, error) {
			if inv == customInvoice {
				trackInvoiceChan <- struct{}{}
				return amt, nil
			}
			return 0, nil
		})
		return mpc
	}

	// Shutdown and recreate Bob.
	ts.recreateClient(bob, withPCIniter(bobPCIniter))

	// Bob should not be attempting to track an expired invoice.
	assert.ChanNotWritten(t, trackInvoiceChan, time.Second)
}
