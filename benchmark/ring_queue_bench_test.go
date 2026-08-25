// BenchmarkRingQueueRemoveExpired measures the allocation cost of the
// idle-worker expiry sweep (RemoveExpired) through the public Pool API
// when the ring-queue idle container is used.
//
// Each iteration parks a batch of idle workers, then waits for the
// background cleaner to sweep the workers idle longer than its hardcoded
// 1s expiry. The ring-queue sweep compacts survivors in place, so the
// timed region allocates nothing (0 B/op, 0 allocs/op).
package benchmark_test

import (
	"testing"
	"time"

	agilepool "github.com/Yiming1997/agilePool/v2"
)

func BenchmarkRingQueueRemoveExpired(b *testing.B) {
	const (
		workers = 1000
		work    = 2 * time.Millisecond // per task, so the scaler ramps up the worker count
	)

	pool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithIdleContainerType(agilepool.RingQueueType),
		agilepool.WithWorkerNumCapacity(workers),
		agilepool.WithTaskQueueSize(workers),
		agilepool.WithCleanPeriod(10*time.Millisecond),
	))
	defer pool.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		for j := 0; j < workers; j++ {
			pool.Submit(agilepool.TaskFunc(func() error { time.Sleep(work); return nil }))
		}
		pool.Wait()
		// Wait until every worker has parked in the idle container so the
		// next sweep removes a full batch.
		for pool.GetRunningWorkersNum() > 0 {
			time.Sleep(time.Millisecond)
		}
		b.StartTimer()

		// Wait for the cleaner to sweep the workers idle for > 1s.
		time.Sleep(1050 * time.Millisecond)
	}
}
