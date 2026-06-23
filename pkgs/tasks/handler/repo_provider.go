package handler

import (
	"context"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/repo"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// RepoProvider returns the *repo.Root that /repo/* handlers and prompt
// mention validation should consult for the current request. Production
// wiring resolves paths from git_worktrees via OpenWorktreeRoot.
type RepoProvider interface {
	OpenWorktreeRoot(ctx context.Context, worktreeID string) (root *repo.Root, reason string, err error)
}

const (
	// RepoReasonOpenFailed: worktree path is set but repo.OpenRoot rejected it.
	RepoReasonOpenFailed = "worktree_open_failed"
	// RepoReasonWorktreeIDRequired: /repo/* called without worktree_id.
	RepoReasonWorktreeIDRequired = "worktree_id_required"
	// RepoReasonWorktreeNotFound: unknown worktree_id.
	RepoReasonWorktreeNotFound = "worktree_not_found"
)

// staticRepoProvider returns a fixed *repo.Root for tests.
type staticRepoProvider struct {
	root *repo.Root
}

// NewStaticRepoProvider wraps r so OpenWorktreeRoot returns it when worktreeID is non-empty.
func NewStaticRepoProvider(r *repo.Root) RepoProvider {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.NewStaticRepoProvider")
	return &staticRepoProvider{root: r}
}

func (p *staticRepoProvider) OpenWorktreeRoot(_ context.Context, worktreeID string) (*repo.Root, string, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.staticRepoProvider.OpenWorktreeRoot")
	if strings.TrimSpace(worktreeID) == "" {
		return nil, RepoReasonWorktreeIDRequired, nil
	}
	if p.root == nil {
		return nil, RepoReasonWorktreeNotFound, nil
	}
	return p.root, "", nil
}

// settingsRepoProvider opens git worktree paths from the store at request time.
type settingsRepoProvider struct {
	store *store.Store
}

// NewSettingsRepoProvider returns a provider backed by git_worktrees in s.
func NewSettingsRepoProvider(s *store.Store) RepoProvider {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.NewSettingsRepoProvider")
	return &settingsRepoProvider{store: s}
}

func (p *settingsRepoProvider) OpenWorktreeRoot(ctx context.Context, worktreeID string) (*repo.Root, string, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.settingsRepoProvider.OpenWorktreeRoot")
	if p == nil || p.store == nil {
		return nil, RepoReasonWorktreeNotFound, nil
	}
	worktreeID = strings.TrimSpace(worktreeID)
	if worktreeID == "" {
		return nil, RepoReasonWorktreeIDRequired, nil
	}
	wt, err := p.store.GetGitWorktreeByID(ctx, worktreeID)
	if err != nil {
		if domain.GitErrCode(err) == domain.GitCodeWorktreeNotFound {
			return nil, RepoReasonWorktreeNotFound, nil
		}
		return nil, "", err
	}
	root, openErr := repo.OpenRoot(wt.Path)
	if openErr != nil {
		return nil, RepoReasonOpenFailed, openErr
	}
	return root, "", nil
}
