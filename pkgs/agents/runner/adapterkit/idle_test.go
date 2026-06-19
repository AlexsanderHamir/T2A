package adapterkit

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStreamIdleWatchdog_firesTiersThenCancels(t *testing.T) {
	t.Parallel()
	stuck := 200 * time.Millisecond
	suspicious, killPending := StreamIdleThresholds(stuck)
	if suspicious != 100*time.Millisecond {
		t.Fatalf("suspicious=%v want 100ms", suspicious)
	}
	if killPending != 100*time.Millisecond && killPending != 150*time.Millisecond {
		t.Fatalf("killPending=%v unexpected for stuck=%v", killPending, stuck)
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	var mu sync.Mutex
	var kinds []StreamIdleKind
	w := newStreamIdleWatchdog(StreamIdleConfig{
		Stuck:  stuck,
		Cancel: cancel,
		OnIdle: func(kind StreamIdleKind) {
			mu.Lock()
			kinds = append(kinds, kind)
			mu.Unlock()
		},
	})
	go w.run(ctx)
	defer w.close()

	onLine := w.wrap(func([]byte) {})
	onLine([]byte(`{"type":"system"}`))

	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		done := len(kinds) >= 2 && errors.Is(context.Cause(ctx), ErrStreamIdle)
		mu.Unlock()
		if done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for idle tiers; kinds=%v cause=%v", kinds, context.Cause(ctx))
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(kinds) < 2 {
		t.Fatalf("kinds=%v want suspicious and kill_pending", kinds)
	}
	if kinds[0] != StreamIdleSuspicious {
		t.Fatalf("first kind=%v want suspicious", kinds[0])
	}
	if !errors.Is(context.Cause(ctx), ErrStreamIdle) {
		t.Fatalf("cause=%v want ErrStreamIdle", context.Cause(ctx))
	}
}

func TestStreamIdleWatchdog_graceUntilFirstLine(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	w := newStreamIdleWatchdog(StreamIdleConfig{
		Stuck:  80 * time.Millisecond,
		Cancel: cancel,
	})
	go w.run(ctx)
	defer w.close()

	time.Sleep(150 * time.Millisecond)
	if ctx.Err() != nil {
		t.Fatalf("expected no cancel before first line, got %v", context.Cause(ctx))
	}
}

func TestStreamIdleThresholds_default60s(t *testing.T) {
	t.Parallel()
	suspicious, killPending := StreamIdleThresholds(60 * time.Second)
	if suspicious != 30*time.Second {
		t.Fatalf("suspicious=%v", suspicious)
	}
	if killPending != 55*time.Second {
		t.Fatalf("killPending=%v", killPending)
	}
}

func TestScanStdoutLines_invokesCallback(t *testing.T) {
	t.Parallel()
	var got []string
	err := ScanStdoutLines(strings.NewReader("a\nb\n"), nil, func(line []byte) {
		got = append(got, string(line))
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got=%v", got)
	}
}
