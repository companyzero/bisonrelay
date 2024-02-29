package e2etests

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// TestClientServerAgreeMaxMsgVersion asserts that client and server agree on
// the max message size and that an RM with the max payload can actually be
// sent from client to server and back.
func TestClientServerAgreeMaxMsgVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		maxSizeVersion rpc.MaxMsgSizeVersion
	}{{
		name:           "v0",
		maxSizeVersion: rpc.MaxMsgSizeV0,
	}, {
		name:           "v1",
		maxSizeVersion: rpc.MaxMsgSizeV1,
	}}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tcfg := testScaffoldCfg{
				serverMaxMsgSizeVersion: tc.maxSizeVersion,
			}
			ts := newTestScaffold(t, tcfg)
			ntfns := client.NewNotificationManager()
			mmvChan := make(chan rpc.MaxMsgSizeVersion, 5)
			disconnectedChan := make(chan struct{}, 5)
			ntfns.Register(client.OnServerSessionChangedNtfn(
				func(connected bool, policy clientintf.ServerPolicy) {
					if connected {
						mmvChan <- policy.MaxMsgSizeVersion
					} else {
						disconnectedChan <- struct{}{}
					}
				}))
			alice := ts.newClient("alice", withNtfns(ntfns))
			assert.ChanWrittenWithVal(t, mmvChan, tc.maxSizeVersion)

			bob := ts.newClient("bob")
			ts.kxUsers(alice, bob)

			bobPmChan := make(chan string, 5)
			bob.handle(client.OnPMNtfn(func(_ *client.RemoteUser, pm rpc.RMPrivateMessage, _ time.Time) {
				bobPmChan <- pm.Message
			}))

			// Send a large PM that still fits the server max msg
			// size. Note the PM is encoded to hex because it is
			// sent as a string.
			maxPayloadSize := rpc.MaxPayloadSizeForVersion(tc.maxSizeVersion)
			data := make([]byte, maxPayloadSize)
			_, _ = rand.Read(data)
			wantMsg := hex.EncodeToString(data[:len(data)/2])
			assert.NilErr(t, alice.PM(bob.PublicID(), wantMsg))
			assert.ChanWrittenWithValTimeout(t, bobPmChan, wantMsg, 2*time.Minute)

			// Send a chunk with the max payload. This will be
			// rejected by Bob due to being unrequested, but it
			// will be accepted by the server and fetched,
			// therefore use a notification hook to detect that
			// Bob received it.
			bobRMChan := make(chan []byte, 5)
			bob.handle(client.OnRMReceived(func(ru *client.RemoteUser, rmh *rpc.RMHeader, p interface{}, ts time.Time) {
				if m, ok := p.(rpc.RMFTGetChunkReply); ok {
					bobRMChan <- m.Chunk
				}
			}))
			rm := rpc.RMFTGetChunkReply{
				FileID: zkidentity.ShortID{}.String(),
				Index:  1<<32 - 1,
				Chunk:  data,
				Tag:    1<<32 - 1,
			}
			assert.NilErr(t, alice.testInterface().SendUserRM(bob.PublicID(), rm))
			gotData := assert.ChanWritten(t, bobRMChan)
			if !bytes.Equal(gotData, data) {
				t.Fatal("sent and received data are not equal")
			}
		})
	}
}
