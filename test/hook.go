package main

import (
	"context"
	"sync/atomic"
	"time"

	agilepool "github.com/Yiming1997/agilePool"
)

// hookEnabled is set to true after setupHookTracking successfully registers
// callbacks. When false, readHookCounters returns all zeros (fast path).
var hookEnabled bool

// Atomic counters tracking task lifecycle via pool hooks.
var (
	submittedCount int64
	enqueuedCount  int64
	startedCount   int64
	completedCount int64
)

// Atomic accumulators for per-interval timing statistics.
// Each accumulator tracks the total duration spent in a phase across all
// completed tasks during the current sampling interval.  readTimingStats
// returns the totals and resets them for the next interval.
var (
	totalHandoff   int64 // nanoseconds
	totalQueueWait int64 // nanoseconds
	totalExec      int64 // nanoseconds
	totalTotal     int64 // nanoseconds
	timedTasks     int64 // number of completed tasks with timing enabled
)

// setupHookTracking registers task-lifecycle hooks on the pool.
//
// Two kinds of hooks are registered:
//   1. Counter hooks — track how many tasks pass through each stage.
//   2. TimingHook + completion accumulator — record timestamps at each
//      lifecycle stage and compute phase durations at completion.
//
// Timing data is accumulated into atomic int64 counters and can be read
// (and reset) via readTimingStats for per-interval CSV/JSON output.
func setupHookTracking(pool *agilepool.Pool) {
	hookEnabled = true

	// ---- Counter hooks ----
	pool.OnTaskSubmitted(func(ctx context.Context, task agilepool.Task) {
		atomic.AddInt64(&submittedCount, 1)
	})
	pool.OnTaskEnqueued(func(ctx context.Context, task agilepool.Task) {
		atomic.AddInt64(&enqueuedCount, 1)
	})
	pool.OnTaskStarted(func(ctx context.Context, task agilepool.Task) {
		atomic.AddInt64(&startedCount, 1)
	})

	// ---- TimingHook — records timestamps into agilepool.Context ----
	s, e, st, co := agilepool.TimingHook()
	pool.OnTaskSubmitted(s)
	pool.OnTaskEnqueued(e)
	pool.OnTaskStarted(st)
	pool.OnTaskCompleted(co)

	// ---- Completion hook — accumulate phase durations + release Context ----
	pool.OnTaskCompleted(func(ctx context.Context, task agilepool.Task, recovered any) {
		atomic.AddInt64(&completedCount, 1)

		pc := agilepool.PoolContextFrom(ctx)
		if pc == nil || !pc.IsTimingEnabled() {
			return
		}

		if h := pc.HandoffLatency(); h > 0 {
			atomic.AddInt64(&totalHandoff, int64(h))
		}
		if qw := pc.QueueWaitLatency(); qw > 0 {
			atomic.AddInt64(&totalQueueWait, int64(qw))
		}
		if ex := pc.ExecLatency(); ex > 0 {
			atomic.AddInt64(&totalExec, int64(ex))
		}
		if tl := pc.TotalLatency(); tl > 0 {
			atomic.AddInt64(&totalTotal, int64(tl))
		}
		atomic.AddInt64(&timedTasks, 1)

		// Return the Context to its pool for reuse.
		pc.Release()
	})
}

// readHookCounters returns a point-in-time snapshot of the four hook counters.
func readHookCounters() (submitted, enqueued, started, completed int64) {
	if !hookEnabled {
		return 0, 0, 0, 0
	}
	return atomic.LoadInt64(&submittedCount),
		atomic.LoadInt64(&enqueuedCount),
		atomic.LoadInt64(&startedCount),
		atomic.LoadInt64(&completedCount)
}

// TimingStats holds per-interval average latencies for all four phases.
type TimingStats struct {
	TimedTasks       int64
	AvgHandoffNs     int64
	AvgQueueWaitNs   int64
	AvgExecNs        int64
	AvgTotalNs       int64
}

// readTimingStats atomically reads and resets the timing accumulators.
// Call this once per sampling interval to get the averages since the
// last call.  Returns nil if hook tracking is disabled.
//
// Swapped-out values are accumulated into global (non-resetting) counters
// so that readCumulativeTimingStats can provide a full-run summary.
func readTimingStats() *TimingStats {
	if !hookEnabled {
		return nil
	}

	h := atomic.SwapInt64(&totalHandoff, 0)
	qw := atomic.SwapInt64(&totalQueueWait, 0)
	ex := atomic.SwapInt64(&totalExec, 0)
	tl := atomic.SwapInt64(&totalTotal, 0)
	n := atomic.SwapInt64(&timedTasks, 0)

	atomic.AddInt64(&cumHandoff, h)
	atomic.AddInt64(&cumQueueWait, qw)
	atomic.AddInt64(&cumExec, ex)
	atomic.AddInt64(&cumTotal, tl)
	atomic.AddInt64(&cumTimedTasks, n)

	if n == 0 {
		return &TimingStats{}
	}

	return &TimingStats{
		TimedTasks:     n,
		AvgHandoffNs:   h / n,
		AvgQueueWaitNs: qw / n,
		AvgExecNs:      ex / n,
		AvgTotalNs:     tl / n,
	}
}

// Global cumulative accumulators (never reset — for final summary).
var (
	cumHandoff    int64
	cumQueueWait  int64
	cumExec       int64
	cumTotal      int64
	cumTimedTasks int64
)

// readCumulativeTimingStats returns the average latencies across the
// entire run.  Call once at shutdown after all tasks have completed.
func readCumulativeTimingStats() *TimingStats {
	if !hookEnabled {
		return nil
	}
	n := atomic.LoadInt64(&cumTimedTasks)
	if n == 0 {
		return &TimingStats{}
	}
	return &TimingStats{
		TimedTasks:     n,
		AvgHandoffNs:   atomic.LoadInt64(&cumHandoff) / n,
		AvgQueueWaitNs: atomic.LoadInt64(&cumQueueWait) / n,
		AvgExecNs:      atomic.LoadInt64(&cumExec) / n,
		AvgTotalNs:     atomic.LoadInt64(&cumTotal) / n,
	}
}

// formatNs formats a nanosecond duration as a human-readable string.
func formatNs(ns int64) string {
	d := time.Duration(ns)
	switch {
	case d >= time.Second:
		return d.Round(time.Millisecond).String()
	case d >= time.Millisecond:
		return d.Round(time.Microsecond).String()
	default:
		return d.String()
	}
}
