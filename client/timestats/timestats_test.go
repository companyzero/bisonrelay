package timestats

import (
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
)

func TestTracker(t *testing.T) {
	n := 200
	tr := NewTracker(n)
	ms := time.Millisecond

	// Uniform distribution.
	for i := 0; i < n; i++ {
		tr.Add(time.Duration(i) * 100 * ms)
	}

	got := tr.Quantiles()
	t.Logf("%s", spew.Sdump(got))
}
