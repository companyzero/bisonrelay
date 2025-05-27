package rtdtserver

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
)

// TestSpentAllowance tests that sending more data than the allowance allows
// makes the server stop forwarding data.
func TestSpentAllowance(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, withTestCookieKey())

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)

	// Bob will generate a cookie with little data.
	const headerOverhead = 18
	bobJc := ts.validJoinCookie(bobId)
	bobJc.PublishAllowance = headerOverhead + 8
	bobCookie := bobJc.Encrypt(nil, ts.cookieKey)
	bobSess := bob.joinSessionWithCookie(bobId, bobCookie)

	// Alice can send a lot of data.
	data1 := randomData(1024)
	aliceSess.sendRandomData(data1, 0)
	bob.assertNextRandomData(alice, data1)

	// Bob sends enough do deplete its allowance.
	data2 := randomData(int(bobJc.PublishAllowance - headerOverhead))
	bobSess.sendRandomData(data2, 0)
	alice.assertNextRandomData(alice, data2)

	// Alice can still send data.
	data3 := randomData(1024)
	aliceSess.sendRandomData(data3, 0)
	bob.assertNextRandomData(alice, data3)

	// Bob cannot send data.
	bobSess.sendRandomData(data2, 0)
	assertNoData(t, alice)
	ts.assertNoAllowanceData(uint64(len(data2) + headerOverhead))

	// Bob replenishes its allowance.
	bob.joinSession(bobId)

	// Bob can now send data.
	assertCanExchangeData(t, aliceSess, bobSess)
}

// TestAllowanceMultiplePayments tests sending data that depletes multiple
// payments.
func TestAllowanceMultiplePayments(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, withTestCookieKey())

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	alice.joinSession(aliceId)

	// Bob will send multiple payments that accumulate an allowance.
	var bobSess *testClientSession
	const headerOverhead = 18
	const payCount = 10
	const dataBytes = 100 * payCount
	for i := 0; i < payCount; i++ {
		bobJc := ts.validJoinCookie(bobId)
		bobJc.PublishAllowance = dataBytes / payCount
		if i == 0 {
			bobJc.PublishAllowance += headerOverhead
		}
		bobCookie := bobJc.Encrypt(nil, ts.cookieKey)
		bobSess = bob.joinSessionWithCookie(bobId, bobCookie)
		bobSess.assertErrCode(t, errCodeNoError)
	}

	// Bob sends enough do deplete its allowance.
	data1 := randomData(dataBytes)
	bobSess.sendRandomData(data1, 0)
	alice.assertNextRandomData(alice, data1)

	// Next send fails.
	data2 := randomData(10)
	bobSess.sendRandomData(data2, 0)
	assertNoData(t, alice)
	ts.assertNoAllowanceData(uint64(len(data2) + headerOverhead))
}

// TestAllowanceAcrossConnections tests that allowance is correctly maintained
// across multiple connections.
func TestAllowanceAcrossConnections(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, withTestCookieKey())

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	alice.joinSession(aliceId)

	// Bob will send multiple payments that accumulate an allowance.
	var bobSess *testClientSession
	const headerOverhead = 18
	const dataSize = 1000
	bobJc := ts.validJoinCookie(bobId)
	bobJc.PublishAllowance = 2*dataSize + 2*headerOverhead
	bobCookie := bobJc.Encrypt(nil, ts.cookieKey)
	bobSess = bob.joinSessionWithCookie(bobId, bobCookie)
	bobSess.assertErrCode(t, errCodeNoError)

	// Bob sends a message which consumes half the allowance.
	data1 := randomData(dataSize)
	bobSess.sendRandomData(data1, 0)
	alice.assertNextRandomData(alice, data1)

	// Bob reconnects under a different connection, but using the same
	// join cookie and sends another message.
	bob2 := ts.newClient()
	bob2Sess := bob2.joinSessionWithCookie(bobId, bobCookie)
	bob2Sess.assertErrCode(t, errCodeNoError)
	data2 := randomData(dataSize)
	bob2Sess.sendRandomData(data2, 0)
	alice.assertNextRandomData(alice, data2)

	// Next send fails because Bob consumed the rest of the allowance. This
	// proves that, if reconnecting refreshed the allowance, this next
	// message would have been relayed.
	data3 := randomData(10)
	bob2Sess.sendRandomData(data3, 0)
	assertNoData(t, alice)
	ts.assertNoAllowanceData(uint64(len(data3) + headerOverhead))
}

// TestSendOnUnjoinedSession tests that sending data for a session the client
// hasn't joined doesn't work.
func TestSendOnUnjoinedSession(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)
	bobSess := bob.joinSession(bobId)

	// Alice and Bob can exchange data on the joined sessions, but trying
	// to send data for a session Bob isn't bound to doesn't relay it.
	assertCanExchangeData(t, aliceSess, bobSess)
	bob.sendRandomData(bobId+100, []byte("data from bob"), 0)
	assertNoData(t, alice, bob)
	assertCanExchangeData(t, aliceSess, bobSess)
}

