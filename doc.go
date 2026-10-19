// Package agilepool provides a high-performance goroutine pool with adaptive
// worker scaling, bounded concurrency, idle-worker reuse and an overflow task
// buffer.
//
// Create a pool with NewPool and submit work through Submit, TrySubmit,
// SubmitCtx or SubmitBefore:
//
//	p := agilepool.NewPool(agilepool.NewConfig(
//		agilepool.WithWorkerNumCapacity(64),
//	))
//	defer p.Close()
//	p.Submit(agilepool.NewTask(func() error { return nil }))
//	p.Wait()
//
// Tasks can be wrapped with NewTask, TaskFunc, TaskWithRetry or UpdateTask,
// and lifecycle callbacks can be registered through Pool.SetHook. See the
// README for a complete feature overview.
package agilepool
