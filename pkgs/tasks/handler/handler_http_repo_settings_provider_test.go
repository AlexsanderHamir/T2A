package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// TestHTTP_repoRoutes_requireWorktreeID pins that /repo/* endpoints require worktree_id.
func TestHTTP_repoRoutes_requireWorktreeID(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil, WithRepoProvider(NewSettingsRepoProvider(st)))
	srv := httptest.NewServer(h)
	defer srv.Close()

	cases := []string{
		"/repo/search?q=anything",
		"/repo/file?path=note.txt",
		"/repo/validate-range?path=note.txt&start=1&end=2",
		"/repo/diff?sha=abc1234",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			res, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusBadRequest {
				b, _ := io.ReadAll(res.Body)
				t.Fatalf("status %d want 400 body=%s", res.StatusCode, b)
			}
		})
	}
}

// TestHTTP_repoRoutes_followRegisteredWorktree pins that a registered git worktree
// makes /repo/search serve from that path on the next call.
func TestHTTP_repoRoutes_followRegisteredWorktree(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings_repo.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	worktreeID, _ := seedTestGitWorktree(t, st, dir)
	h := NewHandler(st, NewSSEHub(), nil, WithRepoProvider(NewSettingsRepoProvider(st)))
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := http.Get(srv.URL + repoSearchWithWorktree(worktreeID, "settings_repo"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d want 200 body=%s", res.StatusCode, b)
	}
	var payload struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Paths) != 1 || !strings.HasSuffix(payload.Paths[0], "settings_repo.txt") {
		t.Fatalf("paths=%#v want [settings_repo.txt]", payload.Paths)
	}
}

// TestHTTP_createTask_skipsMentionValidationWhenNoWorktree pins that POST /tasks
// accepts a prompt without @-mentions even when no git worktree is registered.
func TestHTTP_createTask_skipsMentionValidationWhenNoWorktree(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil, WithRepoProvider(NewSettingsRepoProvider(st)))
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json",
		strings.NewReader(withCreateChecklist(`{"title":"t","initial_prompt":"plain prompt","priority":"medium"}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d want 201 body=%s", res.StatusCode, b)
	}
}

func TestHTTP_createTask_mentionRequiresWorktreeID(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil, WithRepoProvider(NewSettingsRepoProvider(st)))
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json",
		strings.NewReader(withCreateChecklist(`{"title":"t","initial_prompt":"@nope.txt","priority":"medium"}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d want 400 body=%s", res.StatusCode, b)
	}
}

func ptrString(s string) *string { return &s }

func TestHTTP_repoRoutes_unknownWorktree_returns404(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil, WithRepoProvider(NewSettingsRepoProvider(st)))
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := http.Get(srv.URL + repoSearchWithWorktree("00000000-0000-0000-0000-000000000099", "x"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d want 404 body=%s", res.StatusCode, b)
	}
	var body repoUnavailableErrorBody
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Reason != RepoReasonWorktreeNotFound {
		t.Fatalf("reason=%q", body.Reason)
	}
}
