package seederclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
)

type DialFunc func(context.Context, string, string) (net.Conn, error)

// chooseServer chooses the correct server based on the reply from the seeder.
func chooseServer(apiRes rpc.SeederClientAPI) string {
	for _, sg := range apiRes.ServerGroups {
		if !sg.IsMaster || !sg.Online {
			continue
		}

		return sg.Server
	}
	return ""
}

var ErrNoServers = errors.New("seeder returned no master servers")

// QuerySeeder queries a BR seeder service and returns the address of an active
// BR server instance.
func QuerySeeder(ctx context.Context, apiURL string, dialFunc DialFunc) (string, error) {
	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: dialFunc,
		},
		Timeout: time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to make a seeder request: %w", err)
	}
	rep, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query seeder: %w", err)
	}
	defer rep.Body.Close()

	if rep.StatusCode != 200 {
		return "", fmt.Errorf("seeder returned %v", rep.Status)
	}
	body, err := io.ReadAll(rep.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read seeder response: %w", err)
	}
	var api rpc.SeederClientAPI
	if err = json.Unmarshal(body, &api); err != nil {
		return "", fmt.Errorf("failed to unmarshal seeder response: %w", err)
	}
	server := chooseServer(api)
	if server == "" {
		return "", ErrNoServers
	}
	return server, nil
}
