package agilepool

import (
	"context"
	"sync"
)

// TaskHook is the callback signature for task lifecycle events.
type TaskHook func(ctx context.Context, task Task)

// TaskCompleteHook is the callback signature for task completion.
// recovered is nil on normal exit, otherwise the value passed to panic.
type TaskCompleteHook func(ctx context.Context, task Task, recovered any)

// PoolHook is the callback signature for pool-level events.
type PoolHook func(p *Pool)

// hooks stores lifecycle callbacks registered on a Pool.
// Each slice is append-only and protected by mu.
type hooks struct {
	mu sync.Mutex

	taskSubmitted []TaskHook
	taskEnqueued  []TaskHook
	taskStarted   []TaskHook
	taskCompleted []TaskCompleteHook
	poolClosed    []PoolHook
}

func newHooks() *hooks {
	return &hooks{}
}

// OnTaskSubmitted registers fn to be called when a task is accepted by the pool.
//
// NOTE: must be registered before the pool starts; adding hooks mid-execution
// is not supported and may lead to undefined behavior.
func (p *Pool) OnTaskSubmitted(fn TaskHook) {
	p.hooks.mu.Lock()
	p.hooks.taskSubmitted = append(p.hooks.taskSubmitted, fn)
	p.hooks.mu.Unlock()
}

// OnTaskEnqueued registers fn to be called when a task enters the handoff channel.
//
// NOTE: must be registered before the pool starts; adding hooks mid-execution
// is not supported and may lead to undefined behavior.
func (p *Pool) OnTaskEnqueued(fn TaskHook) {
	p.hooks.mu.Lock()
	p.hooks.taskEnqueued = append(p.hooks.taskEnqueued, fn)
	p.hooks.mu.Unlock()
}

// OnTaskStarted registers fn to be called right before task.Process().
//
// NOTE: must be registered before the pool starts; adding hooks mid-execution
// is not supported and may lead to undefined behavior.
func (p *Pool) OnTaskStarted(fn TaskHook) {
	p.hooks.mu.Lock()
	p.hooks.taskStarted = append(p.hooks.taskStarted, fn)
	p.hooks.mu.Unlock()
}

// OnTaskCompleted registers fn to be called after task.Process() returns.
// recovered is nil on normal completion, otherwise the panic value.
//
// NOTE: must be registered before the pool starts; adding hooks mid-execution
// is not supported and may lead to undefined behavior.
func (p *Pool) OnTaskCompleted(fn TaskCompleteHook) {
	p.hooks.mu.Lock()
	p.hooks.taskCompleted = append(p.hooks.taskCompleted, fn)
	p.hooks.mu.Unlock()
}

// OnPoolClosed registers fn to be called when Close() is invoked.
// Fires once; subsequent Close calls are no-ops and do not retrigger.
//
// NOTE: must be registered before the pool starts; adding hooks mid-execution
// is not supported and may lead to undefined behavior.
func (p *Pool) OnPoolClosed(fn PoolHook) {
	p.hooks.mu.Lock()
	p.hooks.poolClosed = append(p.hooks.poolClosed, fn)
	p.hooks.mu.Unlock()
}

func (p *Pool) dispatchTaskSubmitted(ctx context.Context, task Task) {
	for _, fn := range p.hooks.taskSubmitted {
		p.invokeTaskHookSafely(fn, ctx, task, "OnTaskSubmitted")
	}
}

func (p *Pool) dispatchTaskEnqueued(ctx context.Context, task Task) {
	for _, fn := range p.hooks.taskEnqueued {
		p.invokeTaskHookSafely(fn, ctx, task, "OnTaskEnqueued")
	}
}

func (p *Pool) dispatchTaskStarted(ctx context.Context, task Task) {
	for _, fn := range p.hooks.taskStarted {
		p.invokeTaskHookSafely(fn, ctx, task, "OnTaskStarted")
	}
}

func (p *Pool) dispatchTaskCompleted(ctx context.Context, task Task, recovered any) {
	for _, fn := range p.hooks.taskCompleted {
		func() {
			defer func() {
				if r := recover(); r != nil {
					p.logger.Printf("hook OnTaskCompleted panicked: %v\n%s", r, Stack(1))
				}
			}()
			fn(ctx, task, recovered)
		}()
	}
}

func (p *Pool) dispatchPoolClosed() {
	for _, fn := range p.hooks.poolClosed {
		func() {
			defer func() {
				if r := recover(); r != nil {
					p.logger.Printf("hook OnPoolClosed panicked: %v\n%s", r, Stack(1))
				}
			}()
			fn(p)
		}()
	}
}

func (p *Pool) invokeTaskHookSafely(fn TaskHook, ctx context.Context, task Task, hookName string) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Printf("hook %s panicked: %v\n%s", hookName, r, Stack(1))
		}
	}()
	fn(ctx, task)
}
