package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

type taskGitBinding struct {
	WorktreeID   string
	WorktreePath string
	BranchName   string
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func taskHasBinding(task *domain.Task) bool {
	if task == nil {
		return false
	}
	return task.WorktreeID != nil && task.BranchID != nil &&
		strings.TrimSpace(*task.WorktreeID) != "" &&
		strings.TrimSpace(*task.BranchID) != ""
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (w *Worker) gitService() gitwork.Service {
	if w.gitSvc != nil {
		return w.gitSvc
	}
	return gitwork.New()
}

func (w *Worker) resolveTaskGitBinding(ctx context.Context, task *domain.Task) (*taskGitBinding, error) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.resolveTaskGitBinding",
		"task_id", task.ID)
	if !taskHasBinding(task) {
		return nil, fmt.Errorf("missing_task_binding")
	}
	gitCtx, err := w.store.ResolveTaskGitContext(ctx, *task.WorktreeID, *task.BranchID)
	if err != nil {
		if domain.GitErrCode(err) == domain.GitCodeWorktreeNotFound {
			return nil, fmt.Errorf("worktree_missing: %w", err)
		}
		if domain.GitErrCode(err) == domain.GitCodeBranchNotFound {
			return nil, fmt.Errorf("branch_missing: %w", err)
		}
		return nil, err
	}
	if _, err := os.Stat(gitCtx.WorktreePath); err != nil {
		return nil, fmt.Errorf("worktree_missing: %w", err)
	}
	return &taskGitBinding{
		WorktreeID:   strings.TrimSpace(*task.WorktreeID),
		WorktreePath: gitCtx.WorktreePath,
		BranchName:   gitCtx.BranchName,
	}, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (w *Worker) worktreeMutex(worktreeID string) *sync.Mutex {
	v, _ := w.worktreeLocks.LoadOrStore(worktreeID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// prepareGitRun locks the worktree, checks out the branch, and sets harness WorkingDir.
func (w *Worker) prepareGitRun(ctx context.Context, binding *taskGitBinding) (release func(), err error) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.prepareGitRun",
		"worktree_id", binding.WorktreeID)
	mu := w.worktreeMutex(binding.WorktreeID)
	mu.Lock()
	checkoutErr := w.gitService().Checkout(ctx, binding.WorktreePath, binding.BranchName)
	if checkoutErr != nil {
		mu.Unlock()
		return nil, mapGitPrepError(checkoutErr)
	}
	w.harness.SetWorkingDir(binding.WorktreePath)
	return mu.Unlock, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapGitPrepError(err error) error {
	if errors.Is(err, gitwork.ErrDirty) {
		return fmt.Errorf("worktree_dirty: %w", err)
	}
	if errors.Is(err, gitwork.ErrBranchCheckedOut) {
		return fmt.Errorf("branch_checked_out: %w", err)
	}
	if errors.Is(err, gitwork.ErrNotARepository) {
		return fmt.Errorf("worktree_missing: %w", err)
	}
	return err
}

func (w *Worker) abortRunningFromGitPrep(ctx context.Context, taskID string, prepErr error) {
	slog.Warn("agent worker git prep failed", "cmd", workerLogCmd,
		"operation", "agent.worker.Worker.abortRunningFromGitPrep",
		"task_id", taskID, "err", prepErr)
	failed := domain.StatusFailed
	if _, err := w.store.Update(ctx, taskID, store.UpdateTaskInput{Status: &failed}, domain.ActorAgent); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("agent worker git prep task transition failed", "cmd", workerLogCmd,
				"operation", "agent.worker.Worker.abortRunningFromGitPrep.err",
				"task_id", taskID, "err", err)
		}
	}
}

//funclogmeasure:skip category=delegate-already-logs reason="Orchestrator; resolveTaskGitBinding and prepareGitRun emit operation traces."
func (w *Worker) runWithGitPrep(ctx context.Context, task *domain.Task, run func()) {
	binding, err := w.resolveTaskGitBinding(ctx, task)
	if err != nil {
		w.abortRunningFromGitPrep(ctx, task.ID, err)
		return
	}
	release, err := w.prepareGitRun(ctx, binding)
	if err != nil {
		w.abortRunningFromGitPrep(ctx, task.ID, err)
		return
	}
	defer release()
	run()
}
