package idle

import "time"

// IdleContainerType defines the type of data structure used for idle element management.
type IdleContainerType int8

const (
	// LinkedListType uses a doubly linked list (FIFO) for idle element management.
	LinkedListType IdleContainerType = iota
	// MinHeapType uses a min-heap ordered by DatedTime for idle element management.
	MinHeapType
	// SliceType uses a dynamic array (slice) with FIFO order for idle element management.
	SliceType
	// RingQueueType uses a ring buffer (circular buffer) for idle element management.
	// Add and Pop are both O(1), offering better Pop performance than SliceType.
	RingQueueType
	// TreapType uses a randomized treap ordered by DatedTime (LRU).
	// It is optimized for batch expiration cleanup of idle elements.
	TreapType
)

// Dated is the constraint for elements stored in an idle container.
// It exposes the last-active timestamp used for expiry cleanup.
type Dated interface {
	// DatedTime returns the last-active timestamp of the element.
	DatedTime() time.Time
}

// IdleContainer abstracts a data structure for managing idle elements.
// Implementations are NOT safe for concurrent use; the caller serializes
// access (e.g. via a lock).
type IdleContainer[T Dated] interface {
	// Add adds an element to the container.
	Add(T)
	// Pop removes and returns an element from the container.
	// Returns the zero value of T if the container is empty.
	Pop() T
	// RemoveExpired removes all elements whose DatedTime + expiry <= now.
	// Returns the number of elements removed.
	RemoveExpired(now time.Time, expiry time.Duration) int
	// Len returns the number of elements in the container.
	Len() int64
}
