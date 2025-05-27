package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/decred/slog"
)

// TestCanceledRunTerminates ensures running with a canceled context correctly
// terminates the client.
func TestCanceledRunTerminates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rnd := testRand(t)
	id := testID(t, rnd, "alice")
	cfg := Config{
		Dialer:        nil,
		DB:            testDB(t, id, nil),
		LocalIDIniter: fixedIDIniter(id),
		Logger:        func(string) slog.Logger { return slog.Disabled },
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	errChan := make(chan error)
	go func() { errChan <- c.Run(ctx) }()

	// Run with a canceled context.
	select {
	case err := <-errChan:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error. got %v, want %v",
				context.Canceled, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for Run() to complete")
	}
}
