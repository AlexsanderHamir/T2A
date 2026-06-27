package worker

import (
	"sync"
	"testing"
	"time"
)

func TestWorktreeGate_LockSerializesSameWorktree(t *testing.T) {
	t.Parallel()
	var gate WorktreeGate

	unlock := gate.Lock("wt-a")
	second := make(chan struct{})
	go func() {
		u := gate.Lock("wt-a")
		close(second)
		u()
	}()

	select {
	case <-second:
		t.Fatal("second Lock on same worktree acquired before first unlock")
	case <-time.After(20 * time.Millisecond):
	}
	unlock()

	select {
	case <-second:
	case <-time.After(time.Second):
		t.Fatal("second Lock did not acquire after unlock")
	}
}

func TestWorktreeGate_TryLockReturnsFalseWhenBusy(t *testing.T) {
	t.Parallel()
	var gate WorktreeGate

	unlock := gate.Lock("wt-b")
	_, ok := gate.TryLock("wt-b")
	if ok {
		t.Fatal("TryLock expected false while worktree locked")
	}
	unlock()

	unlock2, ok := gate.TryLock("wt-b")
	if !ok {
		t.Fatal("TryLock expected true when worktree free")
	}
	unlock2()
}

func TestWorktreeGate_DifferentWorktreesDoNotBlock(t *testing.T) {
	t.Parallel()
	var gate WorktreeGate

	unlockA := gate.Lock("wt-a")
	unlockB, ok := gate.TryLock("wt-b")
	if !ok {
		t.Fatal("TryLock on distinct worktree should succeed while wt-a is locked")
	}
	unlockB()
	unlockA()
}

func TestWorktreeGate_mutexReusesPerWorktreeID(t *testing.T) {
	t.Parallel()
	var gate WorktreeGate
	mu1 := gate.mutex("shared")
	mu2 := gate.mutex("shared")
	if mu1 != mu2 {
		t.Fatal("mutex helper should return same *sync.Mutex for one worktree id")
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		mu2.Lock()
		mu2.Unlock()
	}()
	mu1.Lock()
	mu1.Unlock()
	wg.Wait()
}
