package agilepool

import (
	"testing"
	"time"
)

// BenchmarkRingQueue_RemoveExpired measures the allocation count of
// RemoveExpired with a mix of expired and alive workers.
func BenchmarkRingQueue_RemoveExpired(b *testing.B) {
	now := time.Now()
	expiry := 5 * time.Second

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		rq := newRingQueue()
		for j := 0; j < 1000; j++ {
			if j%2 == 0 {
				rq.Add(NewWorker(now.Add(-10 * time.Second))) // expired
			} else {
				rq.Add(NewWorker(now.Add(-1 * time.Second))) // alive
			}
		}
		b.StartTimer()

		rq.RemoveExpired(now, expiry)
	}
}
