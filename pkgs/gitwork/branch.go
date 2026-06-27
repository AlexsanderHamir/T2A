package gitwork

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"log/slog"
	"strings"
)

func (s *DefaultService) ListBranches(ctx context.Context, repo *Repository) ([]Branch, error) {
	slog.DebugContext(ctx, "trace", "cmd", calltrace.LogCmd, "operation", "gitwork.ListBranches")
	if repo == nil {
		return nil, ErrNotARepository
	}
	current, err := s.runGit(ctx, repo.Root, "branch", "--show-current")
	if err != nil {
		return nil, err
	}
	current = strings.TrimSpace(current)

	out, err := s.runGit(ctx, repo.Root, "for-each-ref",
		"--format=%(refname:short) %(objectname) %(upstream:short)",
		"refs/heads/")
	if err != nil {
		return nil, err
	}
	var branches []Branch
	for _, line := range splitNonEmptyLines(out) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		b := Branch{
			Name:      fields[0],
			HeadSHA:   fields[1],
			IsCurrent: fields[0] == current,
		}
		if len(fields) >= 3 {
			b.Upstream = fields[2]
		}
		branches = append(branches, b)
	}
	return branches, nil
}

func (s *DefaultService) CreateBranch(ctx context.Context, repo *Repository, name, startPoint string) (*Branch, error) {
	slog.DebugContext(ctx, "trace", "cmd", calltrace.LogCmd, "operation", "gitwork.CreateBranch")
	if repo == nil {
		return nil, ErrNotARepository
	}
	args := []string{"branch", name}
	if startPoint != "" {
		args = append(args, startPoint)
	}
	if _, err := s.runGit(ctx, repo.Root, args...); err != nil {
		return nil, mapBranchCreateErr(err)
	}
	sha, err := s.runGit(ctx, repo.Root, "rev-parse", name)
	if err != nil {
		return nil, err
	}
	return &Branch{Name: name, HeadSHA: sha}, nil
}

func (s *DefaultService) DeleteBranch(ctx context.Context, repo *Repository, name string, force bool) error {
	slog.DebugContext(ctx, "trace", "cmd", calltrace.LogCmd, "operation", "gitwork.DeleteBranch")
	if repo == nil {
		return ErrNotARepository
	}
	flag := "-d"
	if force {
		flag = "-D"
	}
	if _, err := s.runGit(ctx, repo.Root, "branch", flag, name); err != nil {
		return mapBranchDeleteErr(err)
	}
	return nil
}

func (s *DefaultService) WorktreeCurrentBranch(ctx context.Context, worktreePath string) (string, error) {
	slog.DebugContext(ctx, "trace", "cmd", calltrace.LogCmd, "operation", "gitwork.WorktreeCurrentBranch")
	abs, err := absPath(worktreePath)
	if err != nil {
		return "", err
	}
	out, err := s.runGit(ctx, abs, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *DefaultService) Checkout(ctx context.Context, worktreePath, branch string) error {
	slog.DebugContext(ctx, "trace", "cmd", calltrace.LogCmd, "operation", "gitwork.Checkout")
	abs, err := absPath(worktreePath)
	if err != nil {
		return err
	}
	if _, err := s.runGit(ctx, abs, "checkout", branch); err != nil {
		return mapCheckoutErr(err)
	}
	return nil
}
