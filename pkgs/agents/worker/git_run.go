package worker

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
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
	WorktreeID       string
	BranchID         string
	WorktreeBranchID string
	WorktreePath     string
	BranchName       string
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func taskHasBinding(task *domain.Task) bool {
	if task == nil {
		return false
	}
	if task.WorktreeBranchID != nil && strings.TrimSpace(*task.WorktreeBranchID) != "" {
		return true
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
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "agent.worker.Worker.resolveTaskGitBinding",
		"task_id", task.ID)
	if !taskHasBinding(task) {
		return nil, fmt.Errorf("missing_task_binding")
	}
	var (
		gitCtx  store.TaskGitContext
		err     error
		binding taskGitBinding
	)
	if task.WorktreeBranchID != nil && strings.TrimSpace(*task.WorktreeBranchID) != "" {
		wbID := strings.TrimSpace(*task.WorktreeBranchID)
		gitCtx, err = w.store.ResolveTaskGitContextFromAssociation(ctx, wbID)
		if err != nil {
			return nil, mapResolveGitContextError(err)
		}
		assoc, assocErr := w.store.GetWorktreeBranchByID(ctx, wbID)
		if assocErr != nil {
			return nil, mapResolveGitContextError(assocErr)
		}
		binding = taskGitBinding{
			WorktreeID:       assoc.WorktreeID,
			BranchID:         assoc.BranchID,
			WorktreeBranchID: wbID,
			WorktreePath:     gitCtx.WorktreePath,
			BranchName:       gitCtx.BranchName,
		}
	} else {
		gitCtx, err = w.store.ResolveTaskGitContext(ctx, *task.WorktreeID, *task.BranchID)
		if err != nil {
			return nil, mapResolveGitContextError(err)
		}
		binding = taskGitBinding{
			WorktreeID:   strings.TrimSpace(*task.WorktreeID),
			BranchID:     strings.TrimSpace(*task.BranchID),
			WorktreePath: gitCtx.WorktreePath,
			BranchName:   gitCtx.BranchName,
		}
	}
	if _, err := os.Stat(binding.WorktreePath); err != nil {
		return nil, fmt.Errorf("worktree_missing: %w", err)
	}
	return &binding, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapResolveGitContextError(err error) error {
	if domain.GitErrCode(err) == domain.GitCodeWorktreeNotFound {
		return fmt.Errorf("worktree_missing: %w", err)
	}
	if domain.GitErrCode(err) == domain.GitCodeBranchNotFound {
		return fmt.Errorf("branch_missing: %w", err)
	}
	if domain.GitErrCode(err) == domain.GitCodeBranchNotAssociated {
		return fmt.Errorf("branch_not_associated: %w", err)
	}
	return err
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (w *Worker) worktreeMutex(worktreeID string) *sync.Mutex {
	v, _ := w.worktreeLocks.LoadOrStore(worktreeID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// prepareGitRun locks the worktree, checks out the branch, sets active_branch_id, and sets harness WorkingDir.
func (w *Worker) prepareGitRun(ctx context.Context, binding *taskGitBinding) (release func(), err error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "agent.worker.Worker.prepareGitRun",
		"worktree_id", binding.WorktreeID)
	mu := w.worktreeMutex(binding.WorktreeID)
	mu.Lock()
	if guardErr := w.store.GuardBranchNotActiveElsewhere(ctx, binding.WorktreeID, binding.BranchID); guardErr != nil {
		mu.Unlock()
		return nil, mapGitPrepError(guardErr)
	}
	checkoutErr := w.gitService().Checkout(ctx, binding.WorktreePath, binding.BranchName)
	if checkoutErr != nil {
		mu.Unlock()
		return nil, mapGitPrepError(checkoutErr)
	}
	if setErr := w.store.SetActiveBranch(ctx, binding.WorktreeID, binding.BranchID); setErr != nil {
		mu.Unlock()
		return nil, mapGitPrepError(setErr)
	}
	w.harness.SetWorkingDir(binding.WorktreePath)
	return func() {
		if clearErr := w.store.ClearActiveBranch(ctx, binding.WorktreeID, binding.BranchID); clearErr != nil {
			slog.Warn("agent worker clear active branch failed", "cmd", calltrace.LogCmd,
				"operation", "agent.worker.Worker.prepareGitRun.clear_active",
				"worktree_id", binding.WorktreeID, "branch_id", binding.BranchID, "err", clearErr)
		}
		mu.Unlock()
	}, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapGitPrepError(err error) error {
	if domain.GitErrCode(err) == domain.GitCodeBranchActiveElsewhere {
		return fmt.Errorf("branch_active_elsewhere: %w", err)
	}
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
	slog.Warn("agent worker git prep failed", "cmd", calltrace.LogCmd,
		"operation", "agent.worker.Worker.abortRunningFromGitPrep",
		"task_id", taskID, "err", prepErr)
	failed := domain.StatusFailed
	if _, err := w.store.Update(ctx, taskID, store.UpdateTaskInput{Status: &failed}, domain.ActorAgent); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("agent worker git prep task transition failed", "cmd", calltrace.LogCmd,
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
