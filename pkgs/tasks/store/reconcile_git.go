package store

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	reconcileStatusOK                 = "ok"
	reconcileStatusNeedsBootstrapPath = "needs_bootstrap_path"
	reconcileStatusPartial            = "partial"
)

// ReconcileGitInput configures repository/worktree path sync with git.
type ReconcileGitInput struct {
	BootstrapPath string
	RepairGit     bool
	DryRun        bool
	AllowRemove   bool
}

// ReconcileGitOutput is the structured reconcile result for API and operators.
type ReconcileGitOutput struct {
	Status string
	Report ReconcileReport
}

// ReconcileReport counts reconcile actions and skipped rows.
type ReconcileReport struct {
	RepoPathUpdated      bool
	WorktreesPathUpdated int
	WorktreesAdded       int
	WorktreesRemoved     int
	BranchesHeadUpdated  int
	WorktreesSkipped     []ReconcileSkippedWorktree
	NeedsBranchBind      []ReconcileNeedsBranchBind
}

// ReconcileSkippedWorktree describes a DB row reconcile could not remove.
type ReconcileSkippedWorktree struct {
	WorktreeID string
	Reason     string
}

// ReconcileNeedsBranchBind describes a live worktree without Hamix branch binding.
type ReconcileNeedsBranchBind struct {
	Path   string
	Branch string
}

