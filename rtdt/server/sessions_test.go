package rtdtserver

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	randv2 "math/rand/v2"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"golang.org/x/crypto/nacl/secretbox"
)

// TestDifferentSessionIDs tests that changing any of the properties of a join
// cookie generates different session ids.
func TestDifferentSessionIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		modCookie func(jc *rpc.RTDTJoinCookie)
	}{{
		name: "OwnerSecret",
		modCookie: func(jc *rpc.RTDTJoinCookie) {
			jc.OwnerSecret[0] += 1
		},
	}, {
		name: "ServerSecret",
		modCookie: func(jc *rpc.RTDTJoinCookie) {
			jc.ServerSecret[0] += 1
		},
	}, {
		name: "Size",
		modCookie: func(jc *rpc.RTDTJoinCookie) {
			jc.Size -= 1
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := newTestServer(t, withTestCookieKey())

			// Alice, Bob and Charlie use unique IDs. Dave uses the
			// same peer ID as Alice, to prove different cookies
			// generate different on-server sessions.
			var aliceId, bobId, charlieId rpc.RTDTPeerID = 1, 2, 3
			var daveId rpc.RTDTPeerID = aliceId

			alice, bob := ts.newClient(), ts.newClient()
			charlie, dave := ts.newClient(), ts.newClient()
			aliceSess := alice.joinSession(aliceId)
			bobSess := bob.joinSession(bobId)

			// Charlie and Dave will join with a modified join
			// cookie.
			charlieJc := ts.validJoinCookie(charlieId)
			tc.modCookie(&charlieJc)
			charlieSess := charlie.joinSessionWithCookie(charlieId, ts.encryptJoinCookie(&charlieJc))
			daveJc := ts.validJoinCookie(daveId)
			tc.modCookie(&daveJc)
			daveSess := dave.joinSessionWithCookie(daveId, ts.encryptJoinCookie(&daveJc))

			// Alice and Bob exchange data, Charlie and Dave do not
			// (but they do with each other).
			assertCanExchangeData(t, aliceSess, bobSess)
			assertNoData(t, charlie, dave)
			assertCanExchangeData(t, charlieSess, daveSess)

			// Charlie will join again with a standard join cookie
			// (same session as Alice and Bob).
			charlieSess = charlie.joinSession(charlieId)
			assertCanExchangeData(t, aliceSess, bobSess, charlieSess)
			assertNoData(t, dave)

			// Dave is now alone in his session (because Bob's peer
			// id got reassigned to a different session).
			dave.sendRandomData(daveId, []byte("dave data"), 0)
			assertNoData(t, charlie)
		})
	}
}

