package idle

import "time"

// testWorker is a minimal Dated element used across idle container tests.
type testWorker struct {
	lastActiveAt time.Time
}

func (w *testWorker) DatedTime() time.Time { return w.lastActiveAt }

func newTestWorker(lastActiveAt time.Time) *testWorker {
	return &testWorker{lastActiveAt: lastActiveAt}
}
