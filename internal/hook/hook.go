package hook

import (
	"context"
	"log"
	"runtime/debug"
	"sync"

	agilepool "github.com/Yiming1997/agilePool/v2"
)

type TaskHook func(ctx context.Context, task agilepool.Task)
type TaskCompleteHook func(ctx context.Context, task agilepool.Task, recovered any)
type PoolHook func(pool *agilepool.Pool)

// Hooks stores and dispatches lifecycle callbacks without depending on the
// public pool package.
type Hooks struct {
	mu            sync.RWMutex
	taskSubmitted []TaskHook
	taskEnqueued  []TaskHook
	taskStarted   []TaskHook
	taskCompleted []TaskCompleteHook
	poolClosed    []PoolHook
	logger        log.Logger
}

func NewHooks() *Hooks {
	return &Hooks{}
}

func (h *Hooks) AddTaskSubmitted(fn TaskHook) {
	h.mu.Lock()
	h.taskSubmitted = append(h.taskSubmitted, fn)
	h.mu.Unlock()
}

func (h *Hooks) AddTaskEnqueued(fn TaskHook) {
	h.mu.Lock()
	h.taskEnqueued = append(h.taskEnqueued, fn)
	h.mu.Unlock()
}

func (h *Hooks) AddTaskStarted(fn TaskHook) {
	h.mu.Lock()
	h.taskStarted = append(h.taskStarted, fn)
	h.mu.Unlock()
}

func (h *Hooks) AddTaskCompleted(fn TaskCompleteHook) {
	h.mu.Lock()
	h.taskCompleted = append(h.taskCompleted, fn)
	h.mu.Unlock()
}

func (h *Hooks) AddPoolClosed(fn PoolHook) {
	h.mu.Lock()
	h.poolClosed = append(h.poolClosed, fn)
	h.mu.Unlock()
}

// DispatchTaskSubmitted dispatches submission callbacks. It must only be
// called by the pool submission path.
func (h *Hooks) DispatchTaskSubmitted(ctx context.Context, task agilepool.Task) {
	for _, fn := range h.taskSubmitted {
		h.invoke(func() { fn(ctx, task) }, "OnTaskSubmitted")
	}
}

// DispatchTaskEnqueued dispatches enqueue callbacks. It must only be called
// by the pool enqueue path.
func (h *Hooks) DispatchTaskEnqueued(ctx context.Context, task agilepool.Task) {
	for _, fn := range h.taskEnqueued {
		h.invoke(func() { fn(ctx, task) }, "OnTaskEnqueued")
	}
}

// DispatchTaskStarted dispatches start callbacks. It must only be called by
// the worker execution path.
func (h *Hooks) DispatchTaskStarted(ctx context.Context, task agilepool.Task) {
	for _, fn := range h.taskStarted {
		h.invoke(func() { fn(ctx, task) }, "OnTaskStarted")
	}
}

// DispatchTaskCompleted dispatches completion callbacks. It must only be
// called by the worker completion path.
func (h *Hooks) DispatchTaskCompleted(ctx context.Context, task agilepool.Task, recovered any) {
	for _, fn := range h.taskCompleted {
		h.invoke(func() { fn(ctx, task, recovered) }, "OnTaskCompleted")
	}
}

// DispatchPoolClosed dispatches pool-close callbacks. It must only be called
// by Pool.Close.
func (h *Hooks) DispatchPoolClosed(pool *agilepool.Pool) {
	if pool == nil {
		h.logger.Println("[ERROR] DispatchPoolClosed: received nil pointer")
		return
	}
	for _, fn := range h.poolClosed {
		h.invoke(func() { fn(pool) }, "OnPoolClosed")
	}
}

func (h *Hooks) invoke(fn func(), name string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			h.logger.Printf("hook %s panicked: %v\n%s \n", name, recovered, debug.Stack())
		}
	}()
	fn()
}