// TestServerMemberListing tests the behavior of member listings after members
// join and leave.
func TestServerMemberListing(t *testing.T) {
	t.Parallel()

	minListInterval := 500 * time.Millisecond
	maxPingInterval := 6 * minListInterval
	loopTickerInterval := maxPingInterval + 100*time.Millisecond
	sessListInterval := loopTickerInterval + time.Second
	ts := newTestServer(t,
		withPingInterval(maxPingInterval, 1),
		withServerMembersListInterval(sessListInterval, minListInterval),
		withTimeoutLoopTickerInterval(loopTickerInterval),
		withEnabledForceSendMembersList())

	var aliceId, bobId, charlieId rpc.RTDTPeerID = 1, 2, 1<<16 + 1

	alice, bob := ts.newClient(), ts.newClient()

	// Charlie is used to assert no extraneous messages.
	charlie := ts.newClient()
	charlieSess := charlie.joinSession(charlieId)
	charlieSess.assertNextMembersList(t)
	assertNoData(t, charlie)

	// Alice joins, receives a list with only Alice.
	aliceSess := alice.joinSession(aliceId)
	aliceMembers1 := aliceSess.assertNextMembersList(t)
	assert.True(t, aliceMembers1.Contains(uint32(aliceId)))
	assert.False(t, aliceMembers1.Contains(uint32(bobId)))
	assert.False(t, aliceMembers1.Contains(uint32(charlieId)))

	// Bob joins, receives a list with both members, and Alice receives an
	// update.
	time.Sleep(minListInterval)
	bobSess := bob.joinSession(bobId)
	bobMembers1 := bobSess.assertNextMembersList(t)
	assert.True(t, bobMembers1.Contains(uint32(aliceId)))
	assert.True(t, bobMembers1.Contains(uint32(bobId)))
	aliceMembers2 := aliceSess.assertNextMembersList(t)
	assert.True(t, aliceMembers2.Contains(uint32(aliceId)))
	assert.True(t, aliceMembers2.Contains(uint32(bobId)))

	// Bob leaves, Alice receives an update.
	time.Sleep(minListInterval)
	bobSess.leaveSession()
	aliceMembers3 := aliceSess.assertNextMembersList(t)
	assert.True(t, aliceMembers3.Contains(uint32(aliceId)))
	assert.False(t, aliceMembers3.Contains(uint32(bobId)))

	// Bob rejoins.
	time.Sleep(minListInterval)
	bobSess = bob.joinSession(bobId)
	bobMembers2 := bobSess.assertNextMembersList(t)
	assert.True(t, bobMembers2.Contains(uint32(aliceId)))
	assert.True(t, bobMembers2.Contains(uint32(bobId)))
	aliceMembers4 := aliceSess.assertNextMembersList(t)
	assert.True(t, aliceMembers4.Contains(uint32(aliceId)))
	assert.True(t, aliceMembers4.Contains(uint32(bobId)))

	// Bob times out. Keep sending data on Alice and Charlie to ensure they
	// are not timed out.
	start := time.Now()
	for time.Since(start) < sessListInterval+time.Second {
		aliceSess.sendRandomData(randomData(32), 0)
		charlieSess.sendRandomData(randomData(32), 0)
		time.Sleep(100 * time.Millisecond)
	}

	// Alice and Charlie should receive new listings.
	aliceMembers5 := aliceSess.assertNextMembersList(t)
	assert.True(t, aliceMembers5.Contains(uint32(aliceId)))
	assert.False(t, aliceMembers5.Contains(uint32(bobId)))
	charlieSess.assertNextMembersList(t)
}

