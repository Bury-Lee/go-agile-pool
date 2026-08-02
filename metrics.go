package agilepool

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// --- MetricsSnapshot ---

// MetricsSnapshot is a point-in-time snapshot of pool-level task counters
// and average lifecycle latencies.
type MetricsSnapshot struct {
	// Submitted is the total number of tasks accepted by the pool.
	Submitted int64

	// Started is the total number of tasks that began execution.
	Started int64

	// Completed is the total number of tasks that finished (success or panic).
	Completed int64

	// Failed is the total number of tasks that panicked.
	Failed int64

	// AvgHandoffLatency is the average time from submission to enqueueing.
	AvgHandoffLatency time.Duration

	// AvgQueueWaitLatency is the average time from enqueueing to execution start.
	AvgQueueWaitLatency time.Duration

	// AvgExecLatency is the average execution time (started → completed).
	AvgExecLatency time.Duration

	// AvgTotalLatency is the average end-to-end latency (submitted → completed).
	AvgTotalLatency time.Duration
}

// --- Metrics ---

// Metrics collects in-memory task lifecycle counters and average latencies.
// It registers TimingHook and count hooks on construction.
// For Prometheus, see PrometheusMetrics in prometheus.go.
type Metrics struct {
	pool *Pool

	submitted atomic.Int64
	started   atomic.Int64
	completed atomic.Int64
	failed    atomic.Int64

	mu              sync.Mutex
	totalHandoff    time.Duration
	totalQueueWait  time.Duration
	totalExec       time.Duration
	totalTotal      time.Duration
	completedN      int64
}

// NewMetrics creates a Metrics collector and registers TimingHook plus
// counter hooks on p.  Callers must still call ctx.EnableTiming() on
// each task's Context for latency tracking to take effect.
func NewMetrics(p *Pool) *Metrics {
	m := &Metrics{pool: p}

	// Register TimingHook to capture lifecycle timestamps.
	s, e, st, co := TimingHook()
	p.OnTaskSubmitted(s)
	p.OnTaskEnqueued(e)
	p.OnTaskStarted(st)
	p.OnTaskCompleted(co)

	// Register count hooks.
	p.OnTaskSubmitted(func(ctx context.Context, task Task) {
		m.submitted.Add(1)
	})
	p.OnTaskStarted(func(ctx context.Context, task Task) {
		m.started.Add(1)
	})
	p.OnTaskCompleted(func(ctx context.Context, task Task, recovered any) {
		m.completed.Add(1)
		if recovered != nil {
			m.failed.Add(1)
		}
		pc := PoolContextFrom(ctx)
		if pc == nil || !pc.IsTimingEnabled() {
			return
		}

		h := pc.HandoffLatency()
		qw := pc.QueueWaitLatency()
		ex := pc.ExecLatency()
		tot := pc.TotalLatency()

		m.mu.Lock()
		if h > 0 {
			m.totalHandoff += h
		}
		if qw > 0 {
			m.totalQueueWait += qw
		}
		if ex > 0 {
			m.totalExec += ex
		}
		if tot > 0 {
			m.totalTotal += tot
		}
		m.completedN++
		m.mu.Unlock()
	})

	return m
}

// Snapshot returns a point-in-time copy of the current counters and
// average latencies.  It is safe to call from any goroutine.
func (m *Metrics) Snapshot() *MetricsSnapshot {
	m.mu.Lock()
	n := m.completedN
	var avgHandoff, avgQueueWait, avgExec, avgTotal time.Duration
	if n > 0 {
		avgHandoff = m.totalHandoff / time.Duration(n)
		avgQueueWait = m.totalQueueWait / time.Duration(n)
		avgExec = m.totalExec / time.Duration(n)
		avgTotal = m.totalTotal / time.Duration(n)
	}
	m.mu.Unlock()

	return &MetricsSnapshot{
		Submitted:          m.submitted.Load(),
		Started:            m.started.Load(),
		Completed:          m.completed.Load(),
		Failed:             m.failed.Load(),
		AvgHandoffLatency:  avgHandoff,
		AvgQueueWaitLatency: avgQueueWait,
		AvgExecLatency:     avgExec,
		AvgTotalLatency:    avgTotal,
	}
}
