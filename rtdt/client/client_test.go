package rtdtclient

import (
	"testing"

	"github.com/companyzero/bisonrelay/rpc"
)

// TestSessionsWithSameID tests connecting to the server using sessions with
// the same id.
//
// This may happen because the server uses fields inside the join cookie to
// uniquely identify a session, therefore sessions with different join cookies
// may use the same peer id.
func TestSessionsWithSameID(t *testing.T) {
	t.Parallel()

	var id rpc.RTDTPeerID = 1
	ts := newTestServer(t)
	tc := ts.newClient("alice")

	// First connection.
	tsess1 := tc.newHandshakedSession(id)
	tsess1.sendRandomData(randomData(100), 0)

	// Second connection using same id (creates a different socket).
	tsess2 := tc.newHandshakedSession(id)
	tsess2.sendRandomData(randomData(100), 0)

	// The first one still works.
	tsess1.sendRandomData(randomData(100), 0)
}
