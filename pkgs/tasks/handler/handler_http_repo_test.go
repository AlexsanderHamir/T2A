package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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
	srv, wtID, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + repoSearchWithWorktree(wtID, "note"))
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
	resLong, err := http.Get(srv.URL + repoSearchWithWorktree(wtID, longQ))
	if err != nil {
		t.Fatal(err)
	}
	defer resLong.Body.Close()
	if resLong.StatusCode != http.StatusBadRequest {
		t.Fatalf("overlong search q: status %d want %d", resLong.StatusCode, http.StatusBadRequest)
	}

	res2, err := http.Post(srv.URL+"/tasks", "application/json",
		strings.NewReader(withCreateChecklist(fmt.Sprintf(
			`{"title":"t","initial_prompt":"@nope.txt","priority":"medium","worktree_id":%q}`,
			wtID))))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res2.Body)
		t.Fatalf("create status %d body %s", res2.StatusCode, b)
	}
	createBody, _ := io.ReadAll(res2.Body)
	if strings.Contains(string(createBody), "tasks: invalid input") {
		t.Errorf("POST /tasks bad-mention body must not leak ErrInvalidInput prefix; got %s", createBody)
	}
	if !strings.Contains(string(createBody), "mention") || !strings.Contains(string(createBody), "nope.txt") {
		t.Errorf("POST /tasks bad-mention body must still surface the underlying mention reason; got %s", createBody)
	}

	res3, err := http.Post(srv.URL+"/tasks", "application/json",
		strings.NewReader(withCreateChecklist(fmt.Sprintf(
			`{"title":"t2","initial_prompt":"@note.txt(1-2)","priority":"medium","worktree_id":%q}`,
			wtID))))
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
	srv, wtID, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + repoPathWithWorktree(wtID, "note.txt"))
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
	srv, wtID, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	longPath := strings.Repeat("a", maxRepoRelPathQueryBytes+1)
	resFile, err := http.Get(srv.URL + repoPathWithWorktree(wtID, longPath))
	if err != nil {
		t.Fatal(err)
	}
	defer resFile.Body.Close()
	if resFile.StatusCode != http.StatusBadRequest {
		t.Fatalf("repo file overlong path: status %d want %d", resFile.StatusCode, http.StatusBadRequest)
	}

	resVal, err := http.Get(srv.URL + repoValidateRangeWithWorktree(wtID, longPath, "1", "1"))
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
	srv, wtID, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + repoValidateRangeWithWorktree(wtID, "note.txt", "1", "2"))
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
	srv, wtID, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + repoValidateRangeWithWorktree(wtID, "", "", ""))
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
	srv, wtID, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	long := strings.Repeat("1", maxRepoLineQueryParamBytes+1)
	res, err := http.Get(srv.URL + repoValidateRangeWithWorktree(wtID, "note.txt", long, "1"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("validate-range overlong start: status %d want %d", res.StatusCode, http.StatusBadRequest)
	}

	res2, err := http.Get(srv.URL + repoValidateRangeWithWorktree(wtID, "note.txt", "1", long))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusBadRequest {
		t.Fatalf("validate-range overlong end: status %d want %d", res2.StatusCode, http.StatusBadRequest)
	}
}

func TestHTTP_repo_file_resolve_error_doesNotLeakInternalPrefix(t *testing.T) {
	dir := t.TempDir()
	srv, wtID, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + repoPathWithWorktree(wtID, "foo/.."))
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

func TestHTTP_repo_validate_range_resolve_error_doesNotLeakInternalPrefix(t *testing.T) {
	dir := t.TempDir()
	srv, wtID, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + repoValidateRangeWithWorktree(wtID, "foo/..", "1", "1"))
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
	srv, wtID, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + repoValidateRangeWithWorktree(wtID, "a.txt", "nope", "1"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHTTP_repo_routes_require_worktree_id(t *testing.T) {
	dir := t.TempDir()
	srv, _, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/repo/search?q=x")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d want 400", res.StatusCode)
	}
}

func initHTTPTestGitRepo(t *testing.T, dir string) string {
	t.Helper()
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "note.txt")
	run("commit", "-m", "initial")
	return run("rev-parse", "HEAD")
}

func TestHTTP_repo_diff_ok(t *testing.T) {
	dir := t.TempDir()
	sha := initHTTPTestGitRepo(t, dir)
	srv, wtID, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + repoDiffWithWorktree(wtID, sha))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d %s", res.StatusCode, b)
	}
	var payload struct {
		SHA          string `json:"sha"`
		Patch        string `json:"patch"`
		Truncated    bool   `json:"truncated"`
		SizeBytes    int    `json:"size_bytes"`
		Author       string `json:"author"`
		AuthorEmail  string `json:"author_email"`
		FilesChanged int    `json:"files_changed"`
		Insertions   int    `json:"insertions"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.SHA != sha || payload.Truncated || payload.SizeBytes <= 0 {
		t.Fatalf("payload %#v", payload)
	}
	if payload.Author != "Test" || payload.AuthorEmail != "t@example.com" {
		t.Fatalf("author %#v", payload)
	}
	if payload.FilesChanged < 1 || payload.Insertions < 1 {
		t.Fatalf("shortstat %#v", payload)
	}
	if !strings.Contains(payload.Patch, "diff --git") {
		t.Fatalf("patch %#v", payload.Patch)
	}
}

func TestHTTP_repo_diff_not_found(t *testing.T) {
	dir := t.TempDir()
	initHTTPTestGitRepo(t, dir)
	srv, wtID, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + repoDiffWithWorktree(wtID, "deadbeef"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d want 404", res.StatusCode)
	}
}

func TestHTTP_repo_diff_invalid_sha(t *testing.T) {
	dir := t.TempDir()
	initHTTPTestGitRepo(t, dir)
	srv, wtID, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + repoDiffWithWorktree(wtID, "not-a-sha"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d want 400", res.StatusCode)
	}
}

func TestHTTP_repo_diff_missing_sha(t *testing.T) {
	dir := t.TempDir()
	initHTTPTestGitRepo(t, dir)
	srv, wtID, _ := newTaskTestServerWithRepo(t, dir)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/repo/diff?worktree_id=" + wtID)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d want 400", res.StatusCode)
	}
}
