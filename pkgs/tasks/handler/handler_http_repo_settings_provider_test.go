package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// TestHTTP_repoRoutes_returnsConflictWhenNotConfigured pins the Gate 4
// contract: when the production wiring runs without a configured
// AppSettings.RepoRoot, the /repo/* endpoints must respond
// 409 Conflict with the documented reason "repo_root_not_configured"
// and error string "repo root is not configured" (docs/API-HTTP.md) so
// the SPA can render a "Pick a workspace" banner with a link to
// the Settings page (docs/SETTINGS.md). Reading the same store row
// is exercised across all three /repo/* handlers in one test to make
// the contract regression-proof.
func TestHTTP_repoRoutes_returnsConflictWhenNotConfigured(t *testing.T) {
	const wantRepo409Error = "repo root is not configured" // docs/API-HTTP.md (409 /repo/* when repo_root empty)

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil, WithRepoProvider(NewSettingsRepoProvider(st)))
	srv := httptest.NewServer(h)
	defer srv.Close()

	cases := []string{
		"/repo/search?q=anything",
		"/repo/file?path=note.txt",
		"/repo/validate-range?path=note.txt&start=1&end=2",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			res, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusConflict {
				b, _ := io.ReadAll(res.Body)
				t.Fatalf("status %d want 409 body=%s", res.StatusCode, b)
			}
			var body repoUnavailableErrorBody
			if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Reason != RepoReasonNotConfigured {
				t.Fatalf("reason=%q want %q", body.Reason, RepoReasonNotConfigured)
			}
			if body.Error != wantRepo409Error {
				t.Fatalf("error=%q want %q (see docs/API-HTTP.md repo 409 JSON)", body.Error, wantRepo409Error)
			}
		})
	}
}

// TestHTTP_repoRoutes_followAppSettingsRepoRoot pins that PATCHing
// AppSettings.RepoRoot makes /repo/search start serving from the new
// path on the very next call (no process restart required). This is
// the core promise of NewSettingsRepoProvider; if a regression caches
// the legacy nil root the SPA Settings page would silently break.
func TestHTTP_repoRoutes_followAppSettingsRepoRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings_repo.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil, WithRepoProvider(NewSettingsRepoProvider(st)))
	srv := httptest.NewServer(h)
	defer srv.Close()

	if _, err := st.UpdateSettings(context.Background(), store.SettingsPatch{
		RepoRoot: ptrString(dir),
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	res, err := http.Get(srv.URL + "/repo/search?q=settings_repo")
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

// TestHTTP_createTask_skipsMentionValidationWhenRepoNotConfigured pins
// that POST /tasks accepts an @mention payload even when the active
// RepoProvider returns "not configured". The agent worker enforces
// the same check at run-time once the operator wires a workspace, so
// we don't want to block task drafting from an empty install.
func TestHTTP_createTask_skipsMentionValidationWhenRepoNotConfigured(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil, WithRepoProvider(NewSettingsRepoProvider(st)))
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := http.Post(srv.URL+"/tasks", "application/json",
		strings.NewReader(`{"title":"t","initial_prompt":"@nope.txt","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d want 201 body=%s", res.StatusCode, b)
	}
}

func ptrString(s string) *string { return &s }
