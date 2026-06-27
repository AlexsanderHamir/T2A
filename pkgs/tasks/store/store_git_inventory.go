package store

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// worktreePathKey normalizes filesystem paths for Hamix ↔ git comparisons.
// Git paths use forward slashes; DB rows may use OS-native separators on Windows.
func worktreePathKey(path string) string {
	key := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key
}

// WorktreeInventoryRow is a live git worktree plus Hamix registration state.
type WorktreeInventoryRow struct {
	Path       string
	Branch     string
	IsMain     bool
	Detached   bool
	Registered bool
}

// GitWorktreeProbeResult describes whether a path is a linked, registerable worktree.
type GitWorktreeProbeResult struct {
	Path       string
	Linked     bool
	IsMain     bool
	Branch     string
	Registered bool
}

// RepoWorktreeInventory lists live git worktrees for a repository and marks registered paths.
func (s *Store) RepoWorktreeInventory(
	ctx context.Context,
	repo domain.GitRepository,
	gitSvc gitwork.Service,
) ([]WorktreeInventoryRow, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.RepoWorktreeInventory")
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}
	registered, err := s.ListGitWorktreesByRepo(ctx, repo.ID)
	if err != nil {
		return nil, err
	}
	registeredPaths := make(map[string]struct{}, len(registered))
	for _, wt := range registered {
		registeredPaths[worktreePathKey(wt.Path)] = struct{}{}
	}
	opened, err := gitSvc.OpenRepository(ctx, repo.Path)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	live, err := gitSvc.ListWorktrees(ctx, opened)
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	out := make([]WorktreeInventoryRow, 0, len(live))
	for _, wt := range live {
		_, isRegistered := registeredPaths[worktreePathKey(wt.Path)]
		out = append(out, WorktreeInventoryRow{
			Path:       wt.Path,
			Branch:     wt.Branch,
			IsMain:     wt.IsMain,
			Detached:   strings.TrimSpace(wt.Branch) == "",
			Registered: isRegistered,
		})
	}
	return out, nil
}

// FindWorktreeInInventory returns the inventory row for an absolute worktree path.
func FindWorktreeInInventory(rows []WorktreeInventoryRow, path string) (*WorktreeInventoryRow, bool) {
	want := worktreePathKey(path)
	for i := range rows {
		if worktreePathKey(rows[i].Path) == want {
			return &rows[i], true
		}
	}
	return nil, false
}

// ProbeGitWorktree checks whether path is a linked worktree of the repository.
func (s *Store) ProbeGitWorktree(
	ctx context.Context,
	repoID, path string,
	gitSvc gitwork.Service,
) (GitWorktreeProbeResult, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ProbeGitWorktree")
	path = strings.TrimSpace(path)
	if path == "" {
		return GitWorktreeProbeResult{}, fmt.Errorf("%w: path required", domain.ErrInvalidInput)
	}
	repo, err := s.GetGitRepositoryByID(ctx, repoID)
	if err != nil {
		return GitWorktreeProbeResult{}, err
	}
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}
	belongs, err := gitSvc.BelongsToRepository(ctx, path, repo.Path)
	if err != nil {
		return GitWorktreeProbeResult{}, fmt.Errorf("belongs to repository: %w", err)
	}
	if !belongs {
		return GitWorktreeProbeResult{Path: filepath.Clean(path), Linked: false}, nil
	}
	inventory, err := s.RepoWorktreeInventory(ctx, repo, gitSvc)
	if err != nil {
		return GitWorktreeProbeResult{}, err
	}
	row, found := FindWorktreeInInventory(inventory, path)
	if !found {
		return GitWorktreeProbeResult{Path: filepath.Clean(path), Linked: false}, nil
	}
	return GitWorktreeProbeResult{
		Path:       row.Path,
		Linked:     true,
		IsMain:     row.IsMain,
		Branch:     row.Branch,
		Registered: row.Registered,
	}, nil
}
