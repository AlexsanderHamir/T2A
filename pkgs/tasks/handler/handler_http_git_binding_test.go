package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/gittest"
	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func TestHTTP_createTask_branchActiveElsewhere_returns409(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	srv, st, worktreeID, branchID, _ := newTaskTestServerWithRepoStore(t, dir)
	t.Cleanup(srv.Close)

	ctx := context.Background()
	if err := st.SetActiveBranch(ctx, worktreeID, branchID); err != nil {
		t.Fatalf("SetActiveBranch: %v", err)
	}

	wt2Path := filepath.Join(filepath.Dir(dir), "wt-active-test")
	gitSvc := gitwork.New()
	repos, err := st.ListAllGitRepositories(ctx)
	if err != nil || len(repos) == 0 {
		t.Fatalf("ListAllGitRepositories: %v len=%d", err, len(repos))
	}
	wt2, err := st.CreateGitWorktreeForRepo(ctx, repos[0].ID, store.CreateGitWorktreeInput{
		Path:         wt2Path,
		Branch:       "feature-side",
		CreateBranch: true,
	}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitWorktreeForRepo: %v", err)
	}
	// Same branch checked out actively in worktreeID — binding it on wt2 must reject.
	wb2, err := st.AssociateWorktreeBranch(ctx, store.AssociateWorktreeBranchInput{
		WorktreeID: wt2.ID,
		BranchID:   branchID,
	})
	if err != nil {
		t.Fatalf("AssociateWorktreeBranch: %v", err)
	}

	body := fmt.Sprintf(`{"title":"blocked","priority":"medium","worktree_branch_id":%q}`, wb2.ID)
	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status %d body=%s want 409 branch_active_elsewhere", res.StatusCode, raw)
	}
	var errBody jsonCodedErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if errBody.Code != domain.GitCodeBranchActiveElsewhere {
		t.Fatalf("code=%q want %q", errBody.Code, domain.GitCodeBranchActiveElsewhere)
	}
}

func TestHTTP_createTask_projectRepoMismatch_returns409(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	srv, st, _, _, wbID := newTaskTestServerWithRepoStore(t, dir)
	t.Cleanup(srv.Close)

	ctx := context.Background()
	gitSvc := gitwork.New()
	otherDir := t.TempDir()
	gittest.EnsureMain(t, otherDir)
	otherRepo, err := st.CreateGlobalGitRepository(ctx, store.CreateGitRepositoryInput{Path: otherDir}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGlobalGitRepository: %v", err)
	}
	otherRepoID := otherRepo.ID
	otherProj, err := st.CreateProject(ctx, store.CreateProjectInput{
		Name:         "wrong-repo overlay",
		RepositoryID: &otherRepoID,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	body := fmt.Sprintf(
		`{"title":"mismatch","priority":"medium","project_id":%q,"worktree_branch_id":%q}`,
		otherProj.ID, wbID,
	)
	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status %d body=%s want 409 project_repo_mismatch", res.StatusCode, raw)
	}
	var errBody jsonCodedErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if errBody.Code != domain.GitCodeProjectRepoMismatch {
		t.Fatalf("code=%q want %q", errBody.Code, domain.GitCodeProjectRepoMismatch)
	}
}
