package handler

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/gittest"
	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/repo"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// Whitebox handler tests cannot import internal/handlertest (import cycle:
// handlertest → handler). Server wiring mirrors internal/handlertest/server.go.

func newTaskTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	db := tasktestdb.OpenSQLite(t)
	h := NewHandler(store.NewStore(db), NewSSEHub(), nil)
	return httptest.NewServer(h)
}

func newTaskTestServerWithStore(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil)
	return httptest.NewServer(h), st
}

func seedTestGitWorktree(t *testing.T, st *store.Store, repoDir string) (worktreeID, branchID string) {
	t.Helper()
	return gittest.SeedWorktree(t, st, repoDir)
}

func newTaskTestServerWithRepo(t *testing.T, repoDir string) (*httptest.Server, string, string) {
	srv, _, wt, br := newTaskTestServerWithRepoStore(t, repoDir)
	return srv, wt, br
}

func newTaskTestServerWithRepoStore(t *testing.T, repoDir string) (*httptest.Server, *store.Store, string, string) {
	t.Helper()
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	worktreeID, branchID := seedTestGitWorktree(t, st, repoDir)
	r, err := repo.OpenRoot(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(st, NewSSEHub(), r, WithRepoProvider(NewSettingsRepoProvider(st)))
	return httptest.NewServer(h), st, worktreeID, branchID
}

func repoPathWithWorktree(worktreeID, path string) string {
	q := url.Values{}
	q.Set("worktree_id", worktreeID)
	if path != "" {
		q.Set("path", path)
	}
	return "/repo/file?" + q.Encode()
}

func repoSearchWithWorktree(worktreeID, q string) string {
	v := url.Values{}
	v.Set("worktree_id", worktreeID)
	v.Set("q", q)
	return "/repo/search?" + v.Encode()
}

func repoValidateRangeWithWorktree(worktreeID, path, start, end string) string {
	v := url.Values{}
	v.Set("worktree_id", worktreeID)
	v.Set("path", path)
	v.Set("start", start)
	v.Set("end", end)
	return "/repo/validate-range?" + v.Encode()
}

func repoDiffWithWorktree(worktreeID, sha string) string {
	v := url.Values{}
	v.Set("worktree_id", worktreeID)
	v.Set("sha", sha)
	return "/repo/diff?" + v.Encode()
}
