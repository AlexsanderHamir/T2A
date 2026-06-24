package worker_test

import (
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/gittest"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func seedWorkerTestGit(t *testing.T, st *store.Store) (worktreeBranchID, workDir string) {
	t.Helper()
	return gittest.SeedWorktreeBranchTemp(t, st)
}

func (h *harness) gitBinding() *string {
	wb := h.worktreeBranchID
	return &wb
}
