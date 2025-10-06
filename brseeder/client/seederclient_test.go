package seederclient

import (
	"testing"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
)

// TestChooseServer tests choosing the correct server.
func TestChooseServer(t *testing.T) {
	t.Parallel()

	// Helpers.
	server := func(server string, online bool, master bool) rpc.SeederServerGroup {
		return rpc.SeederServerGroup{
			Server:   server,
			Online:   online,
			IsMaster: master,
		}
	}
	apiRes := func(servers ...rpc.SeederServerGroup) rpc.SeederClientAPI {
		return rpc.SeederClientAPI{ServerGroups: servers}
	}

	tests := []struct {
		name   string
		apiRes rpc.SeederClientAPI
		want   string
	}{{
		name: "no servers",
		want: "",
	}, {
		name:   "one non-master server",
		apiRes: apiRes(server("s1", true, false)),
		want:   "",
	}, {
		name:   "one non-online server",
		apiRes: apiRes(server("s1", false, true)),
		want:   "",
	}, {
		name:   "multiple non-master offline servers",
		apiRes: apiRes(server("s1", false, true), server("s2", true, false)),
		want:   "",
	}, {
		name:   "one online master server",
		apiRes: apiRes(server("s1", true, true)),
		want:   "s1",
	}, {
		name:   "multiple online master server",
		apiRes: apiRes(server("s1", true, true), server("s2", true, true)),
		want:   "s1",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := chooseServer(tc.apiRes)
			assert.DeepEqual(t, got, tc.want)
		})
	}
}
