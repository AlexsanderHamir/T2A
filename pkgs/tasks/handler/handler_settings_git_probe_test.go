package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	if out, err := exec.Command("git", "init", "-b", "main", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	if out, err := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v %s", err, out)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func TestHTTP_gitRepositoryProbe_notARepository(t *testing.T) {
	dir := t.TempDir()

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/settings/git-probe?path=" + url.QueryEscape(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body=%s", res.StatusCode, b)
	}
	var body gitRepositoryProbeResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.IsGitRepository {
		t.Fatalf("expected not a repository, got %+v", body)
	}
	if len(body.Branches) != 0 {
		t.Fatalf("branches=%+v", body.Branches)
	}
}

func TestHTTP_gitRepositoryProbe_listsBranches(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	runGit(t, dir, "branch", "feature")

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/settings/git-probe?path=" + url.QueryEscape(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body=%s", res.StatusCode, b)
	}
	var body gitRepositoryProbeResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.IsGitRepository {
		t.Fatalf("expected git repository, got %+v", body)
	}
	if body.CurrentBranch != "main" {
		t.Fatalf("current_branch=%q want main", body.CurrentBranch)
	}
	if len(body.Branches) < 2 {
		t.Fatalf("branches=%+v", body.Branches)
	}
}
