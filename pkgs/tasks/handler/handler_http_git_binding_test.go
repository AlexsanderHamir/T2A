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

func TestHTTP_createTask_branchBoundToWorktree_returns409(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	srv, st, _, _ := newTaskTestServerWithRepoStore(t, dir)
	t.Cleanup(srv.Close)

	ctx := context.Background()
	gitSvc := gitwork.New()
	repos, err := st.ListAllGitRepositories(ctx)
	if err != nil || len(repos) == 0 {
		t.Fatalf("ListAllGitRepositories: %v len=%d", err, len(repos))
	}
	wt2Path := filepath.Join(filepath.Dir(dir), "wt-bound-test")
	_, err = st.CreateGitWorktreeForRepo(ctx, repos[0].ID, store.CreateGitWorktreeInput{
		Path:         wt2Path,
		Branch:       "feature-bound",
		CreateBranch: true,
	}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitWorktreeForRepo first: %v", err)
	}
	wt3Path := filepath.Join(filepath.Dir(dir), "wt-bound-dup")
	_, err = st.CreateGitWorktreeForRepo(ctx, repos[0].ID, store.CreateGitWorktreeInput{
		Path:         wt3Path,
		Branch:       "feature-bound",
		CreateBranch: true,
	}, gitSvc)
	if domain.GitErrCode(err) != domain.GitCodeBranchBoundToWorktree {
		t.Fatalf("CreateGitWorktreeForRepo duplicate branch: got %v want branch_bound_to_worktree", err)
	}
}

func TestHTTP_createTask_projectRepoMismatch_returns409(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	srv, st, worktreeID, _ := newTaskTestServerWithRepoStore(t, dir)
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
		`{"title":"mismatch","priority":"medium","project_id":%q,"worktree_id":%q}`,
		otherProj.ID, worktreeID,
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