// TestTimedOutConns tests that sending data for a session that has been timed
// out works as intended.
func TestTimedOutConns(t *testing.T) {
	t.Parallel()
	timeoutInterval := 3 * time.Second
	ts := newTestServer(t, withPingInterval(timeoutInterval, 1))

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)
	bobSess := bob.joinSession(bobId)
	assertCanExchangeData(t, aliceSess, bobSess)

	// Keep sending data on Alice (to avoid timeout) but skip it on Bob,
	// until the timeout elapses.
	bobTimedOut := false
	for start := time.Now(); !bobTimedOut && time.Since(start) < 4*timeoutInterval; {
		aliceSess.sendRandomData([]byte("data"), 0)
		if !bobTimedOut {
			_, err := bob.readNext() // Drain data.
			bobTimedOut = errors.Is(err, os.ErrDeadlineExceeded)
			if !bobTimedOut {
				time.Sleep(timeoutInterval / 10)
			}
		}
	}

	if !bobTimedOut {
		t.Fatalf("Bob did not timeout during test")
	}

	// Bob can still join back. It needs to perform a new handshake due to
	// having been timed out.
	assert.NilErr(t, bob.handshake())
	bobSess = bob.joinSession(bobId)
	bobSess.assertErrCode(t, errCodeNoError)
	assertCanExchangeData(t, aliceSess, bobSess)
}

// TestAllowanceAcrossTimedOutConnections tests that allowance is correctly
// maintained across multiple connections, when the initial connection timed
// out.
func TestAllowanceAcrossTimedOutConnections(t *testing.T) {
	t.Parallel()
	timeoutInterval := time.Second
	ts := newTestServer(t, withTestCookieKey(), withPingInterval(timeoutInterval, 1))

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)

	// Bob will send multiple payments that accumulate an allowance.
	var bobSess *testClientSession
	const headerOverhead = 18
	const dataSize = 1000
	bobJc := ts.validJoinCookie(bobId)
	bobJc.PublishAllowance = 2*dataSize + 2*headerOverhead
	bobCookie := bobJc.Encrypt(nil, ts.cookieKey)
	bobSess = bob.joinSessionWithCookie(bobId, bobCookie)
	bobSess.assertErrCode(t, errCodeNoError)

	// Bob sends a message which consumes half the allowance.
	data1 := randomData(dataSize)
	bobSess.sendRandomData(data1, 0)
	alice.assertNextRandomData(alice, data1)

	// Keep sending data on Alice (to avoid timeout) but skip it on Bob,
	// until the timeout elapses.
	bobTimedOut := false
	for start := time.Now(); !bobTimedOut && time.Since(start) < 4*timeoutInterval; {
		aliceSess.sendRandomData([]byte("data"), 0)
		if !bobTimedOut {
			_, err := bob.readNext() // Drain data.
			bobTimedOut = errors.Is(err, os.ErrDeadlineExceeded)
			if !bobTimedOut {
				time.Sleep(timeoutInterval / 10)
			}
		}
	}
	if !bobTimedOut {
		t.Fatalf("Bob did not timeout during test")
	}

	// Perform a new handshake, which reuses the udp address on Bob but
	// causes a new conn structure in the server to be allocated.
	assert.NilErr(t, bob.handshake())
	bob2Sess := bob.joinSessionWithCookie(bobId, bobCookie)
	bob2Sess.assertErrCode(t, errCodeNoError)
	data2 := randomData(dataSize)
	bob2Sess.sendRandomData(data2, 0)
	alice.assertNextRandomData(alice, data2)

	// Next send fails because Bob consumed the rest of the allowance. This
	// proves that, if reconnecting refreshed the allowance, this next
	// message would have been relayed.
	data3 := randomData(10)
	bob2Sess.sendRandomData(data3, 0)
	assertNoData(t, alice)
	ts.assertNoAllowanceData(uint64(len(data3) + headerOverhead))
}

// TestAllowanceKeptAfterCookieTimedOut tests that the allowance keeps the peer
// working even if the cookie is timed out.
func TestAllowanceKeptAfterCookieTimedOut(t *testing.T) {
	t.Parallel()
	dropPaymentInterval := time.Second
	ts := newTestServer(t, withTestCookieKey(), withDropPaymentLoopInterval(dropPaymentInterval))

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)
	bobJc := ts.validJoinCookie(bobId)
	bobCookie := ts.encryptJoinCookie(&bobJc)
	bobSess := bob.joinSessionWithCookie(bobId, bobCookie)

	assertCanExchangeData(t, aliceSess, bobSess)

	// Wait until the cookie expires.
	time.Sleep(dropPaymentInterval * 2)

	// The session still works.
	assertCanExchangeData(t, aliceSess, bobSess)

	// But attempting to join again using the cookie does not.
	bob2 := ts.newClient()
	bob2Sess := bob2.joinSessionWithCookie(bobId, bobCookie)
	bob2Sess.assertErrCode(t, errCodeJoinCookieExpired)

	// The original session is still working.
	assertCanExchangeData(t, aliceSess, bobSess)
}

// TestSeqAcceptance tests acceptance of packets with various sequence numbers.
func TestSeqAcceptance(t *testing.T) {
	t.Parallel()
	timeoutInterval := time.Second
	ts := newTestServer(t, withTestCookieKey(), withPingInterval(timeoutInterval, 1))

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)
	bobSess := bob.joinSession(bobId)
	assertCanExchangeData(t, aliceSess, bobSess)

	// Rewind sequence. This should not be accepted.
	alice.pkt.Sequence -= 1
	aliceSess.sendRandomData([]byte("alice data"), 0)
	assertNoData(t, bob)

	// Move sequence forward by a large number (simulates packet loss).
	alice.pkt.Sequence += 20000
	assertCanExchangeData(t, aliceSess, bobSess)

	// Rewind sequence to simulate out-of-order receiving.
	alice.pkt.Sequence -= 5
	assertCanExchangeData(t, aliceSess, bobSess)
}
