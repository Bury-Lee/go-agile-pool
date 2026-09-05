package agilepool

import (
	"context"
)

// TaskHook is the callback signature for task lifecycle events.
type TaskHook func(ctx context.Context, task Task)

// TaskCompleteHook is the callback signature for task completion.
// recovered is nil on normal exit, otherwise the value passed to panic.
type TaskCompleteHook func(ctx context.Context, task Task, recovered any)

// PoolHook is the callback signature for pool-level events.
type PoolHook func(p *Pool)

type hooks interface {
	// DispatchTaskSubmitted must only be called by the pool submission path.
	DispatchTaskSubmitted(context.Context, Task)
	// DispatchTaskEnqueued must only be called by the pool enqueue path.
	DispatchTaskEnqueued(context.Context, Task)

	// DispatchTaskStarted must only be called by the worker execution path.
	DispatchTaskStarted(context.Context, Task)

	// DispatchTaskCompleted must only be called by the worker completion path.
	DispatchTaskCompleted(context.Context, Task, any)
	// DispatchPoolClosed must only be called by Pool.Close.
	DispatchPoolClosed(*Pool)
}
