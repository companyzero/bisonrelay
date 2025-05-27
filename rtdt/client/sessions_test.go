package rtdtclient

import (
	"testing"

	"github.com/companyzero/bisonrelay/rpc"
)

// TestSingleSession tests sending data in a single session.
func TestSingleSession(t *testing.T) {
	t.Parallel()

	var id rpc.RTDTPeerID = 1
	ts := newTestServer(t)
	tc := ts.newClient("alice")
	tsess := tc.newHandshakedSession(id)

	data1 := randomData(100)
	tsess.sendRandomData(data1, 0)
}

// TestSessionMultiplex tests joining and sending data in two different
// sessions.
func TestSessionMultiplex(t *testing.T) {
	t.Parallel()

	var id1, id2 rpc.RTDTPeerID = 1, 1<<16 + 1

	ts := newTestServer(t)
	tc := ts.newClient("alice")

	tsess1 := tc.newHandshakedSession(id1)
	tsess1.sendRandomData(randomData(100), 0)

	tsess2 := tc.newHandshakedSession(id2)
	tsess2.sendRandomData(randomData(100), 0)
}
