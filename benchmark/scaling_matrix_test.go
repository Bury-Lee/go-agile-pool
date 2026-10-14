package benchmark_test

import (
	"fmt"
	"testing"
	"time"

	agilepool "github.com/Yiming1997/agilePool/v2"
)

// Implementation-agnostic scaling matrix. It deliberately avoids any
// version-specific config so the exact same file compiles and runs against
// both the pre-sharding implementation and the current one, letting the two
// be compared by swapping the checkout only.
var scalingCaps = []int{128, 1024, 8192, 20000}

func newScalingPool(capacity int) *agilepool.Pool {
	return agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithCleanPeriod(500*time.Millisecond),
		agilepool.WithTaskQueueSize(100000),
		agilepool.WithWorkerNumCapacity(int64(capacity)),
		agilepool.WithIdleContainerType(agilepool.LinkedListType),
	))
}

// BenchmarkScalingNoop is hand-off bound: no-op tasks, so throughput is the
// pool's submit -> worker pickup rate.
func BenchmarkScalingNoop(b *testing.B) {
	for _, capacity := range scalingCaps {
		b.Run(fmt.Sprintf("cap=%d", capacity), func(b *testing.B) {
			pool := newScalingPool(capacity)
			defer pool.Close()
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					pool.Submit(agilepool.TaskFunc(func() error { return nil }))
				}
			})
		})
	}
}

// BenchmarkScalingSleep10ms is execution bound: each task sleeps 10ms, so the
// limiter is capacity/10ms and the channel hand-off overhead shows up only as
// extra CPU and scheduling jitter, not throughput.
func BenchmarkScalingSleep10ms(b *testing.B) {
	for _, capacity := range scalingCaps {
		b.Run(fmt.Sprintf("cap=%d", capacity), func(b *testing.B) {
			pool := newScalingPool(capacity)
			defer pool.Close()
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					pool.Submit(agilepool.TaskFunc(func() error {
						time.Sleep(10 * time.Millisecond)
						return nil
					}))
				}
			})
			pool.Wait()
		})
	}
}
