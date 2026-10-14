package benchmark_test

import (
	"sync"
	"testing"
	"time"

	agilepool "github.com/Yiming1997/agilePool/v2"
	"github.com/panjf2000/ants/v2"
)

// cmpWorkerCap mirrors the worker capacity used by the agilePool benchmarks so
// ants and agilePool run under the same concurrency budget.
const cmpWorkerCap = 20000

func newAgilePoolForCmp() *agilepool.Pool {
	return agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithCleanPeriod(500*time.Millisecond),
		agilepool.WithTaskQueueSize(100000),
		agilepool.WithWorkerNumCapacity(cmpWorkerCap),
		agilepool.WithIdleContainerType(agilepool.LinkedListType),
	))
}

// ---------------------------------------------------------------------------
// Metric 1 — hand-off throughput: no-op tasks.
//
// The task body does no work, so the measured cost is entirely the pool's
// task hand-off (submit -> worker pickup). ns/op therefore reads directly as
// the per-task hand-off cost; lower is better.
// ---------------------------------------------------------------------------

func BenchmarkCmpChurnAnts(b *testing.B) {
	pool, err := ants.NewPool(cmpWorkerCap)
	if err != nil {
		b.Fatal(err)
	}
	defer pool.Release()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = pool.Submit(func() {})
		}
	})
}

func BenchmarkCmpChurnAgilePool(b *testing.B) {
	pool := newAgilePoolForCmp()
	defer pool.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pool.Submit(agilepool.TaskFunc(func() error { return nil }))
		}
	})
	pool.Wait()
}

// ---------------------------------------------------------------------------
// Metric 2 — CPU usage under an execution-bound load: 10ms sleep tasks (the
// same shape as the agilePool SequentialLinkedList benchmark). Here the task
// duration is the limiter, so the meaningful metric is CPU burned per
// completed task, read from the CPU profile of each benchmark.
// ---------------------------------------------------------------------------

func BenchmarkCmpSleepAnts(b *testing.B) {
	pool, err := ants.NewPool(cmpWorkerCap)
	if err != nil {
		b.Fatal(err)
	}
	defer pool.Release()

	var wg sync.WaitGroup
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			wg.Add(1)
			_ = pool.Submit(func() {
				defer wg.Done()
				time.Sleep(10 * time.Millisecond)
			})
		}
	})
	wg.Wait()
}

func BenchmarkCmpSleepAgilePool(b *testing.B) {
	pool := newAgilePoolForCmp()
	defer pool.Close()

	// Use the same WaitGroup-based completion tracking as the ants variant so
	// neither side gets a harness-specific advantage (both closures capture
	// &wg and therefore heap-allocate identically).
	var wg sync.WaitGroup
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			wg.Add(1)
			pool.Submit(agilepool.TaskFunc(func() error {
				defer wg.Done()
				time.Sleep(10 * time.Millisecond)
				return nil
			}))
		}
	})
	wg.Wait()
}