// ReconcileGitRepository syncs Hamix git rows with git worktree list output,
// preserving stable worktree IDs when paths move on disk.
func (s *Store) ReconcileGitRepository(
	ctx context.Context,
	projectID, repoID string,
	input ReconcileGitInput,
	gitSvc gitwork.Service,
) (ReconcileGitOutput, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ReconcileGitRepository")
	repo, err := s.GetGitRepository(ctx, projectID, repoID)
	if err != nil {
		return ReconcileGitOutput{}, err
	}
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}

	opened, _, err := s.openRepoForReconcile(ctx, repo, strings.TrimSpace(input.BootstrapPath), gitSvc)
	if err != nil {
		return ReconcileGitOutput{}, err
	}
	if opened == nil {
		return ReconcileGitOutput{
			Status: reconcileStatusNeedsBootstrapPath,
			Report: ReconcileReport{},
		}, nil
	}

	if input.RepairGit {
		if err := gitSvc.RepairWorktrees(ctx, opened); err != nil {
			return ReconcileGitOutput{}, fmt.Errorf("repair worktrees: %w", err)
		}
		if err := gitSvc.PruneWorktrees(ctx, opened); err != nil {
			return ReconcileGitOutput{}, fmt.Errorf("prune worktrees: %w", err)
		}
	}

	mainRoot, commonDir, err := gitSvc.ResolveRegistration(ctx, opened.Root)
	if err != nil {
		return ReconcileGitOutput{}, fmt.Errorf("resolve registration: %w", err)
	}

	live, err := gitSvc.ListWorktrees(ctx, opened)
	if err != nil {
		return ReconcileGitOutput{}, fmt.Errorf("list worktrees: %w", err)
	}
	live = filterLiveWorktrees(live)

	branches, err := s.ListGitBranchesByRepo(ctx, repoID)
	if err != nil {
		return ReconcileGitOutput{}, err
	}
	branchByID := make(map[string]domain.GitBranch, len(branches))
	for _, b := range branches {
		branchByID[b.ID] = b
	}

	var dbRows []model.GitWorktree
	if err := s.db.WithContext(ctx).Where("repository_id = ?", repoID).Find(&dbRows).Error; err != nil {
		return ReconcileGitOutput{}, err
	}

	report := ReconcileReport{}
	now := time.Now().UTC()

	apply := func(fn func(tx *gorm.DB) error) error {
		if input.DryRun {
			return fn(s.db.WithContext(ctx).Session(&gorm.Session{DryRun: true}))
		}
		return s.db.WithContext(ctx).Transaction(fn)
	}

	err = apply(func(tx *gorm.DB) error {
		if worktreePathKey(repo.Path) != worktreePathKey(mainRoot) ||
			worktreePathKey(repo.GitCommonDir) != worktreePathKey(commonDir) {
			report.RepoPathUpdated = true
			if err := tx.Model(&model.GitRepository{}).Where("id = ?", repoID).Updates(map[string]any{
				"path":           mainRoot,
				"git_common_dir": commonDir,
				"updated_at":     now,
			}).Error; err != nil {
				return err
			}
		}

		matchedLive := make(map[string]struct{}, len(live))
		matchedRowIDs := make(map[string]struct{}, len(dbRows))
		liveByPath := liveWorktreesByPath(live)
		liveByBranch := liveWorktreesByBranch(live)

		for i := range dbRows {
			row := dbRows[i]
			if row.IsMain {
				if worktreePathKey(row.Path) != worktreePathKey(mainRoot) {
					report.WorktreesPathUpdated++
					if err := tx.Model(&model.GitWorktree{}).Where("id = ?", row.ID).
						Update("path", mainRoot).Error; err != nil {
						return err
					}
					row.Path = mainRoot
				}
				matchedRowIDs[row.ID] = struct{}{}
				for _, wt := range live {
					if wt.IsMain {
						matchedLive[worktreePathKey(wt.Path)] = struct{}{}
						break
					}
				}
				continue
			}

			var liveWT *gitwork.Worktree
			matched := false

			if wt, ok := liveByPath[worktreePathKey(row.Path)]; ok {
				liveWT = &wt
				matched = true
			} else if strings.TrimSpace(row.BranchID) != "" {
				br, ok := branchByID[row.BranchID]
				if !ok {
					report.WorktreesSkipped = append(report.WorktreesSkipped, ReconcileSkippedWorktree{
						WorktreeID: row.ID,
						Reason:     "path_and_branch_not_found",
					})
					continue
				}
				wt, ok := liveByBranch[br.Name]
				if !ok {
					report.WorktreesSkipped = append(report.WorktreesSkipped, ReconcileSkippedWorktree{
						WorktreeID: row.ID,
						Reason:     "path_and_branch_not_found",
					})
					continue
				}
				if countBranchOwners(dbRows, br.Name, branchByID) > 1 {
					return fmt.Errorf("%w: duplicate worktree rows for branch %q", domain.ErrInvalidInput, br.Name)
				}
				liveWT = &wt
				matched = true
			}

			if !matched {
				if strings.TrimSpace(row.BranchID) != "" {
					report.WorktreesSkipped = append(report.WorktreesSkipped, ReconcileSkippedWorktree{
						WorktreeID: row.ID,
						Reason:     "path_and_branch_not_found",
					})
				}
				continue
			}

			if strings.TrimSpace(row.BranchID) != "" && strings.TrimSpace(liveWT.Branch) != "" {
				br, ok := branchByID[row.BranchID]
				if ok && liveWT.Branch != br.Name {
					report.WorktreesSkipped = append(report.WorktreesSkipped, ReconcileSkippedWorktree{
						WorktreeID: row.ID,
						Reason:     "branch_checkout_mismatch",
					})
					matchedLive[worktreePathKey(liveWT.Path)] = struct{}{}
					matchedRowIDs[row.ID] = struct{}{}
					continue
				}
			}

			matchedLive[worktreePathKey(liveWT.Path)] = struct{}{}
			matchedRowIDs[row.ID] = struct{}{}
			if worktreePathKey(row.Path) != worktreePathKey(liveWT.Path) {
				report.WorktreesPathUpdated++
				if err := tx.Model(&model.GitWorktree{}).Where("id = ?", row.ID).
					Update("path", liveWT.Path).Error; err != nil {
					return err
				}
			}
		}

		dbByPath := make(map[string]model.GitWorktree, len(dbRows))
		for _, row := range dbRows {
			dbByPath[worktreePathKey(row.Path)] = row
		}

		for _, wt := range live {
			key := worktreePathKey(wt.Path)
			if _, ok := matchedLive[key]; ok {
				continue
			}
			if _, ok := dbByPath[key]; ok {
				continue
			}
			name := "discovered-" + worktreeDisplayName(wt.Path)
			if err := tx.Create(&model.GitWorktree{
				ID:           uuid.NewString(),
				RepositoryID: repoID,
				Path:         wt.Path,
				Name:         name,
				IsMain:       wt.IsMain,
				CreatedAt:    now,
			}).Error; err != nil {
				return err
			}
			report.WorktreesAdded++
			if strings.TrimSpace(wt.Branch) != "" {
				report.NeedsBranchBind = append(report.NeedsBranchBind, ReconcileNeedsBranchBind{
					Path:   wt.Path,
					Branch: wt.Branch,
				})
			}
		}

		if input.AllowRemove {
			for _, row := range dbRows {
				if row.IsMain {
					continue
				}
				if _, ok := matchedRowIDs[row.ID]; ok {
					continue
				}
				if _, ok := matchedLive[worktreePathKey(row.Path)]; ok {
					continue
				}
				stillLive := false
				for _, wt := range live {
					if worktreePathKey(wt.Path) == worktreePathKey(row.Path) {
						stillLive = true
						break
					}
				}
				if stillLive {
					continue
				}
				ref, err := hasAnyTaskOnWorktree(ctx, tx, row.ID)
				if err != nil {
					return err
				}
				if ref {
					report.WorktreesSkipped = append(report.WorktreesSkipped, ReconcileSkippedWorktree{
						WorktreeID: row.ID,
						Reason:     "has_task_ref",
					})
					continue
				}
				if err := tx.Delete(&model.GitWorktree{}, "id = ?", row.ID).Error; err != nil {
					return err
				}
				report.WorktreesRemoved++
			}
		}

		for _, br := range branches {
			head, err := gitSvc.BranchHead(ctx, opened, br.Name)
			if err != nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(head), strings.TrimSpace(br.HeadSHA)) {
				continue
			}
			if err := tx.Model(&model.GitBranch{}).Where("id = ?", br.ID).
				Update("head_sha", head).Error; err != nil {
				return err
			}
			report.BranchesHeadUpdated++
		}
		return nil
	})
	if err != nil {
		return ReconcileGitOutput{}, err
	}

	status := reconcileStatusOK
	if len(report.WorktreesSkipped) > 0 {
		status = reconcileStatusPartial
	}
	return ReconcileGitOutput{Status: status, Report: report}, nil
}

