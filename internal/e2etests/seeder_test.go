package e2etests

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	seederclient "github.com/companyzero/bisonrelay/brseeder/client"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/jrick/wsrpc/v2"
)

func TestSeenderAndServer(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{runSeeder: true}
	ts := newTestScaffold(t, tcfg)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// Responses to set in server to simulate dcrlnd being online/offline.
	lnInfoOffline := &lnrpc.GetInfoResponse{
		Alias:        seederToken,
		ServerActive: false,
	}
	lnInfoOnline := &lnrpc.GetInfoResponse{
		Alias:        seederToken,
		ServerActive: true,
	}

	// Mock alternative server to simulate various changes.
	seederUrl := fmt.Sprintf("ws://%s/api/v1/status", ts.seederAddr)
	ws, err := wsrpc.Dial(ctx, seederUrl, wsrpc.WithBearerAuthString(mockSeederToken))
	assert.NilErr(t, err)
	simMockStatus := func(isMaster, dbOnline, nodeOnline bool) bool {
		status := rpc.SeederCommandStatus{
			LastUpdated: time.Now().Unix(),
			Database: rpc.SeederCommandStatusDB{
				Online: dbOnline,
				Master: isMaster,
			},
			Node: rpc.SeederCommandStatusNode{
				Alias:  mockSeederToken,
				Online: nodeOnline,
			},
		}
		var reply rpc.SeederCommandStatusReply
		err := ws.Call(ctx, "status", &reply, status)
		assert.NilErr(t, err)
		return reply.Master
	}

	// Helper to assert the current master returned by the seeder.
	clientApiURL := fmt.Sprintf("http://%s/api/v1/live", ts.seederAddr)
	var dialCfg net.Dialer
	assertSeederMasterIs := func(wantMaster string) {
		t.Helper()
		gotMaster, err := seederclient.QuerySeeder(ctx, clientApiURL, dialCfg.DialContext)
		assert.NilErr(t, err)
		assert.DeepEqual(t, gotMaster, wantMaster+":443")
	}

	// Helper to assert that the seeder returns no master.
	assertSeederHasNoMaster := func() {
		t.Helper()
		_, err = seederclient.QuerySeeder(ctx, clientApiURL, dialCfg.DialContext)
		assert.ErrorIs(t, err, seederclient.ErrNoServers)
	}

	// Helper to assert that brserver force sends an update and receives a
	// specific response.
	assertServerForceUpdate := func(wantMaster bool) {
		t.Helper()
		isMaster, err := ts.svr.SeederForceUpdate(ctx)
		assert.NilErr(t, err)
		assert.DeepEqual(t, isMaster, wantMaster)
	}

	// Seeder starts out without master.
	assertSeederHasNoMaster()

	// FS db always reports as master, so server iniates as master.
	assert.DeepEqual(t, ts.svr.IsMaster(), true)

	// After checking, should still be master and seeder should report it as
	// master.
	assertServerForceUpdate(true)
	assertSeederMasterIs(seederToken)

	// Other server sends update. This doesn't change master.
	assert.DeepEqual(t, simMockStatus(false, true, true), false)
	assertSeederMasterIs(seederToken)

	// Force dcrlnd node to be out (to force an offline condition. This
	// removes the master in seeder but NOT as commanded to the server.
	ts.lnRpc.setGetInfoResponse(lnInfoOffline, nil)
	assertServerForceUpdate(true)
	assertSeederHasNoMaster()
	assert.DeepEqual(t, ts.svr.IsMaster(), true)

	// Enough time passes that the server is demoted.
	time.Sleep(seederOfflineLimit)
	assertServerForceUpdate(false)
	assertSeederHasNoMaster()
	assert.DeepEqual(t, ts.svr.IsMaster(), false)

	// Other server sends update, but is not master yet. Seeder makes him
	// master.
	assert.DeepEqual(t, simMockStatus(false, true, true), true)
	assertSeederMasterIs(mockSeederToken)

	// Dcrlnd comes back online on server. Server will try to become master,
	// but it won't happen because the mock server is already master.
	ts.lnRpc.setGetInfoResponse(lnInfoOnline, nil)
	assertServerForceUpdate(false)
	assertSeederMasterIs(mockSeederToken)
	assert.DeepEqual(t, ts.svr.IsMaster(), false)

	// The mock server says it's not master anymore. This makes the seeder
	// without master.
	assert.DeepEqual(t, simMockStatus(false, true, true), false)
	assertSeederHasNoMaster()

	// Finally, the original server signals and goes back to being master.
	assertServerForceUpdate(true)
	assertSeederMasterIs(seederToken)
	assert.DeepEqual(t, ts.svr.IsMaster(), true)
}
