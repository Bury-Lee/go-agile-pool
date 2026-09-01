package idle

import (
	"testing"
	"time"
)

// TestMinHeap_Add verifies that Add updates the heap length correctly.
func TestMinHeap_Add(t *testing.T) {
	tests := []struct {
		name     string
		addCount int
		wantLen  int64
	}{
		{
			name:     "add one worker",
			addCount: 1,
			wantLen:  1,
		},
		{
			name:     "add multiple workers",
			addCount: 5,
			wantLen:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewMinHeap[*testWorker]()
			now := time.Unix(1_700_000_000, 0)

			for i := 0; i < tt.addCount; i++ {
				h.Add(&testWorker{
					lastActiveAt: now.Add(
						time.Duration(i) * time.Second),
				})
			}

			if got := h.Len(); got != tt.wantLen {
				t.Errorf("Len() = %v, want %v", got, tt.wantLen)
			}
		})
	}
}

// TestMinHeap_Pop verifies empty-heap behavior and oldest-worker-first ordering.
func TestMinHeap_Pop(t *testing.T) {
	t.Run("pop from empty heap", func(t *testing.T) {
		h := NewMinHeap[*testWorker]()

		if got := h.Pop(); got != nil {
			t.Errorf("Pop() = %v, want nil", got)
		}

		if got := h.Len(); got != 0 {
			t.Errorf("Len() = %v, want %v", got, 0)
		}
	})

	t.Run("pop oldest worker first", func(t *testing.T) {
		h := NewMinHeap[*testWorker]()
		now := time.Unix(1_700_000_000, 0)

		oldest := &testWorker{
			lastActiveAt: now.Add(-30 * time.Second),
		}

		middle := &testWorker{
			lastActiveAt: now.Add(-20 * time.Second),
		}

		newest := &testWorker{
			lastActiveAt: now.Add(-10 * time.Second),
		}

		// Deliberately add workers out of time order.
		h.Add(middle)
		h.Add(newest)
		h.Add(oldest)

		wants := []*testWorker{oldest, middle, newest}

		for i, want := range wants {
			got := h.Pop()
			if got != want {
				t.Errorf("Pop() order %d = %v, want %v",
					i,
					got,
					want)
			}
		}

		if got := h.Len(); got != 0 {
			t.Errorf(
				"Len() after all pops = %v, want 0",
				got,
			)
		}

		if got := h.Pop(); got != nil {
			t.Errorf(
				"Pop() after drain = %v, want nil",
				got,
			)
		}
	})
}

// TestMinHeap_RemoveExpired verifies expiration removal and boundaries.
func TestMinHeap_RemoveExpired(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	expiry := 5 * time.Second

	t.Run("no workers", func(t *testing.T) {
		h := NewMinHeap[*testWorker]()

		removed := h.RemoveExpired(now, expiry)

		if removed != 0 {
			t.Errorf("RemoveExpired() = %v, want 0", removed)
		}

		if got := h.Len(); got != 0 {
			t.Errorf("Len() = %v, want 0", got)
		}
	})

	t.Run("remove expired and preserve active workers", func(t *testing.T) {
		h := NewMinHeap[*testWorker]()

		expiredOld := &testWorker{
			lastActiveAt: now.Add(-10 * time.Second),
		}
		// A worker exactly at the cutoff is expired, while one nanosecond
		// after the cutoff remains active.
		expiredBoundary := &testWorker{
			lastActiveAt: now.Add(-5 * time.Second),
		}
		activeBoundary := &testWorker{
			lastActiveAt: now.Add(
				-5*time.Second + time.Nanosecond,
			),
		}
		activeNew := &testWorker{
			lastActiveAt: now.Add(-1 * time.Second),
		}

		// Add in mixed order to verify heap ordering.
		h.Add(activeNew)
		h.Add(expiredBoundary)
		h.Add(activeBoundary)
		h.Add(expiredOld)

		removed := h.RemoveExpired(now, expiry)

		if removed != 2 {
			t.Errorf(
				"RemoveExpired() = %v, want 2",
				removed,
			)
		}

		if got := h.Len(); got != 2 {
			t.Fatalf(
				"Len() after removal = %v, want 2",
				got,
			)
		}

		if got := h.Pop(); got != activeBoundary {
			t.Errorf(
				"first Pop() = %v, want activeBoundary",
				got,
			)
		}

		if got := h.Pop(); got != activeNew {
			t.Errorf(
				"second Pop() = %v, want activeNew",
				got,
			)
		}

		if got := h.Pop(); got != nil {
			t.Errorf(
				"Pop() after drain = %v, want nil",
				got,
			)
		}
	})
}

// TestMinHeap_RemoveExpired_AllActive verifies that cleanup stops
// without removing workers when the oldest worker is still active.
func TestMinHeap_RemoveExpired_AllActive(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	expiry := 5 * time.Second

	h := NewMinHeap[*testWorker]()

	older := &testWorker{
		lastActiveAt: now.Add(-4 * time.Second),
	}
	newer := &testWorker{
		lastActiveAt: now.Add(-1 * time.Second),
	}

	h.Add(older)
	h.Add(newer)

	removed := h.RemoveExpired(now, expiry)
	if removed != 0 {
		t.Errorf("RemoveExpired() = %d, want 0", removed)
	}

	if got := h.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}

	if got := h.Pop(); got != older {
		t.Errorf("first Pop() = %p, want %p", got, older)
	}

	if got := h.Pop(); got != newer {
		t.Errorf("second Pop() = %p, want %p", got, newer)
	}
}

// TestMinHeap_RemoveExpired_AllExpired verifies that cleanup can
// remove every worker and leave the heap in a valid empty state.
func TestMinHeap_RemoveExpired_AllExpired(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	expiry := 5 * time.Second
	h := NewMinHeap[*testWorker]()

	h.Add(&testWorker{
		lastActiveAt: now.Add(-20 * time.Second),
	})
	h.Add(&testWorker{
		lastActiveAt: now.Add(-10 * time.Second),
	})
	h.Add(&testWorker{
		lastActiveAt: now.Add(-5 * time.Second),
	})

	removed := h.RemoveExpired(now, expiry)

	if removed != 3 {
		t.Errorf("RemoveExpired() = %d, want 3", removed)
	}

	if got := h.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}

	if got := h.Pop(); got != nil {
		t.Errorf("Pop() = %v, want nil", got)
	}
}
