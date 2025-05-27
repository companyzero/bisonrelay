package audio

import "container/heap"

// tsBufferItem is one item from tsBufferQueue.
type tsBufferItem struct {
	ts   uint32
	data []byte
}

// tsBufferItemSlice is a slice of buffer items that satisfies heap.Interface.
type tsBufferItemSlice []*tsBufferItem

func (s tsBufferItemSlice) Len() int           { return len(s) }
func (s tsBufferItemSlice) Less(i, j int) bool { return s[i].ts < s[j].ts }
func (s tsBufferItemSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s *tsBufferItemSlice) Push(x any)        { *s = append(*s, x.(*tsBufferItem)) }
func (s *tsBufferItemSlice) Pop() any {
	old := *s
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*s = old[0 : n-1]
	return item
}

// tsBufferQueue maintains an ordered queue of buffers (by timestamp) to be
// decoded.
type tsBufferQueue struct {
	itemsLen int
	items    *tsBufferItemSlice

	// unused item objects that may be reused (useful for reducing allocs).
	unused []*tsBufferItem
}

// len returns the number of items in the queue.
func (q *tsBufferQueue) len() int {
	return q.itemsLen
}

// firstTs returns the timestamp of the first item or 0.
func (q *tsBufferQueue) firstTs() uint32 {
	if q.items.Len() > 0 {
		return (*q.items)[0].ts
	}
	return 0
}

// enq enqueues one item in the queue.
func (q *tsBufferQueue) enq(data []byte, ts uint32) {
	// Reuse or alloc new item.
	var item *tsBufferItem
	if len(q.unused) > 0 {
		i := len(q.unused) - 1
		item = q.unused[i]
		q.unused[i] = nil
		q.unused = q.unused[:i]
	} else {
		item = new(tsBufferItem)
	}

	// Store data in priority queue.
	item.data, item.ts = data, ts
	heap.Push(q.items, item)
	q.itemsLen++
}

// deq dequeues one item from the queue.
func (q *tsBufferQueue) deq() (data []byte, ts uint32, ok bool) {
	// Early return when empty.
	if q.items.Len() == 0 {
		data, ts, ok = nil, 0, false
		return
	}

	// Pop from priority queue.
	item := heap.Pop(q.items).(*tsBufferItem)
	data, ts, ok = item.data, item.ts, true

	// Store item pointer for reuse.
	item.data, item.ts = nil, 0
	q.unused = append(q.unused, item)
	q.itemsLen--
	return
}

// newTsBufferQueue initializes a new buffer queue.
func newTsBufferQueue(sizeHint int) *tsBufferQueue {
	unused := make([]*tsBufferItem, sizeHint)
	for i := 0; i < sizeHint; i++ {
		unused[i] = &tsBufferItem{}
	}
	return &tsBufferQueue{
		items:  &tsBufferItemSlice{},
		unused: unused,
	}
}
