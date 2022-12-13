package multipriq

import "testing"

func testDistribution(t *testing.T, prios []uint) {
	q := new(MultiPriorityQueue)

	nbPops := 1000

	// Fill msgs on each prio.
	for _, pri := range prios {
		for i := 0; i < nbPops+1; i++ {
			q.Push(pri, pri)
		}
	}

	// Pop 200 msgs.
	msgCount := make(map[uint]int)
	for i := 0; i < nbPops; i++ {
		gotPri := q.Pop().(uint)
		msgCount[gotPri] += 1
	}

	// For each priority, we expect a fraction of the first priority.
	for _, pri := range prios {
		nb := msgCount[pri]
		ratio1 := float64(nb) / float64(msgCount[0]) * 100
		ratioPops := float64(nb) / float64(nbPops) * 100
		t.Logf("Priority %d: %d (%.2f%% ; %.2f%%)", pri, nb, ratio1, ratioPops)
	}
}

func TestDistribution(t *testing.T) {
	tests := []struct {
		name string
		prio []uint
	}{{
		name: "all distr",
		prio: []uint{0, 1, 2, 3, 4},
	}, {
		name: "1 to 4",
		prio: []uint{1, 2, 3, 4},
	}, {
		name: "2 to 4",
		prio: []uint{2, 3, 4},
	}, {
		name: "0 and 4",
		prio: []uint{0, 4},
	}, {
		name: "1 and 4",
		prio: []uint{1, 4},
	}, {
		name: "2 and 3",
		prio: []uint{2, 3},
	}, {
		name: "2, 3, 4",
		prio: []uint{2, 3, 4},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testDistribution(t, tc.prio)
		})
	}
}

func TestPeekEqualsPop(t *testing.T) {
	q := new(MultiPriorityQueue)
	v := "some value"
	q.Push(v, 0)
	q.Push("other value", 0)
	q.Push("third value", 0)
	gotPeek := q.Peek()
	if gotPeek != v {
		t.Fatalf("Unexpected peek value: got %v, want %v",
			v, gotPeek)
	}
	gotPush := q.Pop()
	if gotPush != v {
		t.Fatalf("Unexpected pop value: got %v, want %v",
			v, gotPush)
	}
}

func TestPopsAll(t *testing.T) {
	q := new(MultiPriorityQueue)
	prios := []uint{0, 1, 2, 3, 4}

	for _, pri := range prios {
		for i := uint(0); i < (pri+1)*100; i++ {
			q.Push(pri, pri)
		}
	}

	msgCount := make(map[uint]uint)
	for q.Len() > 0 {
		gotPri := q.Pop().(uint)
		msgCount[gotPri] += 1
	}

	for _, pri := range prios {
		gotCount := msgCount[pri]
		wantCount := (pri + 1) * 100
		if gotCount != wantCount {
			t.Fatalf("unexpected count for priority %d: got %d, want %d",
				pri, gotCount, wantCount)
		}
	}
}

var noElidePopBench struct{}

func BenchmarkPop(b *testing.B) {
	q := new(MultiPriorityQueue)
	prios := []uint{0, 1, 2, 3, 4}

	for i := 0; i < b.N; i++ {
		q.Push(struct{}{}, prios[i%len(prios)])
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		noElidePopBench = q.Pop().(struct{})
	}
}

func BenchmarkPush(b *testing.B) {
	q := new(MultiPriorityQueue)
	prios := []uint{0, 1, 2, 3, 4}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Push(struct{}{}, prios[i%len(prios)])
	}
}
