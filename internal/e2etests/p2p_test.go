package e2etests

import (
	"testing"
)

// TestCanPM performs a simple E2E KX and PM test.
func TestCanPM(t *testing.T) {
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)
	assertClientsCanPM(t, alice, bob)
}
