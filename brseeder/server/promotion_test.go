package seederserver

import (
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/internal/testutils"
	"github.com/companyzero/bisonrelay/rpc"
)

// TestServerPromotionRules tests the rules for promotion of servers.
func TestServerPromotionRules(t *testing.T) {
	t.Parallel()

	const waitForMaster = 250 * time.Millisecond
	const offlineLimit = time.Second // Can't be < 1 second due to LastUpdated being unix seconds.

	const token1, token2 = "token1", "token2"

	s, err := New(
		WithLogger(testutils.TestLoggerSys(t, "XXXX")),
		withPromotionTimeLimits(waitForMaster, offlineLimit),
		WithTokens(map[string]struct{}{token1: {}, token2: {}}),
	)
	assert.NilErr(t, err)

	testStatus := func(dbOnline bool, dbMaster bool, nodeOnline bool) rpc.SeederCommandStatus {
		return rpc.SeederCommandStatus{
			Database: rpc.SeederCommandStatusDB{Online: dbOnline, Master: dbMaster},
			Node:     rpc.SeederCommandStatusNode{Online: nodeOnline, Alias: "x"},
		}
	}

	testSequence := []struct {
		descr       string
		token       string
		status      rpc.SeederCommandStatus
		wantMaster  bool
		wantErr     error
		sleepBefore time.Duration
		extraChecks func()
	}{{
		descr:   "empty node alias",
		status:  rpc.SeederCommandStatus{},
		wantErr: errNoAlias,
	}, {
		descr:   "wrong clock",
		status:  rpc.SeederCommandStatus{Node: rpc.SeederCommandStatusNode{Alias: "x"}},
		wantErr: errLastUpdateTooOld,
	}, {
		token:  token1,
		descr:  "unhealthy before waitForMaster",
		status: testStatus(false, true, false),
	}, {
		token:  token1,
		descr:  "healthy not master before waitForMaster",
		status: testStatus(true, false, true),
	}, {
		sleepBefore: waitForMaster,
		token:       token1,
		descr:       "not master after waitForMaster",
		status:      testStatus(true, false, true),
		wantMaster:  true,
	}, {
		token:  token2,
		descr:  "update from other non-master server",
		status: testStatus(true, false, true),
	}, {
		token:      token1,
		descr:      "update from master server",
		status:     testStatus(true, true, true),
		wantMaster: true,
	}, {
		token:  token2,
		descr:  "non-master goes offline", // No change to master
		status: testStatus(false, false, false),
	}, {
		token:  token2,
		descr:  "non-master comes back online", // No change to master
		status: testStatus(true, false, true),
	}, {
		token:  token2,
		descr:  "non-master signals master", // master changes only after offlineTime
		status: testStatus(true, true, true),
	}, {
		sleepBefore: offlineLimit,
		token:       token2,
		descr:       "non-master signals master after timeout",
		status:      testStatus(true, true, true),
		wantMaster:  true,
	}, {
		token:  token1,
		descr:  "status update from previous master",
		status: testStatus(true, false, true),
	}, {
		token:      token2,
		descr:      "master signals db offline", // Still master
		status:     testStatus(false, true, true),
		wantMaster: true,
	}, {
		sleepBefore: 100 * time.Millisecond,
		token:       token2,
		descr:       "master status update db still offline", // Still master
		status:      testStatus(false, true, true),
		wantMaster:  true,
	}, {
		sleepBefore: offlineLimit,
		token:       token2,
		descr:       "master db still offline gets demoted",
		status:      testStatus(false, true, true),
		wantMaster:  false,
	}, {
		token:  token2,
		descr:  "master db still offline",
		status: testStatus(false, true, true),
	}, {
		token:      token1,
		descr:      "token1 instructed to become master",
		status:     testStatus(true, false, true),
		wantMaster: true,
	}, {
		token:      token1,
		descr:      "token1 confirmed became master",
		status:     testStatus(true, true, true),
		wantMaster: true,
	}, {
		token:      token1,
		descr:      "master dcrlnd went offline",
		status:     testStatus(true, true, false),
		wantMaster: true,
	}, {
		sleepBefore: offlineLimit,
		token:       token2,
		descr:       "token2 promoted to master due to master's dcrlnd offline",
		status:      testStatus(true, false, true),
		wantMaster:  true,
	}}

	for _, ts := range testSequence {
		if ts.sleepBefore != 0 {
			time.Sleep(ts.sleepBefore)
			t.Logf("Slept for %s", ts.sleepBefore)
		}

		// Fill the status update time (if not testing error cases).
		if ts.wantErr == nil {
			ts.status.LastUpdated = time.Now().Unix()
		}

		t.Log(ts.descr)
		gotMaster, gotErr := s.processStatusUpdate(ts.token, ts.status)
		assert.ErrorIs(t, gotErr, ts.wantErr)
		assert.DeepEqual(t, gotMaster, ts.wantMaster)

		// If indicated as master, then it should be the master.
		if gotMaster {
			s.mtx.Lock()
			gotMasterToken := s.serverMaster.token
			s.mtx.Unlock()
			assert.DeepEqual(t, gotMasterToken, ts.token)
		}

		if ts.extraChecks != nil {
			ts.extraChecks()
		}

	}

}
