package worker_test

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/worker"
)

func TestPool_NewPoolClampsConcurrency(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	q := agents.NewMemoryQueue(1)
	r := runnerfake.New()

	low := worker.NewPool(h.store, q, r, worker.Options{}, 0)
	if got := len(low.Slots()); got != 1 {
		t.Fatalf("concurrency 0 slots = %d, want 1", got)
	}

	high := worker.NewPool(h.store, q, r, worker.Options{}, 64)
	if got := len(high.Slots()); got != 32 {
		t.Fatalf("concurrency 64 slots = %d, want 32", got)
	}
}

func TestPool_RunNilReceiver(t *testing.T) {
	t.Parallel()
	var pool *worker.Pool
	if err := pool.Run(context.Background()); err == nil {
		t.Fatal("expected error from nil pool Run")
	}
}

func TestPool_CancelCurrentRunNilReceiver(t *testing.T) {
	t.Parallel()
	var pool *worker.Pool
	if pool.CancelCurrentRun() {
		t.Fatal("CancelCurrentRun on nil pool should return false")
	}
}

func TestPool_RunReturnsOnCancel(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	pool := worker.NewPool(h.store, h.queue, runnerfake.New(), worker.Options{}, 2)

	done := make(chan error, 1)
	go func() {
		done <- pool.Run(ctx)
	}()

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("pool Run err = %v, want nil on cancel", err)
	}
}

func TestPool_SharesWorktreeGateAcrossSlots(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	q := agents.NewMemoryQueue(1)
	pool := worker.NewPool(h.store, q, runnerfake.New(), worker.Options{}, 2)
	slots := pool.Slots()
	if len(slots) != 2 {
		t.Fatalf("slot count = %d, want 2", len(slots))
	}
	if slots[0] == slots[1] {
		t.Fatal("pool slots must be distinct workers")
	}
}
