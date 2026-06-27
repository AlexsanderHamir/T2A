package worker

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

type taskGitBinding struct {
	WorktreeID   string
	BranchID     string
	WorktreePath string
	BranchName   string
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func taskHasBinding(task *domain.Task) bool {
	if task == nil {
		return false
	}
	return task.WorktreeID != nil && strings.TrimSpace(*task.WorktreeID) != ""
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
	wtID := strings.TrimSpace(*task.WorktreeID)
	gitCtx, err := w.store.ResolveTaskGitContext(ctx, wtID)
	if err != nil {
		return nil, mapResolveGitContextError(err)
	}
	binding := &taskGitBinding{
		WorktreeID:   gitCtx.WorktreeID,
		BranchID:     gitCtx.BranchID,
		WorktreePath: gitCtx.WorktreePath,
		BranchName:   gitCtx.BranchName,
	}
	if _, err := os.Stat(binding.WorktreePath); err != nil {
		return nil, fmt.Errorf("worktree_missing: %w", err)
	}
	return binding, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapResolveGitContextError(err error) error {
	if domain.GitErrCode(err) == domain.GitCodeWorktreeNotFound {
		return fmt.Errorf("worktree_missing: %w", err)
	}
	if domain.GitErrCode(err) == domain.GitCodeBranchNotFound {
		return fmt.Errorf("branch_missing: %w", err)
	}
	return err
}

// prepareGitRun verifies HEAD matches the bound branch and sets harness WorkingDir.
// The caller must hold the worktree gate lock for binding.WorktreeID.
func (w *Worker) prepareGitRun(ctx context.Context, binding *taskGitBinding) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "agent.worker.Worker.prepareGitRun",
		"worktree_id", binding.WorktreeID)
	head, headErr := w.gitService().WorktreeCurrentBranch(ctx, binding.WorktreePath)
	if headErr != nil {
		return mapGitPrepError(headErr)
	}
	if head != binding.BranchName {
		return fmt.Errorf("branch_mismatch: worktree HEAD %q, bound branch %q", head, binding.BranchName)
	}
	w.harness.SetWorkingDir(binding.WorktreePath)
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapGitPrepError(err error) error {
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
	if err := w.prepareGitRun(ctx, binding); err != nil {
		w.abortRunningFromGitPrep(ctx, task.ID, err)
		return
	}
	run()
}
