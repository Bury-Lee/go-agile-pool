package agilepool

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSubmitCtxCancelsAtBackpressureLimit(t *testing.T) {
	p := NewPool(NewConfig(
		WithWorkerNumCapacity(1),
		WithTaskQueueSize(1),
	))

	started := make(chan struct{})
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
		p.Close()
	}()

	p.Submit(TaskFunc(func() error {
		close(started)
		<-release
		return nil
	}))
	<-started

	// Fill the handoff channel, then construct the exact full overflow state
	// without making this regression test spend time submitting 100,000 tasks.
	p.Submit(TaskFunc(func() error { return nil }))
	p.wg.Add(maxChunkLen)
	atomic.AddInt64(&p.pendingTasks, maxChunkLen)
	p.taskBuf.taskMu.Lock()
	noop := TaskFunc(func() error { return nil })
	for i := 0; i < maxChunkLen; i++ {
		p.taskBuf.pushTail(noop)
	}
	p.taskBuf.taskMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	returned := make(chan struct{})
	go func() {
		p.SubmitCtx(ctx, TaskFunc(func() error { return nil }))
		close(returned)
	}()

	deadline := time.Now().Add(time.Second)
	for atomic.LoadInt64(&p.pendingTasks) < maxChunkLen+3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := atomic.LoadInt64(&p.pendingTasks); got < maxChunkLen+3 {
		t.Fatalf("submission did not reach the full-buffer branch: pending=%d", got)
	}

	cancel()
	select {
	case <-returned:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("SubmitCtx did not return after cancellation at the backpressure limit")
	}

	close(release)
	p.Wait()
}
