package audio

import "container/heap"

// tsBufferItem is one item from tsBufferQueue.
type tsBufferItem struct {
	ts   uint32
	data *[]byte
	size int
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
func (q *tsBufferQueue) enq(data *[]byte, size int, ts uint32) {
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
	item.data, item.size, item.ts = data, size, ts
	heap.Push(q.items, item)
	q.itemsLen++
}

// deq dequeues one item from the queue.
func (q *tsBufferQueue) deq() (data *[]byte, size int, ts uint32, ok bool) {
	// Early return when empty.
	if q.items.Len() == 0 {
		data, ts, ok = nil, 0, false
		return
	}

	// Pop from priority queue.
	item := heap.Pop(q.items).(*tsBufferItem)
	data, size, ts, ok = item.data, item.size, item.ts, true

	// Store item pointer for reuse.
	item.data, item.size, item.ts = nil, 0, 0
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

type inputPacketQueue struct {
	items []inputPacket
	s     int // start
	e     int // end
	l     int // len
}

func (q *inputPacketQueue) isFull() bool {
	return q.l == len(q.items)
}

func (q *inputPacketQueue) enq(p inputPacket) {
	if q.isFull() {
		panic("cannot push on full queue")
	}
	q.items[q.e] = p
	q.e = (q.e + 1) % len(q.items)
	q.l++
}

func (q *inputPacketQueue) isEmpty() bool {
	return q.l == 0
}

func (q *inputPacketQueue) len() int {
	return q.l
}

func (q *inputPacketQueue) deq() (res inputPacket) {
	if q.isEmpty() {
		panic("cannot pop from empty queue")
	}

	res = q.items[q.s]
	q.s = (q.s + 1) % len(q.items)
	q.l--
	return
}

func (q *inputPacketQueue) peek() inputPacket {
	if q.isEmpty() {
		panic("cannot peek from empty queue")
	}

	return q.items[q.s]
}

func newInputPacketQueue(size int) *inputPacketQueue {
	return &inputPacketQueue{items: make([]inputPacket, size)}
}
