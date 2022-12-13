package e2etests

import (
	"fmt"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/zkidentity"
)

func assertClientsKXd(t testing.TB, alice, bob *testClient) {
	t.Helper()
	var gotAlice, gotBob bool
	for i := 0; (!gotAlice || !gotBob) && i < 100; i++ {
		if alice.UserExists(bob.PublicID()) {
			gotAlice = true
		}
		if bob.UserExists(alice.PublicID()) {
			gotBob = true
		}
		time.Sleep(time.Millisecond * 100)
	}
	if !gotAlice || !gotBob {
		t.Fatalf("KX did not complete %v %v", gotAlice, gotBob)
	}
}

func assertClientInGC(t testing.TB, c *testClient, gcID zkidentity.ShortID) {
	t.Helper()
	for i := 0; i < 100; i++ {
		_, err := c.GetGC(gcID)
		if err == nil {
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
	t.Fatalf("Client did not join GC %s before timeout", gcID)
}

// assertClientUpToDate verifies the client has no pending updates to send
// to the server.
func assertClientUpToDate(t testing.TB, c *testClient) {
	t.Helper()
	var err error
	for i := 0; i < 200; i++ {
		err = nil
		if !c.RVsUpToDate() {
			err = fmt.Errorf("RVs are not up to date in the server")
		} else if c.RMQLen() != 0 {
			err = fmt.Errorf("RMQ is not empty")
		}
		if err != nil {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
}
