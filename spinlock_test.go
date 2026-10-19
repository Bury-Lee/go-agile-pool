package agilepool

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestSpinLockLockUnlock(t *testing.T) {
	locker := NewSpinLock()
	if locker == nil {
		t.Fatal("NewSpinLock() returned nil")
	}
	locker.Lock()
	locker.Unlock()

	s := newSpinLock()
	if s == nil {
		t.Fatal("newSpinLock() returned nil")
	}
	s.Lock()
	s.Unlock()
	// Unlocking an already-free lock is a plain atomic store and must not panic.
	s.Unlock()
}

func TestSpinLockMutualExclusion(t *testing.T) {
	const (
		goroutines = 8
		iterations = 500
	)

	locker := NewSpinLock()
	var counter int64
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				locker.Lock()
				counter++
				locker.Unlock()
			}
		}()
	}
	wg.Wait()

	if got, want := counter, int64(goroutines*iterations); got != want {
		t.Fatalf("counter = %d, want %d", got, want)
	}
}

func TestSpinLockContendedLockWaitsForHolder(t *testing.T) {
	s := newSpinLock()
	s.Lock()

	var acquired atomic.Bool
	done := make(chan struct{})
	go func() {
		s.Lock()
		acquired.Store(true)
		s.Unlock()
		close(done)
	}()

	// Give the contending goroutine time to enter the spin loop.
	for i := 0; i < 100; i++ {
		if acquired.Load() {
			break
		}
	}
	if acquired.Load() {
		t.Fatal("lock acquired while still held")
	}

	s.Unlock()
	<-done
	if !acquired.Load() {
		t.Fatal("contending goroutine did not acquire the lock after unlock")
	}
}