// BenchmarkSessionMem benchmarks how much memory an additional session takes.
//
// NOTE: when running on linux, if the kernel socket buffers are too low, this
// may fail.
func BenchmarkSessionMem(b *testing.B) {
	for _, withLogging := range []bool{false, true} {
		b.Run(fmt.Sprintf("log=%v", withLogging), func(b *testing.B) {
			opts := []testScaffoldOption{withTestCookieKey()}
			if !withLogging {
				opts = append(opts, withDisabledLogger())
			}

			ts := newTestServer(b, opts...)
			tc := ts.newClient()

			// Prepare requests beforehand, to avoid benchmarking the test client
			// itself.
			requests := make([][]byte, b.N)
			var joinCmd rpc.RTDTServerCmdJoinSession
			for i := 0; i < b.N; i++ {
				id := rpc.RTDTPeerID(i + 1)
				jc := ts.validJoinCookie(id)
				binary.BigEndian.PutUint32(jc.OwnerSecret[:], uint32(id))
				joinCmd.JoinCookie = ts.encryptJoinCookie(&jc)
				tc.pkt.Source = id
				tc.pkt.Sequence++
				framedBytes := joinCmd.AppendFramed(&tc.pkt, nil)

				var nonce [24]byte
				rand.Read(nonce[:])
				out := append([]byte(nil), nonce[:]...)
				requests[i] = secretbox.Seal(out, framedBytes, &nonce, tc.sessKey)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := tc.c.Write(requests[i])
				if err != nil {
					b.Fatal(err)
				}
			}

			// Sanity check the server actually created the sessions.
			var gotCount int
			for i := 0; i < 3000; i++ {
				gotCount = ts.s.sessions.len()
				if gotCount == b.N {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if gotCount != b.N {
				// Possibly kernel socket buffers are too small. Increase with
				// sysctl -w net.core.{rmem_max,rmem_default,wmem_max,wmem_default}=999999999
				b.Fatalf("Unexpected number of server sessions: got %d, want %d",
					gotCount, b.N)
			}
		})
	}
}

// BenchmarkExtraPeerMem benchmarks adding additional peers to a single session.
//
// NOTE: when running on linux, if the kernel socket buffers are too low, this
// may fail.
func BenchmarkExtraPeerMem(b *testing.B) {
	for _, withLogging := range []bool{false, true} {
		b.Run(fmt.Sprintf("log=%v", withLogging), func(b *testing.B) {
			opts := []testScaffoldOption{withTestCookieKey()}
			if !withLogging {
				opts = append(opts, withDisabledLogger())
			}
			ts := newTestServer(b, opts...)
			tc := ts.newClient()

			// Prepare requests beforehand, to avoid benchmarking the test client
			// itself.
			requests := make([][]byte, b.N)
			var joinCmd rpc.RTDTServerCmdJoinSession
			for i := 0; i < b.N; i++ {
				id := rpc.RTDTPeerID(1 + i)
				jc := ts.validJoinCookie(id)
				jc.OwnerSecret = zkidentity.ShortID{}
				joinCmd.JoinCookie = ts.encryptJoinCookie(&jc)
				tc.pkt.Source = id
				tc.pkt.Sequence++
				framedBytes := joinCmd.AppendFramed(&tc.pkt, nil)

				var nonce [24]byte
				rand.Read(nonce[:])
				out := append([]byte(nil), nonce[:]...)
				requests[i] = secretbox.Seal(out, framedBytes, &nonce, tc.sessKey)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := tc.c.Write(requests[i])
				if err != nil {
					b.Fatal(err)
				}
			}

			// Sanity check the server actually created the peers.
			var gotPeers int
			for i := 0; i < 10000; i++ {
				ts.s.sessions.mtx.Lock()
				var firstSess *session
				for _, firstSess = range ts.s.sessions.sessions {
					break
				}
				ts.s.sessions.mtx.Unlock()

				if firstSess != nil {
					firstSess.mtx.Lock()
					gotPeers = len(firstSess.peers)
					firstSess.mtx.Unlock()

					if gotPeers == b.N {
						break
					}
				}

				time.Sleep(time.Millisecond)
			}
			if gotPeers != b.N {
				// Possibly kernel socket buffers are too small. Increase with
				// sysctl -w net.core.{rmem_max,rmem_default,wmem_max,wmem_default}=999999999
				b.Fatalf("Unexpected number of peers: got %d, want %d",
					gotPeers, b.N)
			}
		})
	}
}

// BenchmarkExtraPendingConnMem benchmarks adding an additional pending
// connection to the server.
//
// NOTE: Due to its nature, this necessarily also benchmarks the kernel on the
// client side. Manual evaluation of the benchmark results may be necessary.
func BenchmarkExtraPendingConnMem(b *testing.B) {
	for _, withLogging := range []bool{false, true} {
		b.Run(fmt.Sprintf("log=%v", withLogging), func(b *testing.B) {
			opts := []testScaffoldOption{withTestCookieKey()}
			if !withLogging {
				opts = append(opts, withDisabledLogger())
			}
			ts := newTestServer(b, opts...)

			// Prepare the handshake requests beforehand, to reduce
			// noise as much as possible.
			requests := make([][]byte, b.N)
			for i := 0; i < b.N; i++ {
				cipherSessKey, _ := ts.serverPub.Encapsulate()
				requests[i] = cipherSessKey[:]
			}

			serverAddr := net.UDPAddrFromAddrPort(netip.MustParseAddrPort(ts.addr))
			replyBuf := make([]byte, rpc.RTDTMaxMessageSize)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				udpConn, err := net.DialUDP("udp", nil, serverAddr)
				if err != nil {
					b.Fatal(err)
				}

				_, err = udpConn.Write(requests[i])
				if err != nil {
					b.Fatal(err)
				}

				udpConn.Read(replyBuf[:])
			}

			// Sanity check the server actually created the conns.
			var gotCount int
			for i := 0; i < 10000; i++ {
				_, pending := ts.s.totalConnCounts()
				gotCount = int(pending)

				if gotCount == b.N {
					break
				}

				time.Sleep(time.Millisecond)
			}
			if gotCount != b.N {
				b.Fatalf("Unexpected number of pending connections: got %d, want %d",
					gotCount, b.N)
			}
		})
	}
}

// BenchmarkExtraConnMem benchmarks adding an additional connection to the
// server.
//
// NOTE: Due to its nature, this necessarily also benchmarks the kernel on the
// client side. Manual evaluation of the benchmark results may be necessary.
func BenchmarkExtraConnMem(b *testing.B) {
	for _, withLogging := range []bool{false, true} {
		b.Run(fmt.Sprintf("log=%v", withLogging), func(b *testing.B) {
			opts := []testScaffoldOption{withTestCookieKey()}
			if !withLogging {
				opts = append(opts, withDisabledLogger())
			}
			ts := newTestServer(b, opts...)

			// Prepare the requests beforehand, to reduce
			// noise as much as possible.
			//
			// The first request is the handshake, the second is
			// a ping command that finalizes the handshake.
			requests := make([][2][]byte, b.N)
			pkt := rpc.RTDTFramedPacket{Source: 1, Sequence: 1}
			pingCmd := rpc.RTDTPingCmd{}
			var cmdNonce [24]byte
			for i := 0; i < b.N; i++ {
				// Handshake.
				cipherSessKey, sessKey := ts.serverPub.Encapsulate()
				requests[i][0] = cipherSessKey[:]

				// Ping command (first encrypted pkt finalizes
				// conn).
				cmdPlain := pingCmd.AppendFramed(&pkt, nil)
				rand.Read(cmdNonce[:])
				cmdCipher := append([]byte(nil), cmdNonce[:]...)
				cmdCipher = secretbox.Seal(cmdCipher, cmdPlain, &cmdNonce, sessKey)
				requests[i][1] = cmdCipher
			}

			serverAddr := net.UDPAddrFromAddrPort(netip.MustParseAddrPort(ts.addr))
			replyBuf := make([]byte, rpc.RTDTMaxMessageSize)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				udpConn, err := net.DialUDP("udp", nil, serverAddr)
				if err != nil {
					b.Fatal(err)
				}

				// Send handshake, receive reply.
				_, err = udpConn.Write(requests[i][0])
				if err != nil {
					b.Fatal(err)
				}
				_, err = udpConn.Read(replyBuf[:])
				if err != nil {
					b.Fatal(err)
				}

				// Send ping, receive reply.
				_, err = udpConn.Write(requests[i][1])
				if err != nil {
					b.Fatal(err)
				}
				_, err = udpConn.Read(replyBuf[:])
				if err != nil {
					b.Fatal(err)
				}
			}

			// Sanity check the server actually created the conns.
			var gotCount int
			for i := 0; i < 10000; i++ {
				conns, _ := ts.s.totalConnCounts()
				gotCount = int(conns)

				if gotCount == b.N {
					break
				}

				time.Sleep(time.Millisecond)
			}
			if gotCount != b.N {
				b.Fatalf("Unexpected number of pending connections: got %d, want %d",
					gotCount, b.N)
			}
		})
	}
}

