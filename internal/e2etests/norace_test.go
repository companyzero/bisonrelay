//go:build !race

package e2etests

import "time"

// sleep for d duration or m*d if running in race mode.
func sleep(d time.Duration, m int) {
	time.Sleep(d)
}

// chooseTimeout chooses which timeout to use based on whether we are running
// in -race mode or not.
func chooseTimeout(norace, race time.Duration) time.Duration {
	return norace
}
