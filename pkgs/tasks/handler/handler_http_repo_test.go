package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTTP_repo_search_and_create_rejects_bad_file_mention(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(p, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/repo/search?q=note")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("search status %d", res.StatusCode)
	}
	var searchPayload struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(res.Body).Decode(&searchPayload); err != nil {
		t.Fatal(err)
	}
	if len(searchPayload.Paths) != 1 || searchPayload.Paths[0] != "note.txt" {
		t.Fatalf("paths %#v", searchPayload.Paths)
	}

	longQ := strings.Repeat("a", maxRepoSearchQueryBytes+1)
	resLong, err := http.Get(srv.URL + "/repo/search?q=" + longQ)
	if err != nil {
		t.Fatal(err)
	}
	defer resLong.Body.Close()
	if resLong.StatusCode != http.StatusBadRequest {
		t.Fatalf("overlong search q: status %d want %d", resLong.StatusCode, http.StatusBadRequest)
	}

	res2, err := http.Post(srv.URL+"/tasks", "application/json",
		strings.NewReader(`{"title":"t","initial_prompt":"@nope.txt","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res2.Body)
		t.Fatalf("create status %d body %s", res2.StatusCode, b)
	}
	// The validatePromptMentionsIfRepo path returns errors wrapped with
	// domain.ErrInvalidInput (via repo.wrapMention). Without prefix-aware
	// error rendering the wire body echoed the internal "tasks: invalid
	// input: " marker into the SPA banner. Pin the clean phrasing here so
	// we cannot regress to the raw wrap on POST /tasks.
	createBody, _ := io.ReadAll(res2.Body)
	if strings.Contains(string(createBody), "tasks: invalid input") {
		t.Errorf("POST /tasks bad-mention body must not leak ErrInvalidInput prefix; got %s", createBody)
	}
	if !strings.Contains(string(createBody), "mention") || !strings.Contains(string(createBody), "nope.txt") {
		t.Errorf("POST /tasks bad-mention body must still surface the underlying mention reason; got %s", createBody)
	}

	res3, err := http.Post(srv.URL+"/tasks", "application/json",
		strings.NewReader(`{"title":"t2","initial_prompt":"@note.txt(1-2)","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res3.Body.Close()
	if res3.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res3.Body)
		t.Fatalf("create valid mention status %d body %s", res3.StatusCode, b)
	}
}

func TestHTTP_repo_file_ok(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(p, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/repo/file?path=note.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d %s", res.StatusCode, b)
	}
	var payload struct {
		Path      string `json:"path"`
		Content   string `json:"content"`
		Binary    bool   `json:"binary"`
		Truncated bool   `json:"truncated"`
		LineCount int    `json:"line_count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Path != "note.txt" || payload.Binary || payload.Truncated {
		t.Fatalf("payload %#v", payload)
	}
	if payload.Content != "line1\nline2\n" || payload.LineCount != 2 {
		t.Fatalf("content/line_count %#v", payload)
	}
}

func TestHTTP_repo_file_and_validate_range_reject_overlong_path(t *testing.T) {
	dir := t.TempDir()
	srv := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	longPath := strings.Repeat("a", maxRepoRelPathQueryBytes+1)
	resFile, err := http.Get(srv.URL + "/repo/file?path=" + longPath)
	if err != nil {
		t.Fatal(err)
	}
	defer resFile.Body.Close()
	if resFile.StatusCode != http.StatusBadRequest {
		t.Fatalf("repo file overlong path: status %d want %d", resFile.StatusCode, http.StatusBadRequest)
	}

	resVal, err := http.Get(srv.URL + "/repo/validate-range?path=" + longPath + "&start=1&end=1")
	if err != nil {
		t.Fatal(err)
	}
	defer resVal.Body.Close()
	if resVal.StatusCode != http.StatusBadRequest {
		t.Fatalf("validate-range overlong path: status %d want %d", resVal.StatusCode, http.StatusBadRequest)
	}
}

func TestHTTP_repo_validate_range_ok(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(p, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/repo/validate-range?path=note.txt&start=1&end=2")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var payload struct {
		OK        bool `json:"ok"`
		LineCount int  `json:"line_count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.OK || payload.LineCount != 3 {
		t.Fatalf("ok=%v line_count=%d", payload.OK, payload.LineCount)
	}
}

func TestHTTP_repo_validate_range_missing_params(t *testing.T) {
	dir := t.TempDir()
	srv := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/repo/validate-range")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_repo_validate_range_reject_overlong_start_end(t *testing.T) {
	dir := t.TempDir()
	srv := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	long := strings.Repeat("1", maxRepoLineQueryParamBytes+1)
	res, err := http.Get(srv.URL + "/repo/validate-range?path=note.txt&start=" + long + "&end=1")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("validate-range overlong start: status %d want %d", res.StatusCode, http.StatusBadRequest)
	}

	res2, err := http.Get(srv.URL + "/repo/validate-range?path=note.txt&start=1&end=" + long)
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusBadRequest {
		t.Fatalf("validate-range overlong end: status %d want %d", res2.StatusCode, http.StatusBadRequest)
	}
}

