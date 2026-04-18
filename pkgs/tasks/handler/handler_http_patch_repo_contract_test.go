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

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// patchRepoTestSetup wires a REPO_ROOT-aware test server, a single seed file
// for valid mentions, and one created task whose id can be patched in subtests.
// Centralized here so each subtest below is just one PATCH + one assertion.
func patchRepoTestSetup(t *testing.T) (srv *httptest.Server, dir, taskID string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("l1\nl2\nl3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	srv = newTaskTestServerWithRepo(t, dir)
	t.Cleanup(srv.Close)

	res, err := http.Post(srv.URL+"/tasks", "application/json",
		strings.NewReader(`{"title":"seed","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create seed status %d body=%s", res.StatusCode, body)
	}
	var created domain.Task
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatal(err)
	}
	return srv, dir, created.ID
}

// TestHTTP_patchTask_repoMentionValidation pins the documented behavior of
// PATCH /tasks/{id} `initial_prompt` validation when REPO_ROOT is configured
// (handler.Handler.patch -> h.repo.ValidatePromptMentions). Session 9 covered
// the non-repo path of PATCH; this is the missing repo-side coverage from the
// queue. Pins six subcases that together describe the full validation
// surface: skip-if-omitted, skip-if-empty, valid mention, unresolved path,
// out-of-range line numbers, and directory-not-file.
func TestHTTP_patchTask_repoMentionValidation(t *testing.T) {
	t.Run("validMention_returns200", func(t *testing.T) {
		srv, _, id := patchRepoTestSetup(t)
		res, raw := patchTask(t, srv.URL, id, `{"initial_prompt":"see @ok.txt(1-2)"}`)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status %d body=%s (valid in-range mention should pass)", res.StatusCode, raw)
		}
	})

	t.Run("emptyInitialPrompt_skipsValidation", func(t *testing.T) {
		// initial_prompt:"" parses to zero mentions in repo.ParseFileMentions,
		// so ValidatePromptMentions returns nil and the patch succeeds. This
		// is the documented "no mentions to validate" path; if a future
		// refactor adds a "non-empty after trim" gate before the repo call,
		// this subtest catches the regression.
		srv, _, id := patchRepoTestSetup(t)
		res, raw := patchTask(t, srv.URL, id, `{"initial_prompt":""}`)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status %d body=%s (empty initial_prompt should bypass mention validation)", res.StatusCode, raw)
		}
	})

	t.Run("noInitialPromptField_skipsValidation", func(t *testing.T) {
		// PATCH that doesn't include initial_prompt at all (e.g. {"title":"x"})
		// must not invoke repo.ValidatePromptMentions. Pins the
		// `body.InitialPrompt != nil` guard in handler_task_crud.go::patch.
		srv, _, id := patchRepoTestSetup(t)
		res, raw := patchTask(t, srv.URL, id, `{"title":"renamed"}`)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status %d body=%s (no initial_prompt field should bypass mention validation)", res.StatusCode, raw)
		}
	})

	t.Run("unresolvedPath_returns400", func(t *testing.T) {
		srv, _, id := patchRepoTestSetup(t)
		res, raw := patchTask(t, srv.URL, id, `{"initial_prompt":"see @nope.txt"}`)
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d body=%s (unresolved mention should reject)", res.StatusCode, raw)
		}
		assertMentionErrorContains(t, raw, "@nope.txt")
	})

	t.Run("outOfRangeLines_returns400", func(t *testing.T) {
		srv, _, id := patchRepoTestSetup(t)
		res, raw := patchTask(t, srv.URL, id, `{"initial_prompt":"see @ok.txt(1-99)"}`)
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d body=%s (out-of-range mention should reject)", res.StatusCode, raw)
		}
		// Range failures preserve the (start-end) suffix so clients can pinpoint
		// the offending mention even when several appear in the same prompt.
		assertMentionErrorContains(t, raw, "@ok.txt")
		assertMentionErrorContains(t, raw, "(1-99)")
	})

	t.Run("mentionToDirectory_returns400", func(t *testing.T) {
		srv, _, id := patchRepoTestSetup(t)
		res, raw := patchTask(t, srv.URL, id, `{"initial_prompt":"see @subdir"}`)
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d body=%s (directory mention should reject)", res.StatusCode, raw)
		}
		assertMentionErrorContains(t, raw, "@subdir")
		// repo.ValidatePromptMentions wraps directory hits with "path is a
		// directory, not a file"; pin the actionable substring so a future
		// rewording stays human-readable.
		assertMentionErrorContains(t, raw, "directory")
	})

	t.Run("pathEscape_returns400", func(t *testing.T) {
		srv, _, id := patchRepoTestSetup(t)
		res, raw := patchTask(t, srv.URL, id, `{"initial_prompt":"see @../escape.txt"}`)
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d body=%s (path escape should reject)", res.StatusCode, raw)
		}
		// The Resolve() rejection messages include "invalid path" or
		// "path escapes repo root"; both share the @<path> prefix in the wrap.
		assertMentionErrorContains(t, raw, "@../escape.txt")
	})
}

// TestHTTP_patchTask_noRepoMentionValidationWhenUnconfigured pins that PATCH
// silently accepts a malformed @mention when REPO_ROOT is *not* configured
// (h.repo == nil). This is the dual of the REPO_ROOT-set path above and
// guards the existing newTaskTestServer wiring (no repo -> no validation).
func TestHTTP_patchTask_noRepoMentionValidationWhenUnconfigured(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	id := mustCreateTask(t, srv.URL, `{"title":"x","priority":"medium"}`)
	res, raw := patchTask(t, srv.URL, id, `{"initial_prompt":"see @nope.txt"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s (no REPO_ROOT -> no mention validation)", res.StatusCode, raw)
	}
}

// assertMentionErrorContains decodes a JSON {"error": "..."} body and fails
// the test if the error message does not contain `substr`. Used by the PATCH
// repo-mention subtests above to pin the structural shape (message includes
// the offending mention path/range) without locking the doubled error-wrap
// prefix that comes from repo.ValidatePromptMentions wrapping a Resolve()
// error that itself wraps domain.ErrInvalidInput.
func assertMentionErrorContains(t *testing.T, raw []byte, substr string) {
	t.Helper()
	var errBody jsonErrorBody
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if !strings.Contains(errBody.Error, substr) {
		t.Fatalf("error=%q missing substring %q (docs/API-HTTP.md says messages include the offending @mention)", errBody.Error, substr)
	}
}
