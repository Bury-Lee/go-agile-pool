package idle

import (
	"sync/atomic"
	"time"
)

const defaultRingQueueCapacity = 64

// RingQueue implements IdleContainer using a circular buffer (ring queue).
// Add and Pop are both O(1) operations. RemoveExpired requires a full scan.
// Not safe for concurrent use; the caller (Pool) serializes access.
type RingQueue[T Dated] struct {
	buf    []T
	head   int
	tail   int
	length int64
}

// NewRingQueue creates a new empty RingQueue.
func NewRingQueue[T Dated]() *RingQueue[T] {
	return &RingQueue[T]{
		buf: make([]T, defaultRingQueueCapacity),
	}
}

// grow doubles the buffer capacity and re-arranges elements in FIFO order.
func (rq *RingQueue[T]) grow() {
	n := int(atomic.LoadInt64(&rq.length))
	cap := len(rq.buf)

	newCap := cap * 2
	newBuf := make([]T, newCap)

	// Copy elements in FIFO order from the circular buffer
	for i := 0; i < n; i++ {
		newBuf[i] = rq.buf[(rq.head+i)%cap]
	}

	rq.head = 0
	rq.tail = n
	rq.buf = newBuf
}

// Add appends an element to the tail of the ring queue. O(1) amortized.
func (rq *RingQueue[T]) Add(w T) {
	if len(rq.buf) == int(atomic.LoadInt64(&rq.length)) {
		rq.grow()
	}
	rq.buf[rq.tail] = w
	rq.tail = (rq.tail + 1) % len(rq.buf)
	atomic.AddInt64(&rq.length, 1)
}

// Pop removes and returns the element at the head of the ring queue.
// Returns the zero value of T if the queue is empty. O(1).
func (rq *RingQueue[T]) Pop() T {
	if atomic.LoadInt64(&rq.length) == 0 {
		var zero T
		return zero
	}
	w := rq.buf[rq.head]
	var zero T
	rq.buf[rq.head] = zero
	rq.head = (rq.head + 1) % len(rq.buf)
	atomic.AddInt64(&rq.length, -1)
	return w
}

// RemoveExpired removes all elements whose DatedTime + expiry <= now.
// Since DatedTime is not monotonic with insertion order, a full scan is
// required. Survivors are compacted in place (zero allocations): the write
// position never passes the scan position, so a write target has always
// already been read, which keeps FIFO order without a temporary slice.
// O(n) where n is the number of idle elements.
func (rq *RingQueue[T]) RemoveExpired(now time.Time, expiry time.Duration) int {
	n := int(atomic.LoadInt64(&rq.length))
	if n == 0 {
		return 0
	}

	cutoff := now.Add(-expiry)
	cap := len(rq.buf)

	// In-place compaction: write survivors into the ring buffer positions
	// starting at head. writeIdx (the number of survivors written so far)
	// never exceeds the scan position i, so the write target has always
	// been read already — no temp slice needed.
	writeIdx := 0
	for i := 0; i < n; i++ {
		idx := (rq.head + i) % cap
		w := rq.buf[idx]
		if w.DatedTime().After(cutoff) {
			if writeIdx != i {
				rq.buf[(rq.head+writeIdx)%cap] = w
			}
			writeIdx++
		}
	}

	// Clear the tail slots that used to hold removed elements.
	var zero T
	for i := writeIdx; i < n; i++ {
		rq.buf[(rq.head+i)%cap] = zero
	}

	removed := n - writeIdx
	rq.tail = (rq.head + writeIdx) % cap
	atomic.AddInt64(&rq.length, -int64(removed))
	return removed
}

// Len returns the number of elements in the ring queue.
func (rq *RingQueue[T]) Len() int64 {
	return atomic.LoadInt64(&rq.length)
}
