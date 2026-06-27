package worker

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"log/slog"
	"sync"
)

// WorktreeGate serializes git prep per worktree. TryLock enables pool admission
// without blocking a slot when the worktree is already in use.
type WorktreeGate struct {
	locks sync.Map // worktreeID -> *sync.Mutex
}

func (g *WorktreeGate) mutex(worktreeID string) *sync.Mutex {
	v, _ := g.locks.LoadOrStore(worktreeID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// Lock blocks until the worktree is available and returns an unlock function.
func (g *WorktreeGate) Lock(worktreeID string) func() {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "agent.worker.WorktreeGate.Lock",
		"worktree_id", worktreeID)
	mu := g.mutex(worktreeID)
	mu.Lock()
	return func() { mu.Unlock() }
}

// TryLock acquires the worktree lock without blocking. ok is false when busy.
func (g *WorktreeGate) TryLock(worktreeID string) (unlock func(), ok bool) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "agent.worker.WorktreeGate.TryLock",
		"worktree_id", worktreeID)
	mu := g.mutex(worktreeID)
	if !mu.TryLock() {
		return nil, false
	}
	return func() { mu.Unlock() }, true
}
