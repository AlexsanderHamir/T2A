package worker_test

import (
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/gittest"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func seedWorkerTestGit(t *testing.T, st *store.Store) (worktreeID, workDir string) {
	t.Helper()
	return gittest.SeedWorktreeTemp(t, st)
}

func (h *harness) gitBinding() *string {
	wt := h.worktreeID
	return &wt
}
