package hook

import (
	"context"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"testing"

	agilepool "github.com/Yiming1997/agilePool/v2"
)

func newTestHooks() *Hooks {
	h := NewHooks()
	h.logger = *log.New(io.Discard, "", 0)
	return h
}

func TestNewHooksDispatchesAllEvents(t *testing.T) {
	h := newTestHooks()

	var submitted, enqueued, started, completed, closed atomic.Int64
	h.AddTaskSubmitted(func(context.Context) { submitted.Add(1) })
	h.AddTaskEnqueued(func(context.Context) { enqueued.Add(1) })
	h.AddTaskStarted(func(context.Context) { started.Add(1) })
	h.AddTaskCompleted(func(context.Context, any) { completed.Add(1) })
	h.AddPoolClosed(func(*agilepool.Pool) { closed.Add(1) })

	pool := agilepool.NewPool(agilepool.NewConfig())
	defer pool.Close()

	ctx := context.Background()
	h.DispatchTaskSubmitted(ctx)
	h.DispatchTaskEnqueued(ctx)
	h.DispatchTaskStarted(ctx)
	h.DispatchTaskCompleted(ctx, nil)
	h.DispatchPoolClosed(pool)

	if got := submitted.Load(); got != 1 {
		t.Fatalf("submitted callbacks = %d, want 1", got)
	}
	if got := enqueued.Load(); got != 1 {
		t.Fatalf("enqueued callbacks = %d, want 1", got)
	}
	if got := started.Load(); got != 1 {
		t.Fatalf("started callbacks = %d, want 1", got)
	}
	if got := completed.Load(); got != 1 {
		t.Fatalf("completed callbacks = %d, want 1", got)
	}
	if got := closed.Load(); got != 1 {
		t.Fatalf("closed callbacks = %d, want 1", got)
	}
}

func TestCallbackPanicsAreIsolatedPerEvent(t *testing.T) {
	h := newTestHooks()

	var calls atomic.Int64
	boom := func() { panic("hook panic") }

	h.AddTaskSubmitted(func(context.Context) { boom() })
	h.AddTaskSubmitted(func(context.Context) { calls.Add(1) })
	h.AddTaskEnqueued(func(context.Context) { boom() })
	h.AddTaskEnqueued(func(context.Context) { calls.Add(1) })
	h.AddTaskStarted(func(context.Context) { boom() })
	h.AddTaskStarted(func(context.Context) { calls.Add(1) })
	h.AddTaskCompleted(func(context.Context, any) { boom() })
	h.AddTaskCompleted(func(context.Context, any) { calls.Add(1) })
	h.AddPoolClosed(func(*agilepool.Pool) { boom() })
	h.AddPoolClosed(func(*agilepool.Pool) { calls.Add(1) })

	pool := agilepool.NewPool(agilepool.NewConfig())
	defer pool.Close()

	ctx := context.Background()
	h.DispatchTaskSubmitted(ctx)
	h.DispatchTaskEnqueued(ctx)
	h.DispatchTaskStarted(ctx)
	h.DispatchTaskCompleted(ctx, nil)
	h.DispatchPoolClosed(pool)

	if got := calls.Load(); got != 5 {
		t.Fatalf("surviving callbacks = %d, want 5", got)
	}
}

func TestDispatchPoolClosedNilPoolIsIgnored(t *testing.T) {
	h := newTestHooks()

	var calls atomic.Int64
	h.AddPoolClosed(func(*agilepool.Pool) { calls.Add(1) })

	h.DispatchPoolClosed(nil)

	if got := calls.Load(); got != 0 {
		t.Fatalf("closed callbacks with nil pool = %d, want 0", got)
	}
}

type ctxKey struct{}

func TestHooksReceiveContextThroughPool(t *testing.T) {
	h := newTestHooks()

	var submittedValue, enqueuedValue, startedValue, completedValue atomic.Value
	h.AddTaskSubmitted(func(ctx context.Context) { submittedValue.Store(ctx.Value(ctxKey{})) })
	h.AddTaskEnqueued(func(ctx context.Context) { enqueuedValue.Store(ctx.Value(ctxKey{})) })
	h.AddTaskStarted(func(ctx context.Context) { startedValue.Store(ctx.Value(ctxKey{})) })
	h.AddTaskCompleted(func(ctx context.Context, _ any) { completedValue.Store(ctx.Value(ctxKey{})) })

	pool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(2),
	))
	pool.SetLogger(log.New(io.Discard, "", 0))
	if err := pool.SetHook(h); err != nil {
		t.Fatalf("SetHook() error = %v", err)
	}
	defer pool.Close()

	ctx := context.WithValue(context.Background(), ctxKey{}, "trace-value")
	pool.SubmitCtx(ctx, agilepool.TaskFunc(func() error { return nil }))
	pool.Wait()

	check := func(name string, got any) {
		t.Helper()
		if got != "trace-value" {
			t.Fatalf("%s hook context value = %v, want trace-value", name, got)
		}
	}
	check("submitted", submittedValue.Load())
	check("enqueued", enqueuedValue.Load())
	check("started", startedValue.Load())
	check("completed", completedValue.Load())
}

func TestHooksIntegrationWithPoolAndPanickingTask(t *testing.T) {
	h := newTestHooks()

	var submitted, enqueued, started, completed, closed atomic.Int64
	var mu sync.Mutex
	var recovered []any
	h.AddTaskSubmitted(func(context.Context) { submitted.Add(1) })
	h.AddTaskEnqueued(func(context.Context) { enqueued.Add(1) })
	h.AddTaskStarted(func(context.Context) { started.Add(1) })
	h.AddTaskCompleted(func(_ context.Context, r any) {
		completed.Add(1)
		mu.Lock()
		recovered = append(recovered, r)
		mu.Unlock()
	})
	h.AddPoolClosed(func(*agilepool.Pool) { closed.Add(1) })

	pool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(2),
	))
	pool.SetLogger(log.New(io.Discard, "", 0))
	if err := pool.SetHook(h); err != nil {
		t.Fatalf("SetHook() error = %v", err)
	}

	pool.Submit(agilepool.TaskFunc(func() error { return nil }))
	pool.Submit(agilepool.TaskFunc(func() error { panic("task boom") }))
	pool.Wait()
	pool.Close()

	if got := submitted.Load(); got != 2 {
		t.Fatalf("submitted callbacks = %d, want 2", got)
	}
	if got := enqueued.Load(); got != 2 {
		t.Fatalf("enqueued callbacks = %d, want 2", got)
	}
	if got := started.Load(); got != 2 {
		t.Fatalf("started callbacks = %d, want 2", got)
	}
	if got := completed.Load(); got != 2 {
		t.Fatalf("completed callbacks = %d, want 2", got)
	}
	if got := closed.Load(); got != 1 {
		t.Fatalf("closed callbacks = %d, want 1", got)
	}

	mu.Lock()
	defer mu.Unlock()
	var sawNil, sawPanic bool
	for _, r := range recovered {
		if r == nil {
			sawNil = true
		}
		if r == "task boom" {
			sawPanic = true
		}
	}
	if !sawNil || !sawPanic {
		t.Fatalf("recovered values = %v, want both nil and \"task boom\"", recovered)
	}
}