// TestHTTP_repo_file_resolve_error_doesNotLeakInternalPrefix pins the wire
// shape for repo /file Resolve failures so the SPA never has to render the
// raw "tasks: invalid input: " prefix that domain.ErrInvalidInput stamps on
// every Resolve / ValidateRange / LineCount return path. Without the
// repoErrUserMessage helper, the GET /repo/file path-segment-traversal
// reject was returning {"error":"tasks: invalid input: invalid path", ...}
// — the same body the SPA echoes verbatim into the @-mention sidebar tooltip,
// which then surfaced an internal-looking phrase to end users that other
// handlers had already stripped via storeErrorClientMessage. The asymmetry
// (only repoValidateRange's ValidateRange branch trimmed the prefix) made
// the leak hard to spot through black-box testing — both /repo/file and
// /repo/validate-range's Resolve branch leaked it, but only one half of
// the latter was covered.
func TestHTTP_repo_file_resolve_error_doesNotLeakInternalPrefix(t *testing.T) {
	dir := t.TempDir()
	srv := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/repo/file?path=foo/..")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body.Error, "tasks: invalid input") {
		t.Errorf("error message must not leak internal ErrInvalidInput prefix; got %q", body.Error)
	}
	if !strings.Contains(body.Error, "invalid path") {
		t.Errorf("error message must still surface the underlying reason; got %q", body.Error)
	}
}

// TestHTTP_repo_validate_range_resolve_error_doesNotLeakInternalPrefix is
// the sibling test for repoValidateRange's Resolve branch (the warning
// field on the 200-with-OK-false response). The validate-range handler
// already stripped the prefix on its ValidateRange branch (see line
// ~165 of repo_handlers.go), but the earlier Resolve branch (line ~149)
// fell through with the raw err.Error() until repoErrUserMessage
// centralized the strip.
func TestHTTP_repo_validate_range_resolve_error_doesNotLeakInternalPrefix(t *testing.T) {
	dir := t.TempDir()
	srv := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/repo/validate-range?path=foo/..&start=1&end=1")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	var payload struct {
		OK      bool   `json:"ok"`
		Warning string `json:"warning"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.OK {
		t.Fatalf("expected ok=false for traversal reject; got %#v", payload)
	}
	if strings.Contains(payload.Warning, "tasks: invalid input") {
		t.Errorf("warning must not leak internal ErrInvalidInput prefix; got %q", payload.Warning)
	}
	if !strings.Contains(payload.Warning, "invalid path") {
		t.Errorf("warning must still surface the underlying reason; got %q", payload.Warning)
	}
}

func TestHTTP_repo_validate_range_invalid_start_end(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/repo/validate-range?path=a.txt&start=nope&end=1")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}
