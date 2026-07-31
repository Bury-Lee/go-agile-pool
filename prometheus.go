package agilepool

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ---------------------------------------------------------------------------
// PrometheusMetricsOpts — configuration for Prometheus task-latency metrics
// ---------------------------------------------------------------------------

// PrometheusMetricsOpts configures the Prometheus histograms exposed for
// task lifecycle timing.
type PrometheusMetricsOpts struct {
	// Namespace is the Prometheus namespace, typically the application name
	// (e.g. "myapp").
	Namespace string

	// Subsystem is the Prometheus subsystem, typically "agilepool".
	Subsystem string

	// ConstLabels are constant labels applied to every histogram.
	ConstLabels prometheus.Labels

	// Buckets defines the histogram buckets for latency observations.
	// If nil, DefaultLatencyBuckets is used.
	Buckets []float64

	// Registerer is the prometheus.Registerer to use.  If nil,
	// prometheus.DefaultRegisterer is used.
	Registerer prometheus.Registerer
}

// DefaultLatencyBuckets provides a sensible set of histogram buckets suitable
// for most microservice workloads, ranging from 1 ms to 30 s.
var DefaultLatencyBuckets = []float64{
	0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30,
}

// ---------------------------------------------------------------------------
// PrometheusMetrics — full-lifecycle task-latency histograms
// ---------------------------------------------------------------------------

// PrometheusMetrics collects and exposes task lifecycle timing via Prometheus
// histograms.  It observes all four lifecycle phases at task completion:
//
//   - handoff latency   (submitted → enqueued)
//   - queue-wait latency (enqueued → started)
//   - exec latency       (started → completed)
//   - total latency      (submitted → completed)
//
// TimingHook must be registered separately to capture the lifecycle
// timestamps.  EnableTiming() must be called on the Context.
//
// Usage:
//
//	pm := agilepool.NewPrometheusMetrics(agilepool.PrometheusMetricsOpts{
//	    Namespace: "myapp",
//	    Subsystem: "agilepool",
//	})
//	// Register TimingHook to capture timestamps.
//	s, e, st, co := agilepool.TimingHook()
//	pool.OnTaskSubmitted(s)
//	pool.OnTaskEnqueued(e)
//	pool.OnTaskStarted(st)
//	pool.OnTaskCompleted(co)
//	// Register Prometheus observation hooks.
//	pm.RegisterOn(pool)
//
//	// In the task, enable timing on the Context:
//	pc := agilepool.PoolContextFrom(ctx)
//	pc.EnableTiming()
type PrometheusMetrics struct {
	handoffLatency   prometheus.Histogram
	queueWaitLatency prometheus.Histogram
	execLatency      prometheus.Histogram
	totalLatency     prometheus.Histogram
}

// NewPrometheusMetrics creates and registers Prometheus histograms for
// all four task lifecycle phases.  If opts.Buckets is nil,
// DefaultLatencyBuckets is used.
func NewPrometheusMetrics(opts PrometheusMetricsOpts) *PrometheusMetrics {
	buckets := opts.Buckets
	if buckets == nil {
		buckets = DefaultLatencyBuckets
	}
	reg := opts.Registerer
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	factory := promauto.With(reg)

	return &PrometheusMetrics{
		handoffLatency: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			Name:        "task_handoff_latency_seconds",
			Help:        "Time from submission to enqueueing (submitted → enqueued).",
			Buckets:     buckets,
			ConstLabels: opts.ConstLabels,
		}),
		queueWaitLatency: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			Name:        "task_queue_wait_latency_seconds",
			Help:        "Time spent waiting in the queue (enqueued → started).",
			Buckets:     buckets,
			ConstLabels: opts.ConstLabels,
		}),
		execLatency: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			Name:        "task_exec_latency_seconds",
			Help:        "Time spent executing (started → completed).",
			Buckets:     buckets,
			ConstLabels: opts.ConstLabels,
		}),
		totalLatency: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			Name:        "task_total_latency_seconds",
			Help:        "End-to-end latency (submitted → completed).",
			Buckets:     buckets,
			ConstLabels: opts.ConstLabels,
		}),
	}
}

// Hooks returns the four lifecycle hooks that record observations into the
// Prometheus histograms at task completion.
//
//	submitted, enqueued, started, completed := pm.Hooks()
//	pool.OnTaskSubmitted(submitted)
//	pool.OnTaskEnqueued(enqueued)
//	pool.OnTaskStarted(started)
//	pool.OnTaskCompleted(completed)
func (pm *PrometheusMetrics) Hooks() (TaskHook, TaskHook, TaskHook, TaskCompleteHook) {
	noop := func(ctx context.Context, task Task) {}

	completed := func(ctx context.Context, task Task, recovered any) {
		pc := PoolContextFrom(ctx)
		if pc == nil || !pc.IsTimingEnabled() {
			return
		}
		if h := pc.HandoffLatency(); h > 0 {
			pm.handoffLatency.Observe(h.Seconds())
		}
		if qw := pc.QueueWaitLatency(); qw > 0 {
			pm.queueWaitLatency.Observe(qw.Seconds())
		}
		if el := pc.ExecLatency(); el > 0 {
			pm.execLatency.Observe(el.Seconds())
		}
		if tl := pc.TotalLatency(); tl > 0 {
			pm.totalLatency.Observe(tl.Seconds())
		}
	}

	return noop, noop, noop, completed
}

// RegisterOn is a convenience method that registers all four Prometheus
// observation hooks on p in a single call.  Equivalent to calling Hooks()
// and registering each hook manually.
func (pm *PrometheusMetrics) RegisterOn(p *Pool) {
	s, e, st, co := pm.Hooks()
	p.OnTaskSubmitted(s)
	p.OnTaskEnqueued(e)
	p.OnTaskStarted(st)
	p.OnTaskCompleted(co)
}

// ---------------------------------------------------------------------------
// SetupTiming — one-shot wiring of TimingHook + PrometheusMetrics
// ---------------------------------------------------------------------------

// SetupTiming registers both TimingHook (to capture timestamps) and
// PrometheusMetrics (to observe them) on p in a single call.  It returns
// the PrometheusMetrics instance so the caller can inspect or unregister
// the metrics later.
//
// Usage:
//
//	pm := agilepool.SetupTiming(pool, agilepool.PrometheusMetricsOpts{
//	    Namespace: "myapp",
//	    Subsystem: "agilepool",
//	})
func SetupTiming(p *Pool, opts PrometheusMetricsOpts) *PrometheusMetrics {
	s, e, st, co := TimingHook()
	p.OnTaskSubmitted(s)
	p.OnTaskEnqueued(e)
	p.OnTaskStarted(st)
	p.OnTaskCompleted(co)

	pm := NewPrometheusMetrics(opts)
	pm.RegisterOn(p)
	return pm
}
