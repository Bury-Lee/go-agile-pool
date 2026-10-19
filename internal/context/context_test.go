package context

import (
	stdctx "context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewContextWithNilParent(t *testing.T) {
	c := NewContext(nil)
	if c == nil {
		t.Fatal("NewContext(nil) returned nil")
	}
	if c.Context == nil {
		t.Fatal("NewContext(nil) did not install a parent context")
	}
	if v, ok := c.Get("missing"); ok || v != nil {
		t.Fatalf("Get(missing) = (%v, %v), want (nil, false)", v, ok)
	}
	c.Release()
}

func TestStoreGetAndTypedHelpers(t *testing.T) {
	c := NewContext(stdctx.Background())
	defer c.Release()

	c.Store("name", "agile")
	if v, ok := c.Get("name"); !ok || v != "agile" {
		t.Fatalf("Get(name) = (%v, %v), want (agile, true)", v, ok)
	}
	if got := c.GetString("name"); got != "agile" {
		t.Fatalf("GetString(name) = %q, want %q", got, "agile")
	}

	// Missing key and wrong value type both yield the zero value.
	if got := c.GetString("missing"); got != "" {
		t.Fatalf("GetString(missing) = %q, want empty", got)
	}
	c.Store("number", 42)
	if got := c.GetString("number"); got != "" {
		t.Fatalf("GetString(number) = %q, want empty", got)
	}

	now := time.Now()
	c.Store("when", now)
	if got := c.GetTime("when"); !got.Equal(now) {
		t.Fatalf("GetTime(when) = %v, want %v", got, now)
	}
	if got := c.GetTime("name"); !got.IsZero() {
		t.Fatalf("GetTime(name) = %v, want zero", got)
	}
	if got := c.GetTime("missing"); !got.IsZero() {
		t.Fatalf("GetTime(missing) = %v, want zero", got)
	}

	// Overwriting a label replaces the previous value.
	c.Store("name", "pool")
	if got := c.GetString("name"); got != "pool" {
		t.Fatalf("GetString(name) after overwrite = %q, want %q", got, "pool")
	}
}

type parentKey struct{}

func TestValueSearchesLabelsThenParent(t *testing.T) {
	parent := stdctx.WithValue(stdctx.Background(), parentKey{}, "parent-value")
	c := NewContext(parent)
	defer c.Release()

	c.Store("label", "label-value")

	if got := c.Value("label"); got != "label-value" {
		t.Fatalf("Value(label) = %v, want label-value", got)
	}
	if got := c.Value(parentKey{}); got != "parent-value" {
		t.Fatalf("Value(parentKey) = %v, want parent-value", got)
	}
	if got := c.Value("missing"); got != nil {
		t.Fatalf("Value(missing) = %v, want nil", got)
	}
}

func TestPoolContextFrom(t *testing.T) {
	c := NewContext(stdctx.Background())
	defer c.Release()

	if got := PoolContextFrom(c); got != c {
		t.Fatalf("PoolContextFrom(*Context) = %p, want %p", got, c)
	}

	wrapped := stdctx.WithValue(stdctx.Background(), "outer", "v")
	wrapped = stdctx.WithValue(wrapped, poolContextKey, c)
	if got := PoolContextFrom(wrapped); got != c {
		t.Fatalf("PoolContextFrom(wrapped) = %p, want %p", got, c)
	}

	if got := PoolContextFrom(stdctx.Background()); got != nil {
		t.Fatalf("PoolContextFrom(Background) = %p, want nil", got)
	}
}

func TestResetClearsLabelsAndKeepsParent(t *testing.T) {
	parent := stdctx.WithValue(stdctx.Background(), "p", "v")
	c := NewContext(parent)
	c.Store("a", 1)
	c.Store("b", 2)

	c.Reset()

	if _, ok := c.Get("a"); ok {
		t.Fatal("label a survived Reset")
	}
	if _, ok := c.Get("b"); ok {
		t.Fatal("label b survived Reset")
	}
	if got := c.Value("p"); got != "v" {
		t.Fatalf("parent value after Reset = %v, want v", got)
	}
	c.Release()
}

func TestReleaseClearsLabelsAndDropsParent(t *testing.T) {
	c := NewContext(stdctx.Background())
	c.Store(LabelTraceID, "trace")
	c.Release()

	if _, ok := c.Get(LabelTraceID); ok {
		t.Fatal("label survived Release")
	}
	if c.Context != nil {
		t.Fatal("Release did not drop the parent context")
	}
}

func TestTraceIDAutoGeneratesAndIsStable(t *testing.T) {
	c := NewContext(stdctx.Background())
	defer c.Release()

	first, err := c.TraceID()
	if err != nil {
		t.Fatalf("TraceID() error = %v", err)
	}
	if len(first) != 32 {
		t.Fatalf("TraceID() = %q, want 32 hex chars", first)
	}

	second, err := c.TraceID()
	if err != nil {
		t.Fatalf("TraceID() second call error = %v", err)
	}
	if second != first {
		t.Fatalf("TraceID() not stable: %q then %q", first, second)
	}
	if got := c.GetString(LabelTraceID); got != first {
		t.Fatalf("stored trace id = %q, want %q", got, first)
	}

	other := NewContext(stdctx.Background())
	defer other.Release()
	otherID, err := other.TraceID()
	if err != nil {
		t.Fatalf("other.TraceID() error = %v", err)
	}
	if otherID == first {
		t.Fatalf("two contexts generated the same trace id %q", first)
	}
}

