package agilepool

import (
	"errors"
	"testing"
	"time"
)

// TestTaskWithRetry_BackOffStrategyReceivesCurrentRetryNum 验证自定义 BackOffStrategy
// 收到的 retryNum 是当前第几次重试（1,2,3），而不是总重试次数（3,3,3）。
func TestTaskWithRetry_BackOffStrategyReceivesCurrentRetryNum(t *testing.T) {
	var received []uint
	task := &TaskWithRetry{
		MinBackOff: 1 * time.Millisecond,
		MaxBackOff: 5 * time.Millisecond,
		RetryNum:   3,
		BackOffStrategy: func(min, max time.Duration, retryNum uint) time.Duration {
			received = append(received, retryNum)
			return 1 * time.Millisecond
		},
		Task: func() error {
			return errors.New("always fail")
		},
	}

	task.Process()

	if len(received) != 3 {
		t.Fatalf("got %d backoff calls, want 3", len(received))
	}

	want := []uint{1, 2, 3}
	for i, v := range want {
		if received[i] != v {
			t.Errorf("backoff call %d: got retryNum=%d, want %d", i+1, received[i], v)
		}
	}
}