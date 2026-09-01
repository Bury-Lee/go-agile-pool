package idle

import (
	"sync/atomic"
	"time"
)

// Slice implements IdleContainer using a dynamic array (slice).
// Elements are stored in FIFO order by insertion time, matching the behavior
// of LinkedList. Add appends to the tail, Pop removes from the head.
// Not safe for concurrent use; the caller (Pool) serializes access.
type Slice[T Dated] struct {
	workers []T
	length  int64
}

// NewSlice creates a new empty Slice.
func NewSlice[T Dated]() *Slice[T] {
	return &Slice[T]{
		workers: make([]T, 0),
	}
}

// Add appends an element to the tail of the slice. O(1) amortized.
func (s *Slice[T]) Add(w T) {
	s.workers = append(s.workers, w)
	atomic.AddInt64(&s.length, 1)
}

// Pop removes and returns the element at the head of the slice (FIFO).
// Returns the zero value of T if the slice is empty. O(n) due to shifting elements.
func (s *Slice[T]) Pop() T {
	if s.Len() == 0 {
		var zero T
		return zero
	}
	w := s.workers[0]
	var zero T
	s.workers[0] = zero
	s.workers = s.workers[1:]
	atomic.AddInt64(&s.length, -1)
	return w
}

// RemoveExpired removes all elements whose DatedTime + expiry <= now.
// Elements are ordered by insertion time, which does not guarantee monotonic
// DatedTime values, so all elements must be scanned. Survivors retain FIFO
// order. O(n) where n is the number of idle elements.
func (s *Slice[T]) RemoveExpired(now time.Time, expiry time.Duration) int {
	if s.Len() == 0 {
		return 0
	}

	cutoff := now.Add(-expiry)
	originalLen := len(s.workers)
	survivors := s.workers[:0]
	for _, w := range s.workers {
		if w.DatedTime().After(cutoff) {
			survivors = append(survivors, w)
		}
	}

	removed := originalLen - len(survivors)
	if removed > 0 {
		var zero T
		for i := len(survivors); i < originalLen; i++ {
			s.workers[i] = zero
		}
		s.workers = survivors
		atomic.AddInt64(&s.length, -int64(removed))
	}

	return removed
}

// Len returns the number of elements in the slice.
func (s *Slice[T]) Len() int64 {
	return atomic.LoadInt64(&s.length)
}