// RelocateGitRepository runs reconcile with bootstrap path and git worktree repair.
func (s *Store) RelocateGitRepository(
	ctx context.Context,
	projectID, repoID, path string,
	gitSvc gitwork.Service,
) (ReconcileGitOutput, error) {
	return s.ReconcileGitRepository(ctx, projectID, repoID, ReconcileGitInput{
		BootstrapPath: path,
		RepairGit:     true,
		AllowRemove:   true,
	}, gitSvc)
}

// RelocateGitWorktree updates a registered worktree path after verifying it belongs to the repo.
func (s *Store) RelocateGitWorktree(
	ctx context.Context,
	worktreeID, path string,
	gitSvc gitwork.Service,
) (domain.GitWorktree, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.RelocateGitWorktree")
	path = strings.TrimSpace(path)
	if path == "" {
		return domain.GitWorktree{}, fmt.Errorf("%w: path required", domain.ErrInvalidInput)
	}
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}
	wt, err := s.GetGitWorktreeByID(ctx, worktreeID)
	if err != nil {
		return domain.GitWorktree{}, err
	}
	repo, err := s.GetGitRepositoryByID(ctx, wt.RepositoryID)
	if err != nil {
		return domain.GitWorktree{}, err
	}
	opened, _, err := s.openRegisteredRepo(ctx, repo, "", false, gitSvc)
	if err != nil {
		return domain.GitWorktree{}, err
	}
	if opened == nil {
		opened, err = gitSvc.OpenRepository(ctx, path)
		if err != nil {
			return domain.GitWorktree{}, fmt.Errorf("open repository: %w", err)
		}
		branches, err := s.ListGitBranchesByRepo(ctx, repo.ID)
		if err != nil {
			return domain.GitWorktree{}, err
		}
		if err := gitSvc.VerifySameRepository(ctx, registeredCheckoutFromRepo(repo, branches), opened); err != nil {
			return domain.GitWorktree{}, mapGitworkBootstrapErr(err)
		}
	}
	belongs, err := gitSvc.BelongsToRepository(ctx, path, opened.Root)
	if err != nil {
		return domain.GitWorktree{}, fmt.Errorf("belongs to repository: %w", err)
	}
	if !belongs {
		return domain.GitWorktree{}, fmt.Errorf("%w: path is not a linked worktree of this repository", domain.ErrInvalidInput)
	}
	live, err := gitSvc.ListWorktrees(ctx, opened)
	if err != nil {
		return domain.GitWorktree{}, fmt.Errorf("list worktrees: %w", err)
	}
	var found *gitwork.Worktree
	for i := range live {
		if worktreePathKey(live[i].Path) == worktreePathKey(path) {
			found = &live[i]
			break
		}
	}
	if found == nil {
		return domain.GitWorktree{}, fmt.Errorf("%w: path is not a linked worktree of this repository", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(wt.BranchID) != "" {
		br, err := s.GetGitBranchByID(ctx, wt.BranchID)
		if err != nil {
			return domain.GitWorktree{}, err
		}
		if strings.TrimSpace(found.Branch) != "" && found.Branch != br.Name {
			return domain.GitWorktree{}, fmt.Errorf("%w: worktree checkout branch %q does not match bound branch %q",
				domain.ErrInvalidInput, found.Branch, br.Name)
		}
	}
	if err := s.db.WithContext(ctx).Model(&model.GitWorktree{}).Where("id = ?", worktreeID).
		Update("path", found.Path).Error; err != nil {
		return domain.GitWorktree{}, fmt.Errorf("update worktree path: %w", err)
	}
	return s.GetGitWorktreeByID(ctx, worktreeID)
}

//funclogmeasure:skip category=hot-path reason="Bootstrap open helper; operation trace is emitted by ReconcileGitRepository."
func (s *Store) openRepoForReconcile(
	ctx context.Context,
	repo domain.GitRepository,
	bootstrap string,
	gitSvc gitwork.Service,
) (*gitwork.Repository, gitwork.ResolveResult, error) {
	return s.openRegisteredRepo(ctx, repo, bootstrap, false, gitSvc)
}

func (s *Store) openRegisteredRepo(
	ctx context.Context,
	repo domain.GitRepository,
	candidatePath string,
	allowDiscover bool,
	gitSvc gitwork.Service,
) (*gitwork.Repository, gitwork.ResolveResult, error) {
	branches, err := s.ListGitBranchesByRepo(ctx, repo.ID)
	if err != nil {
		return nil, gitwork.ResolveResult{}, err
	}
	result, err := gitSvc.OpenRegisteredCheckout(ctx, gitwork.ResolveInput{
		Registered:    registeredCheckoutFromRepo(repo, branches),
		CandidatePath: candidatePath,
		AllowDiscover: allowDiscover,
	})
	if err != nil {
		if errors.Is(err, gitwork.ErrBootstrapMismatch) {
			return nil, gitwork.ResolveResult{}, domain.NewGitErr(domain.GitCodeBootstrapMismatch, "bootstrap path is not the same repository")
		}
		if errors.Is(err, gitwork.ErrNotARepository) {
			return nil, gitwork.ResolveResult{}, domain.NewGitErr(domain.GitCodeNotARepository, "bootstrap path is not a git repository")
		}
		if errors.Is(err, gitwork.ErrAmbiguousDiscovery) {
			return nil, gitwork.ResolveResult{}, nil
		}
		return nil, gitwork.ResolveResult{}, fmt.Errorf("open registered repository: %w", err)
	}
	if result.Repo == nil {
		return nil, gitwork.ResolveResult{}, nil
	}
	return result.Repo, result, nil
}

//funclogmeasure:skip category=hot-path reason="Pure mapper without I/O; operation trace is emitted by openRegisteredRepo."
func registeredCheckoutFromRepo(repo domain.GitRepository, branches []domain.GitBranch) gitwork.RegisteredCheckout {
	heads := make(map[string]string, len(branches))
	for _, b := range branches {
		name := strings.TrimSpace(b.Name)
		if name == "" {
			continue
		}
		heads[name] = strings.TrimSpace(b.HeadSHA)
	}
	return gitwork.RegisteredCheckout{
		CachedMainPath:  repo.Path,
		CachedCommonDir: repo.GitCommonDir,
		BranchHeads:     heads,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure error mapper; operation trace is emitted by openRegisteredRepo."
func mapGitworkBootstrapErr(err error) error {
	if errors.Is(err, gitwork.ErrBootstrapMismatch) {
		return domain.NewGitErr(domain.GitCodeBootstrapMismatch, "bootstrap path is not the same repository")
	}
	return err
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by ReconcileGitRepository."
func filterLiveWorktrees(live []gitwork.Worktree) []gitwork.Worktree {
	out := make([]gitwork.Worktree, 0, len(live))
	for _, wt := range live {
		if wt.Prunable {
			continue
		}
		out = append(out, wt)
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by ReconcileGitRepository."
func liveWorktreesByPath(live []gitwork.Worktree) map[string]gitwork.Worktree {
	out := make(map[string]gitwork.Worktree, len(live))
	for _, wt := range live {
		out[worktreePathKey(wt.Path)] = wt
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by ReconcileGitRepository."
func liveWorktreesByBranch(live []gitwork.Worktree) map[string]gitwork.Worktree {
	out := make(map[string]gitwork.Worktree, len(live))
	for _, wt := range live {
		if wt.IsMain || strings.TrimSpace(wt.Branch) == "" {
			continue
		}
		out[wt.Branch] = wt
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by ReconcileGitRepository."
func countBranchOwners(rows []model.GitWorktree, branchName string, branchByID map[string]domain.GitBranch) int {
	n := 0
	for _, row := range rows {
		if row.IsMain {
			continue
		}
		br, ok := branchByID[row.BranchID]
		if ok && br.Name == branchName {
			n++
		}
	}
	return n
}

// ReconcileGitRepositoriesOnStartup runs conservative reconcile for repositories whose stored main path exists.
func (s *Store) ReconcileGitRepositoriesOnStartup(ctx context.Context, gitSvc gitwork.Service) {
	repos, err := s.ListGitRepositories(ctx, "")
	if err != nil {
		slog.Warn("git startup reconcile list failed", "cmd", calltrace.LogCmd,
			"operation", "tasks.store.ReconcileGitRepositoriesOnStartup", "err", err)
		return
	}
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}
	for _, repo := range repos {
		path := strings.TrimSpace(repo.Path)
		if path == "" {
			continue
		}
		st, statErr := os.Stat(path)
		if statErr != nil || !st.IsDir() {
			slog.Info("git startup reconcile skip missing path", "cmd", calltrace.LogCmd,
				"operation", "tasks.store.ReconcileGitRepositoriesOnStartup",
				"repository_id", repo.ID, "path", path)
			continue
		}
		out, recErr := s.ReconcileGitRepository(ctx, "", repo.ID, ReconcileGitInput{
			AllowRemove: false,
			RepairGit:   false,
		}, gitSvc)
		if recErr != nil {
			slog.Warn("git startup reconcile failed", "cmd", calltrace.LogCmd,
				"operation", "tasks.store.ReconcileGitRepositoriesOnStartup",
				"repository_id", repo.ID, "err", recErr)
			continue
		}
		slog.Info("git startup reconcile ok", "cmd", calltrace.LogCmd,
			"operation", "tasks.store.ReconcileGitRepositoriesOnStartup",
			"repository_id", repo.ID, "status", out.Status,
			"repo_path_updated", out.Report.RepoPathUpdated,
			"worktrees_path_updated", out.Report.WorktreesPathUpdated,
			"branches_head_updated", out.Report.BranchesHeadUpdated)
	}
}
