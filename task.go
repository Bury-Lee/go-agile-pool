package agilepool

import (
	"math"
	"time"
)

// Task is a unit of work executed by a worker. Use NewTask, TaskFunc,
// TaskWithRetry or UpdateTask to build one.
type Task interface {
	process()
}

// NewTask wraps a function as a Task so it can be submitted directly.
func NewTask(fn func() error) Task {
	return TaskFunc(fn)
}

// TaskFunc adapts a plain function to Task.
type TaskFunc func() error

func (tf TaskFunc) process() {
	tf()
}

// TaskWithRetry runs Task up to RetryNum additional times with exponential
// backoff whenever it returns an error. BackOffStrategy replaces the default
// backoff schedule when set.
type TaskWithRetry struct {
	MinBackOff      time.Duration
	MaxBackOff      time.Duration
	RetryNum        uint
	BackOffStrategy func(min, max time.Duration, retryNum uint) time.Duration
	Task            func() error
}

func (t *TaskWithRetry) process() {
	if t.Task() != nil {
		t.runBackOffStrategy()
	}
}

func (t *TaskWithRetry) runBackOffStrategy() {
	for i := 1; i <= int(t.RetryNum); i++ {
		backOffTime := t.getBackOffTime(uint(i))
		time.Sleep(backOffTime)
		if t.Task() != nil {
			continue
		}
		break
	}
}

func (t *TaskWithRetry) getBackOffTime(retryNum uint) time.Duration {
	if t.BackOffStrategy != nil {
		return t.BackOffStrategy(t.MinBackOff, t.MaxBackOff, retryNum)
	}

	return defaultBackOffStrategy(t.MinBackOff, t.MaxBackOff, retryNum)
}

func defaultBackOffStrategy(min, max time.Duration, retryNum uint) time.Duration {
	mult := math.Pow(2, float64(retryNum)) * float64(min)
	sleep := time.Duration(mult)
	if float64(sleep) != mult || sleep > max {
		sleep = max
	}
	return sleep
}
