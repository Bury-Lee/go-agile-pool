package idle

import (
	"container/heap"
	"sync/atomic"
	"time"
)

// minHeapInner implements heap.Interface for []T.
type minHeapInner[T Dated] []T

func (h minHeapInner[T]) Len() int           { return len(h) }
func (h minHeapInner[T]) Less(i, j int) bool { return h[i].DatedTime().Before(h[j].DatedTime()) }
func (h minHeapInner[T]) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *minHeapInner[T]) Push(x any) {
	*h = append(*h, x.(T))
}

func (h *minHeapInner[T]) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	var zero T
	old[n-1] = zero // avoid memory leak
	*h = old[:n-1]
	return x
}

// MinHeap implements IdleContainer using a binary min-heap
// ordered by DatedTime. The element with the smallest (oldest)
// DatedTime is always at the root, making expiration cleanup efficient.
type MinHeap[T Dated] struct {
	inner minHeapInner[T]
	size  int64
}

// NewMinHeap creates a new empty MinHeap.
func NewMinHeap[T Dated]() *MinHeap[T] {
	h := &MinHeap[T]{
		inner: make(minHeapInner[T], 0),
	}
	heap.Init(&h.inner)
	return h
}

// Add pushes an element into the heap. O(log n).
func (h *MinHeap[T]) Add(w T) {
	heap.Push(&h.inner, w)
	atomic.AddInt64(&h.size, 1)
}

// Pop removes and returns the element with the smallest DatedTime.
// Returns the zero value of T if the heap is empty. O(log n).
func (h *MinHeap[T]) Pop() T {
	if h.inner.Len() == 0 {
		var zero T
		return zero
	}
	w := heap.Pop(&h.inner).(T)
	atomic.AddInt64(&h.size, -1)
	return w
}

// RemoveExpired removes all elements whose DatedTime + expiry <= now.
// Since the root has the smallest DatedTime, if the root is not expired,
// no other element can be expired, so we can stop immediately.
// O(k log n) where k is the number of expired elements.
func (h *MinHeap[T]) RemoveExpired(now time.Time, expiry time.Duration) int {
	removed := 0
	for h.inner.Len() > 0 {
		root := h.inner[0]
		if root.DatedTime().Add(expiry).After(now) {
			break
		}
		h.Pop()
		removed++
	}
	return removed
}

// Len returns the number of elements in the heap.
func (h *MinHeap[T]) Len() int64 {
	return atomic.LoadInt64(&h.size)
}
