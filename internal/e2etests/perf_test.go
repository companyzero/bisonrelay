//go:build dcrlnde2e

package e2etests

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"golang.org/x/sync/errgroup"
)

func TestPerfSendPMs(t *testing.T) {
	tcfg := testScaffoldCfg{skipNewServer: true}
	ts := newTestScaffold(t, tcfg)
	ts.svrAddr = "localhost:12345"

	nbAlts := runtime.NumCPU()

	altIds := make([]clientintf.UserID, nbAlts)
	alice := ts.newClient("alice", withSimnetEnvDcrlndPayClient(t, false))
	for i := 0; i < nbAlts; i++ {
		name := fmt.Sprintf("alt_%d", i)
		alt := ts.newClient(name, withSimnetEnvDcrlndPayClient(t, true))
		altIds[i] = alt.PublicID()
		ts.kxUsers(alice, alt)
		assertClientsCanPM(t, alice, alt)
		ts.stopClient(alt)
	}

	time.Sleep(10 * time.Millisecond)
	alice.log.Info("Starting the parallel send goroutines")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	g, gctx := errgroup.WithContext(ctx)

	var maxQLen, maxAckLen int
	go func() {
		for {
			select {
			case <-gctx.Done():
				return
			case <-time.After(50 * time.Millisecond):
				qlen, acklen := alice.RMQLen()
				if qlen > maxQLen {
					maxQLen = qlen
				}
				if acklen > maxAckLen {
					maxAckLen = acklen
				}
			}
		}
	}()

	// Send multiple messages for each client. The clients are offline, so
	// we're testing performance of the send stack.
	nbMsgs := 200
	startTime := time.Now()
	aliceTI := alice.testInterface()
	for altIdx, alt := range altIds {
		altIdx := altIdx
		alt := alt
		g.Go(func() error {
			for i := 0; i < nbMsgs; i++ {
				i := i
				select {
				case <-gctx.Done():
					break
				default:
				}

				msg := rpc.RMPrivateMessage{
					Mode:    rpc.RMPrivateMessageModeNormal,
					Message: fmt.Sprintf("msg_%d_%d", altIdx, i),
				}
				err := aliceTI.QueueUserRM(alt, msg)
				if err != nil {
					return err
				}
			}
			return nil
		})

	}

	assert.NilErr(t, g.Wait())

	// Wait until the queues are empty.
	maxCheck := 12000
	for i := 0; i < maxCheck; i++ {
		qlen, acklen := alice.RMQLen()
		if qlen+acklen == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
		if i == maxCheck-1 {
			t.Fatal("timeout waiting for queue to drain")
		}
	}

	totalTime := time.Since(startTime)

	t.Logf("Finished sending %d messages", nbAlts*nbMsgs)
	t.Logf("MaxQLen: %d, MaxAckLen: %d", maxQLen, maxAckLen)
	t.Logf("Total time: %s", totalTime)
	t.Logf("Messages/sec: %.2f", float64(nbAlts*nbMsgs)/totalTime.Seconds())
}
