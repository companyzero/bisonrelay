package multipriq

import (
	"container/list"
	"fmt"
)

// revPriorities is the reverse of the relative weights of the priorities.
//
// Note: TestDistribution can be used to verify any tweaks to these values under
// various loads in the priorities.
var revPriorities = []uint{128, 32, 8, 2, 1}

// MultiPriorityQueue is a probabilistic multi-priority queue: elements are
// pushed with a priority number (from 0 to 4) and are popped following a
// distribution that ensures elements from all priorities are (eventually)
// popped while preferring to pop elements with lower priority number.
//
// The distribution is empirically determined, but with a full queue, the
// popping rate approaches the following:
//
//	Priority   Rate of Pop
//	    0         50.10%
//	    1         37.50%
//	    2          9.30%
//	    3          2.40%
//	    4          0.70%
//
// The empty value is a valid MultiPriorityQueue. This structure is not safe for
// concurrent access.
type MultiPriorityQueue struct {
	// The queue is implemented as 5 different "lanes": each one corresponds
	// to one of the priority numbers (0 to 4). At any point in time, the
	// queue maintains an invariant where "l" points to the next lane from
	// which the next value to be popped must come from.
	//
	// After popping an element, the next lane to be sent from is selected
	// based on a probabilistic distribution governed by the 'revPriorities'
	// array.

	lanes [5]*list.List
	l     uint
	len   int

	// i is a counter used to determine the next msg to send. It is ok for
	// this value to overflow.
	i uint
}

// Push the given value to the queue, using the specified priority number.
//
// This function panics if pri is >= 5.
func (q *MultiPriorityQueue) Push(v interface{}, pri uint) {
	if pri >= uint(len(revPriorities)) {
		panic(fmt.Errorf("max priority is %d", len(revPriorities)-1))
	}

	l := q.lanes[pri]
	if l == nil {
		l = list.New()
		q.lanes[pri] = l
	}

	l.PushBack(v)
	if q.len == 0 {
		q.i = 0
		q.l = pri
	}
	q.len += 1
}

// Peek returns the next value of the queue (but does not remove it from the
// queue).
func (q *MultiPriorityQueue) Peek() interface{} {
	if q.len == 0 {
		return nil
	}

	return q.lanes[q.l].Front().Value
}

// advance finds the next lane from which to fetch a value. Must not be called
// on an empty queue.
func (q *MultiPriorityQueue) advance() {
	// Find the first lane where q.i % priority == 0. Disambiguate by
	// choosing the one with highest priority (i.e. lower priority number).
	//
	// This could be cleverer.
	for {
		q.i += 1
		for i, pri := range revPriorities {
			// Invert to get the priority number l
			l := len(revPriorities) - i - 1

			// Only consider lanes that have elements.
			lane := q.lanes[l]
			if lane == nil || lane.Len() == 0 {
				continue
			}

			if q.i%pri == 0 {
				q.l = uint(l)
				return
			}
		}
	}
}

// Pop returns the next value to be removed from the queue and removes it.
//
// Returns a nil interface when the queue is empty.
func (q *MultiPriorityQueue) Pop() interface{} {
	if q.len == 0 {
		return nil
	}

	// Pop element from lane.
	el := q.lanes[q.l].Front()
	v := el.Value
	q.lanes[q.l].Remove(el)
	q.len -= 1

	if q.len != 0 {
		q.advance()
	}

	return v
}

// Len returns the number of items in the queue.
func (q *MultiPriorityQueue) Len() int {
	return q.len
}