// BenchmarkRelayData benchmarks relaying data between sessions with various
// number of users.
//
// NOTE: run with -benchtime 1m -memprofilerate 1 to get better stats.
func BenchmarkRelayData(b *testing.B) {
	type outboundPkt struct {
		tc   *testClient
		data []byte
	}

	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			opts := []testScaffoldOption{withTestCookieKey()}
			ts := newTestServer(b, opts...)
			var clients []*testClientSession
			for i := 0; i < n; i++ {
				tc := ts.newClient()
				sess := tc.joinSession(rpc.RTDTPeerID(i) + 1)
				clients = append(clients, sess)
			}

			// Create and serialize the messages beforehand, to
			// ensure we only benchmark server side message
			// processing.
			outData := make([]outboundPkt, b.N)
			for i := 0; i < b.N; i++ {
				// Pick a random source client.
				srcIndex := randv2.IntN(n)
				srcClient := clients[srcIndex]

				// Create, E2E encrypt, then C2S encrypt.
				data := randomData(200)
				e2eEncData := srcClient.tc.prepareDataPkt(srcClient.id,
					rpc.RTDTStreamRandom, data, 0)
				serverEncData := srcClient.tc.prepareWrite(e2eEncData)

				// Store it already prepared to send data.
				outData[i] = outboundPkt{
					tc:   srcClient.tc,
					data: append([]byte(nil), serverEncData...),
				}
			}

			// Wait to ensure everyone is in the session.
			time.Sleep(10 * time.Millisecond)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				outPkt := outData[i]
				outPkt.tc.c.Write(outPkt.data)
			}
		})
	}
}