func TestSetTraceIDOverridesGeneratedValue(t *testing.T) {
	c := NewContext(stdctx.Background())
	defer c.Release()

	c.SetTraceID("incoming-trace")
	got, err := c.TraceID()
	if err != nil {
		t.Fatalf("TraceID() error = %v", err)
	}
	if got != "incoming-trace" {
		t.Fatalf("TraceID() = %q, want incoming-trace", got)
	}
	if v := c.Value(LabelTraceID); v != "incoming-trace" {
		t.Fatalf("Value(LabelTraceID) = %v, want incoming-trace", v)
	}
}

func TestTraceIDRegeneratesAfterEmptySet(t *testing.T) {
	c := NewContext(stdctx.Background())
	defer c.Release()

	c.SetTraceID("")
	got, err := c.TraceID()
	if err != nil {
		t.Fatalf("TraceID() error = %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("TraceID() after empty SetTraceID = %q, want 32 hex chars", got)
	}
}

func TestTimingEnabledAndLifecycleTimestamps(t *testing.T) {
	c := NewContext(stdctx.Background())
	defer c.Release()

	if c.IsTimingEnabled() {
		t.Fatal("timing enabled by default")
	}
	c.EnableTiming()
	if !c.IsTimingEnabled() {
		t.Fatal("IsTimingEnabled() = false after EnableTiming()")
	}

	submitted := time.Now().Add(-time.Hour)
	enqueued := submitted.Add(10 * time.Millisecond)
	started := enqueued.Add(20 * time.Millisecond)
	completed := started.Add(30 * time.Millisecond)
	c.Store(LabelTimingSubmitted, submitted)
	c.Store(LabelTimingEnqueued, enqueued)
	c.Store(LabelTimingStarted, started)
	c.Store(LabelTimingCompleted, completed)

	gotSubmitted, gotEnqueued, gotStarted, gotCompleted := c.Timing()
	if !gotSubmitted.Equal(submitted) || !gotEnqueued.Equal(enqueued) ||
		!gotStarted.Equal(started) || !gotCompleted.Equal(completed) {
		t.Fatalf("Timing() = (%v, %v, %v, %v), want (%v, %v, %v, %v)",
			gotSubmitted, gotEnqueued, gotStarted, gotCompleted,
			submitted, enqueued, started, completed)
	}

	if got, want := c.TotalLatency(), completed.Sub(submitted); got != want {
		t.Fatalf("TotalLatency() = %v, want %v", got, want)
	}
	if got, want := c.QueueLatency(), started.Sub(submitted); got != want {
		t.Fatalf("QueueLatency() = %v, want %v", got, want)
	}
	if got, want := c.ExecLatency(), completed.Sub(started); got != want {
		t.Fatalf("ExecLatency() = %v, want %v", got, want)
	}
	if got, want := c.HandoffLatency(), enqueued.Sub(submitted); got != want {
		t.Fatalf("HandoffLatency() = %v, want %v", got, want)
	}
	if got, want := c.QueueWaitLatency(), started.Sub(enqueued); got != want {
		t.Fatalf("QueueWaitLatency() = %v, want %v", got, want)
	}
	if got := c.Duration(); got <= 0 {
		t.Fatalf("Duration() = %v, want > 0", got)
	}
}

func TestLatencyHelpersReturnZeroWithoutTimestamps(t *testing.T) {
	c := NewContext(stdctx.Background())
	defer c.Release()

	if got := c.Duration(); got != 0 {
		t.Fatalf("Duration() = %v, want 0", got)
	}
	if got := c.TotalLatency(); got != 0 {
		t.Fatalf("TotalLatency() = %v, want 0", got)
	}
	if got := c.QueueLatency(); got != 0 {
		t.Fatalf("QueueLatency() = %v, want 0", got)
	}
	if got := c.ExecLatency(); got != 0 {
		t.Fatalf("ExecLatency() = %v, want 0", got)
	}
	if got := c.HandoffLatency(); got != 0 {
		t.Fatalf("HandoffLatency() = %v, want 0", got)
	}
	if got := c.QueueWaitLatency(); got != 0 {
		t.Fatalf("QueueWaitLatency() = %v, want 0", got)
	}

	// Partial timestamps still yield zero for the affected helpers.
	c.Store(LabelTimingStarted, time.Now().Add(-time.Second))
	if got := c.ExecLatency(); got != 0 {
		t.Fatalf("ExecLatency() without completed = %v, want 0", got)
	}
	if got := c.QueueLatency(); got != 0 {
		t.Fatalf("QueueLatency() without submitted = %v, want 0", got)
	}
}

func TestConcurrentLabelAccess(t *testing.T) {
	c := NewContext(stdctx.Background())
	defer c.Release()

	const (
		goroutines = 8
		iterations = 200
	)
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", g)
			for i := 0; i < iterations; i++ {
				c.Store(key, i)
				if _, ok := c.Get(key); !ok {
					t.Errorf("Get(%s) missed after Store", key)
					return
				}
				c.GetString(key)
				c.GetTime(key)
				if _, ok := c.Value(key).(int); !ok {
					t.Errorf("Value(%s) is not an int", key)
					return
				}
				if _, err := c.TraceID(); err != nil {
					t.Errorf("TraceID() error = %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	if got := c.GetString(LabelTraceID); len(got) != 32 {
		t.Fatalf("trace id after concurrent access = %q, want 32 hex chars", got)
	}
}
