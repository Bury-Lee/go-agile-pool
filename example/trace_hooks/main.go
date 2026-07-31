// Trace hooks example: demonstrates full lifecycle tracing and metrics.
//
// Features demonstrated:
//   - Context with trace_id and business labels
//   - TimingHook records timestamps at all four lifecycle stages
//   - SlowTaskLogHook detects and logs slow tasks (with caller location)
//   - PrometheusMetrics exposes histograms for Prometheus scraping
//   - Metrics provides in-memory counters and average latencies
//
// Run:
//
//	go run ./example/trace_hooks/
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	agilepool "github.com/Yiming1997/agilePool/v2"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ---------------------------------------------------------------------------
// Custom structured logger
// ---------------------------------------------------------------------------

type traceLogger struct{ *log.Logger }

func (l *traceLogger) Printf(format string, v ...any)  { l.Logger.Printf(format, v...) }
func (l *traceLogger) Println(v ...any)                 { l.Logger.Println(v...) }

func main() {
	// ---- 1. Create pool ----
	pool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithCleanPeriod(500*time.Millisecond),
		agilepool.WithTaskQueueSize(1000),
		agilepool.WithWorkerNumCapacity(500),
	))

	logger := &traceLogger{log.New(os.Stdout, "", log.Lmicroseconds|log.Lmsgprefix)}
	pool.SetLogger(logger)

	// ---- 2. Built-in Metrics (registers TimingHook + counters) ----
	metrics := agilepool.NewMetrics(pool)

	// ---- 3. SlowTaskLogHook — logs a warning when exec > 15ms ----
	//     Includes file:line function via runtime.Caller for diagnostics.
	pool.OnTaskCompleted(agilepool.SlowTaskLogHook(15*time.Millisecond, logger))

	// ---- 4. Prometheus metrics — histograms auto-registered on DefaultRegisterer ----
	promMetrics := agilepool.NewPrometheusMetrics(agilepool.PrometheusMetricsOpts{
		Namespace: "example",
		Subsystem: "agilepool",
	})
	promMetrics.RegisterOn(pool)

	// Expose /metrics endpoint for Prometheus scraping.
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		logger.Printf("Prometheus metrics served at http://localhost:2112/metrics")
		if err := http.ListenAndServe(":2112", nil); err != nil {
			logger.Printf("metrics server: %v", err)
		}
	}()

	// ---- 5. Custom trace logging hooks (stack on top of pre-built hooks) ----
	pool.OnTaskSubmitted(func(ctx context.Context, task agilepool.Task) {
		pc := agilepool.PoolContextFrom(ctx)
		if pc == nil {
			return
		}
		tid, _ := pc.TraceID()
		logger.Printf("[trace=%s] event=submitted  tenant=%s  order_id=%s",
			tid, pc.GetString("tenant"), pc.GetString("order_id"))
	})

	pool.OnTaskEnqueued(func(ctx context.Context, task agilepool.Task) {
		pc := agilepool.PoolContextFrom(ctx)
		if pc == nil {
			return
		}
		tid, _ := pc.TraceID()
		logger.Printf("[trace=%s] event=enqueued", tid)
	})

	pool.OnTaskStarted(func(ctx context.Context, task agilepool.Task) {
		pc := agilepool.PoolContextFrom(ctx)
		if pc == nil {
			return
		}
		tid, _ := pc.TraceID()
		logger.Printf("[trace=%s] event=started", tid)
	})

	pool.OnTaskCompleted(func(ctx context.Context, task agilepool.Task, recovered any) {
		pc := agilepool.PoolContextFrom(ctx)
		if pc == nil {
			return
		}
		tid, _ := pc.TraceID()
		if recovered != nil {
			logger.Printf("[trace=%s] event=panicked  exec=%v  panic=%v",
				tid, pc.ExecLatency(), recovered)
			return
		}
		// Full lifecycle breakdown: handoff / queue-wait / exec / total.
		logger.Printf("[trace=%s] event=completed  handoff=%v  queue_wait=%v  exec=%v  total=%v",
			tid,
			pc.HandoffLatency(),
			pc.QueueWaitLatency(),
			pc.ExecLatency(),
			pc.TotalLatency())
	})

	pool.OnPoolClosed(func(p *agilepool.Pool) {
		logger.Printf("event=pool_closed  running_workers=%d  pending=%d",
			p.GetRunningWorkersNum(), p.GetTaskQueueLen())
	})

	// ---- 6. Submit tasks with Context ----
	fmt.Println("=== Trace Hooks Demo ===")
	fmt.Println("Submitting 20 tasks with auto-generated trace IDs (32-char hex)...")
	fmt.Println("  - Task 7:  artificially slow (~60ms, will trigger SlowTaskLogHook)")
	fmt.Println("  - Task 13: panics intentionally")
	fmt.Println("  - Other:   random 10-50ms sleep")
	fmt.Println()

	var submitWG sync.WaitGroup
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 1; i <= 20; i++ {
		submitWG.Add(1)

		tenantID := fmt.Sprintf("tenant-%c", 'A'+rng.Intn(3))
		orderID := fmt.Sprintf("order-%d", 1000+i)
		taskID := i

		go func() {
			defer submitWG.Done()

			pc := agilepool.NewContext(context.Background())
			// TraceID is auto-generated (16-byte hex) on first access — no SetTraceID needed.
			pc.Store("tenant", tenantID)
			pc.Store("order_id", orderID)
			pc.EnableTiming() // required for timing/metrics hooks

			var t agilepool.Task
			switch {
			case taskID == 7:
				// Artificially slow — triggers SlowTaskLogHook (threshold=15ms).
				t = agilepool.TaskFunc(func() error {
					time.Sleep(60 * time.Millisecond)
					return nil
				})
			case taskID == 13:
				t = agilepool.TaskFunc(func() error {
					time.Sleep(time.Duration(10+rng.Intn(40)) * time.Millisecond)
					panic("simulated panic in task-13")
				})
			default:
				t = agilepool.TaskFunc(func() error {
					time.Sleep(time.Duration(10+rng.Intn(40)) * time.Millisecond)
					return nil
				})
			}

			pool.SubmitCtx(pc, t)
		}()
	}

	submitWG.Wait()
	fmt.Println("All tasks submitted — waiting for completion...")
	time.Sleep(300 * time.Millisecond)

	pool.Close()
	pool.Wait()

	// ---- 7. Print metrics summary (all four lifecycle phases) ----
	snap := metrics.Snapshot()
	if snap != nil {
		fmt.Println()
		fmt.Println("=== Metrics Summary ===")
		fmt.Printf("  submitted:  %d\n", snap.Submitted)
		fmt.Printf("  started:    %d\n", snap.Started)
		fmt.Printf("  completed:  %d\n", snap.Completed)
		fmt.Printf("  failed:     %d\n", snap.Failed)
		fmt.Println("  --- average latencies ---")
		fmt.Printf("  handoff:     %v  (submitted → enqueued)\n", snap.AvgHandoffLatency)
		fmt.Printf("  queue_wait:  %v  (enqueued → started)\n", snap.AvgQueueWaitLatency)
		fmt.Printf("  exec:        %v  (started → completed)\n", snap.AvgExecLatency)
		fmt.Printf("  total:       %v  (submitted → completed)\n", snap.AvgTotalLatency)
	}

	// ---- 8. Prometheus metrics reminder ----
	fmt.Println()
	fmt.Println("=== Prometheus ===")
	fmt.Println("Histograms registered (scrape http://localhost:2112/metrics):")
	fmt.Println("  example_agilepool_task_handoff_latency_seconds")
	fmt.Println("  example_agilepool_task_queue_wait_latency_seconds")
	fmt.Println("  example_agilepool_task_exec_latency_seconds")
	fmt.Println("  example_agilepool_task_total_latency_seconds")

	_ = promMetrics
	fmt.Println()
	fmt.Println("=== Done ===")
}
