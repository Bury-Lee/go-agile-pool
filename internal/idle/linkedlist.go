package idle

import (
	"sync/atomic"
	"time"
)

// LinkedList implements IdleContainer using a doubly linked list (FIFO).
type LinkedList[T Dated] struct {
	head   *llNode[T]
	tail   *llNode[T]
	length int64
}

type llNode[T Dated] struct {
	val  T
	next *llNode[T]
	prev *llNode[T]
}

func newLLNode[T Dated](val T) *llNode[T] {
	return &llNode[T]{
		val: val,
	}
}

// NewLinkedList creates a new empty LinkedList.
func NewLinkedList[T Dated]() *LinkedList[T] {
	return &LinkedList[T]{}
}

// Add adds an element to the tail of the linked list.
func (ll *LinkedList[T]) Add(w T) {
	node := newLLNode(w)
	if ll.head == nil && ll.tail == nil {
		ll.head = node
		ll.tail = node
		atomic.AddInt64(&ll.length, 1)
		return
	}
	prev := ll.tail
	ll.tail.next = node
	ll.tail = ll.tail.next
	ll.tail.prev = prev
	atomic.AddInt64(&ll.length, 1)
}

// Pop removes and returns the element at the head of the linked list.
// Returns the zero value of T if the list is empty.
func (ll *LinkedList[T]) Pop() T {
	if ll.head == nil {
		var zero T
		return zero
	}
	val := ll.head.val
	if ll.head == ll.tail {
		ll.head, ll.tail = nil, nil
	} else {
		ll.head = ll.head.next
		ll.head.prev = nil
	}
	atomic.AddInt64(&ll.length, -1)
	return val
}

// RemoveExpired removes all elements whose DatedTime + expiry <= now.
// The linked list is FIFO by insertion order, but DatedTime is not monotonic
// with insertion order (an element finishing a long task may have a newer
// DatedTime than an element inserted after it). Therefore, a full traversal
// is required.
func (ll *LinkedList[T]) RemoveExpired(now time.Time, expiry time.Duration) int {
	removed := 0
	node := ll.head
	for node != nil {
		next := node.next // save next before potential removal
		if !node.val.DatedTime().Add(expiry).After(now) {
			// Element is expired, remove this node
			ll.removeNode(node)
			removed++
		}
		node = next
	}
	return removed
}

// removeNode removes the given node from the linked list.
func (ll *LinkedList[T]) removeNode(node *llNode[T]) {
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		ll.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		ll.tail = node.prev
	}
	node.prev = nil
	node.next = nil
	atomic.AddInt64(&ll.length, -1)
}

// Len returns the number of elements in the linked list.
func (ll *LinkedList[T]) Len() int64 {
	return atomic.LoadInt64(&ll.length)
}
