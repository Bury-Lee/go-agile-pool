package idle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------- Unit tests for RingQueue ----------

func TestRingQueueAddPop(t *testing.T) {
	rq := NewRingQueue[*testWorker]()
	assert.Equal(t, int64(0), rq.Len())

	w1 := newTestWorker(time.Now())
	w2 := newTestWorker(time.Now())
	w3 := newTestWorker(time.Now())

	rq.Add(w1)
	assert.Equal(t, int64(1), rq.Len())

	rq.Add(w2)
	assert.Equal(t, int64(2), rq.Len())

	rq.Add(w3)
	assert.Equal(t, int64(3), rq.Len())

	// FIFO order
	assert.Same(t, w1, rq.Pop())
	assert.Equal(t, int64(2), rq.Len())
	assert.Same(t, w2, rq.Pop())
	assert.Equal(t, int64(1), rq.Len())
	assert.Same(t, w3, rq.Pop())
	assert.Equal(t, int64(0), rq.Len())
}

func TestRingQueuePopEmpty(t *testing.T) {
	rq := NewRingQueue[*testWorker]()
	assert.Nil(t, rq.Pop())
	assert.Equal(t, int64(0), rq.Len())
}

func TestRingQueueGrow(t *testing.T) {
	rq := NewRingQueue[*testWorker]()
	workers := make([]*testWorker, defaultRingQueueCapacity+1)
	for i := range workers {
		workers[i] = newTestWorker(time.Now())
	}

	// Fill the buffer to exactly capacity (triggers grow on the last Add)
	for _, w := range workers {
		rq.Add(w)
	}
	assert.Equal(t, int64(len(workers)), rq.Len())

	// Verify FIFO order after growth
	for _, w := range workers {
		assert.Same(t, w, rq.Pop())
	}
	assert.Equal(t, int64(0), rq.Len())

}

func TestRingQueueAddPopMultipleGrows(t *testing.T) {
	rq := NewRingQueue[*testWorker]()
	const count = 1000
	workers := make([]*testWorker, count)
	for i := range workers {
		workers[i] = newTestWorker(time.Now())
	}

	for _, w := range workers {
		rq.Add(w)
	}
	assert.Equal(t, int64(count), rq.Len())

	for _, w := range workers {
		assert.Same(t, w, rq.Pop())
	}
	assert.Equal(t, int64(0), rq.Len())
}

func TestRingQueueRemoveExpiredNone(t *testing.T) {
	rq := NewRingQueue[*testWorker]()
	now := time.Now()

	w1 := &testWorker{lastActiveAt: now}
	w2 := &testWorker{lastActiveAt: now}
	rq.Add(w1)
	rq.Add(w2)

	removed := rq.RemoveExpired(now, time.Hour) // expiry far in the future
	assert.Equal(t, 0, removed)
	assert.Equal(t, int64(2), rq.Len())

	// Verify FIFO order preserved
	assert.Same(t, w1, rq.Pop())
	assert.Same(t, w2, rq.Pop())
}

func TestRingQueueRemoveExpiredAll(t *testing.T) {
	rq := NewRingQueue[*testWorker]()
	now := time.Now()
	past := now.Add(-2 * time.Second)

	for i := 0; i < 10; i++ {
		rq.Add(&testWorker{lastActiveAt: past})
	}

	removed := rq.RemoveExpired(now, time.Second)
	assert.Equal(t, 10, removed)
	assert.Equal(t, int64(0), rq.Len())
}

func TestRingQueueRemoveExpiredPartial(t *testing.T) {
	rq := NewRingQueue[*testWorker]()
	now := time.Now()

	// Add interleaved: expired, alive, expired, alive, ...
	workers := make([]*testWorker, 10)
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			workers[i] = &testWorker{lastActiveAt: now.Add(-2 * time.Second)} // expired
		} else {
			workers[i] = &testWorker{lastActiveAt: now} // alive
		}
		rq.Add(workers[i])
	}

	removed := rq.RemoveExpired(now, time.Second)
	assert.Equal(t, 5, removed)
	assert.Equal(t, int64(5), rq.Len())

	// Verify survivors are still in FIFO order (indices 1,3,5,7,9)
	for i := 1; i < 10; i += 2 {
		assert.Same(t, workers[i], rq.Pop())
	}
}

func TestRingQueueRemoveExpiredWrapAround(t *testing.T) {
	// Force a smaller capacity buffer for controlled wrap test
	rq := &RingQueue[*testWorker]{
		buf: make([]*testWorker, 8),
	}
	now := time.Now()

	// Add 5 workers (indices 0-4)
	w0 := &testWorker{lastActiveAt: now}
	w1 := &testWorker{lastActiveAt: now}
	w2 := &testWorker{lastActiveAt: now.Add(-2 * time.Second)} // expired
	w3 := &testWorker{lastActiveAt: now}
	w4 := &testWorker{lastActiveAt: now}
	rq.Add(w0)
	rq.Add(w1)
	rq.Add(w2)
	rq.Add(w3)
	rq.Add(w4)
	assert.Equal(t, int64(5), rq.Len())

	// Pop 3 (w0,w1,w2), now head=3, tail=5, len=2
	rq.Pop() // w0
	rq.Pop() // w1
	rq.Pop() // w2 (expired, popped before cleaner handles it)
	assert.Equal(t, int64(2), rq.Len())

	// Add 5 more (w5-w9), tail wraps: start at 5, add 5 → tail=(5+5)%8=2, head=3
	w5 := &testWorker{lastActiveAt: now.Add(-2 * time.Second)} // expired
	w6 := &testWorker{lastActiveAt: now}
	w7 := &testWorker{lastActiveAt: now}
	w8 := &testWorker{lastActiveAt: now.Add(-2 * time.Second)} // expired
	w9 := &testWorker{lastActiveAt: now}
	rq.Add(w5)
	rq.Add(w6)
	rq.Add(w7)
	rq.Add(w8)
	rq.Add(w9)

	// Now we have 7 elements in FIFO order:
	// buf[3]=w3, buf[4]=w4, buf[5]=w5, buf[6]=w6, buf[7]=w7, buf[0]=w8, buf[1]=w9
	// head=3, tail=2, len=7
	assert.Equal(t, int64(7), rq.Len())

	// RemoveExpired should remove w5 and w8 (both expired)
	removed := rq.RemoveExpired(now, time.Second)
	assert.Equal(t, 2, removed) // w5 and w8
	assert.Equal(t, int64(5), rq.Len())

	// Verify survivors: w3, w4, w6, w7, w9 in FIFO order
	assert.Same(t, w3, rq.Pop())
	assert.Same(t, w4, rq.Pop())
	assert.Same(t, w6, rq.Pop())
	assert.Same(t, w7, rq.Pop())
	assert.Same(t, w9, rq.Pop())
	assert.Nil(t, rq.Pop())
}
