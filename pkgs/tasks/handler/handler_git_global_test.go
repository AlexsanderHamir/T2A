package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func createGlobalGitRepo(t *testing.T, h http.Handler, main string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"path": main})
	req := httptest.NewRequest(http.MethodPost, "/git/repositories", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create repo status=%d body=%s", rec.Code, rec.Body.String())
	}
	var repo gitRepositoryJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &repo); err != nil {
		t.Fatal(err)
	}
	if repo.ID == "" {
		t.Fatal("empty repo id")
	}
	return repo.ID
}

func addHandlerGitWorktree(t *testing.T, main, branch string) string {
	t.Helper()
	wtPath := filepath.Join(filepath.Dir(main), "wt-"+branch)
	runHandlerGit(t, main, "worktree", "add", wtPath, "-b", branch)
	t.Cleanup(func() { _ = os.RemoveAll(wtPath) })
	return wtPath
}

func TestHandler_getAndDeleteGlobalGitRepository(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)

	getReq := httptest.NewRequest(http.MethodGet, "/git/repositories/"+repoID, nil)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getRec.Code, getRec.Body.String())
	}
	var repo gitRepositoryJSON
	if err := json.Unmarshal(getRec.Body.Bytes(), &repo); err != nil {
		t.Fatal(err)
	}
	if repo.Path == "" {
		t.Fatal("empty path")
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/git/repositories/"+repoID, nil)
	delRec := httptest.NewRecorder()
	h.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", delRec.Code, delRec.Body.String())
	}
}

func TestHandler_createGlobalGitWorktree(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)
	wtPath := filepath.Join(filepath.Dir(main), "wt-create-http")
	body, _ := json.Marshal(gitWorktreeCreateJSON{
		Path:         wtPath,
		Name:         "created",
		Branch:       "create-http",
		CreateBranch: true,
	})
	req := httptest.NewRequest(http.MethodPost, "/git/repositories/"+repoID+"/worktrees", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var wt gitWorktreeJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &wt); err != nil {
		t.Fatal(err)
	}
	if wt.BranchID == "" {
		t.Fatal("expected branch_id")
	}
}

func TestHandler_listGlobalGitBranchesAndLive(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)

	listReq := httptest.NewRequest(http.MethodGet, "/git/repositories/"+repoID+"/branches", nil)
	listRec := httptest.NewRecorder()
	h.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("branches status=%d body=%s", listRec.Code, listRec.Body.String())
	}
	var branches gitBranchesListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &branches); err != nil {
		t.Fatal(err)
	}
	if len(branches.Branches) < 1 {
		t.Fatal("expected at least main branch")
	}

	liveReq := httptest.NewRequest(http.MethodGet, "/git/repositories/"+repoID+"/branches/live", nil)
	liveRec := httptest.NewRecorder()
	h.ServeHTTP(liveRec, liveReq)
	if liveRec.Code != http.StatusOK {
		t.Fatalf("live branches status=%d body=%s", liveRec.Code, liveRec.Body.String())
	}
	var live gitLiveBranchesListResponse
	if err := json.Unmarshal(liveRec.Body.Bytes(), &live); err != nil {
		t.Fatal(err)
	}
	if len(live.Branches) < 1 {
		t.Fatal("expected live branches")
	}
}

func TestHandler_listRepoProjects(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)
	req := httptest.NewRequest(http.MethodGet, "/git/repositories/"+repoID+"/projects", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("projects status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_relocateGlobalGitWorktree(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)
	wtPath := addHandlerGitWorktree(t, main, "relocate-wt")
	regBody, _ := json.Marshal(gitWorktreeRegisterJSON{
		Path: wtPath,
		Name: "relocate",
		Branch: &gitWorktreeBranchBindJSON{
			Name: "relocate-wt",
		},
	})
	regReq := httptest.NewRequest(http.MethodPost, "/git/repositories/"+repoID+"/worktrees/register", bytes.NewReader(regBody))
	regRec := httptest.NewRecorder()
	h.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", regRec.Code, regRec.Body.String())
	}
	var wt gitWorktreeJSON
	if err := json.Unmarshal(regRec.Body.Bytes(), &wt); err != nil {
		t.Fatal(err)
	}
	movedPath := filepath.Join(filepath.Dir(main), "relocate-wt-moved")
	runHandlerGit(t, main, "worktree", "move", wtPath, movedPath)
	t.Cleanup(func() { _ = os.RemoveAll(movedPath) })

	body, _ := json.Marshal(map[string]string{"path": movedPath})
	req := httptest.NewRequest(http.MethodPost, "/git/worktrees/"+wt.ID+"/relocate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("relocate status=%d body=%s", rec.Code, rec.Body.String())
	}
	var updated gitWorktreeJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if worktreePathKeyHandler(updated.Path) != worktreePathKeyHandler(movedPath) {
		t.Fatalf("path=%q want %q", updated.Path, movedPath)
	}
}

func TestHandler_listGlobalGitRepositories_afterCreate(t *testing.T) {
	h, main := gitHandlerTest(t)
	createGlobalGitRepo(t, h, main)
	req := httptest.NewRequest(http.MethodGet, "/git/repositories", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp gitRepositoriesListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Repositories) != 1 {
		t.Fatalf("len=%d want 1", len(resp.Repositories))
	}
}

func TestHandler_listGlobalGitWorktrees_serializesBranchID(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)

	req := httptest.NewRequest(http.MethodGet, "/git/repositories/"+repoID+"/worktrees", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp gitWorktreesListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Worktrees) != 1 {
		t.Fatalf("len=%d want 1 main worktree", len(resp.Worktrees))
	}
	if resp.Worktrees[0].BranchID == "" {
		t.Fatal("main worktree must have branch_id after global register")
	}
}

func TestHandler_gitErrHTTP_domainSentinels(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"not found", domain.ErrNotFound, http.StatusNotFound},
		{"invalid input", domain.ErrInvalidInput, http.StatusBadRequest},
		{"conflict", domain.ErrConflict, http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, _ := gitErrHTTP(tt.err)
			if status != tt.status || code != "" {
				t.Fatalf("status=%d code=%q want status=%d code=\"\"", status, code, tt.status)
			}
		})
	}
}

func TestHandler_listGlobalGitWorktreesLive_registeredFlag(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)
	wtPath := addHandlerGitWorktree(t, main, "live-reg")

	req := httptest.NewRequest(http.MethodGet, "/git/repositories/"+repoID+"/worktrees/live", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("live status=%d body=%s", rec.Code, rec.Body.String())
	}
	var live gitLiveWorktreesListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &live); err != nil {
		t.Fatal(err)
	}
	var mainRow, linkedRow *gitLiveWorktreeJSON
	for i := range live.Worktrees {
		switch {
		case live.Worktrees[i].IsMain:
			mainRow = &live.Worktrees[i]
		case worktreePathKeyHandler(live.Worktrees[i].Path) == worktreePathKeyHandler(wtPath):
			linkedRow = &live.Worktrees[i]
		}
	}
	if mainRow == nil || !mainRow.Registered {
		t.Fatalf("main must be registered: %+v", live.Worktrees)
	}
	if linkedRow == nil || linkedRow.Registered {
		t.Fatalf("unregistered linked worktree: %+v", live.Worktrees)
	}
}

func TestHandler_probeGlobalGitWorktree(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)
	wtPath := addHandlerGitWorktree(t, main, "probe-http")
	foreign := initHandlerGitRepo(t)

	probe := func(path string) gitWorktreeProbeResponse {
		t.Helper()
		q := url.Values{"path": {path}}
		req := httptest.NewRequest(http.MethodGet, "/git/repositories/"+repoID+"/worktrees/probe?"+q.Encode(), nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("probe %s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
		var out gitWorktreeProbeResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	linked := probe(wtPath)
	if !linked.Linked || linked.Registered {
		t.Fatalf("linked unregistered: %+v", linked)
	}
	mainProbe := probe(main)
	if !mainProbe.Linked || !mainProbe.Registered || !mainProbe.IsMain {
		t.Fatalf("main: %+v", mainProbe)
	}
	unlinked := probe(foreign)
	if unlinked.Linked {
		t.Fatalf("foreign: %+v", unlinked)
	}
}

func TestHandler_registerGlobalGitWorktree(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)
	wtPath := addHandlerGitWorktree(t, main, "register-http")

	body, _ := json.Marshal(gitWorktreeRegisterJSON{
		Path: wtPath,
		Name: "feature",
		Branch: &gitWorktreeBranchBindJSON{
			Name: "register-http",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/git/repositories/"+repoID+"/worktrees/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", rec.Code, rec.Body.String())
	}
	var wt gitWorktreeJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &wt); err != nil {
		t.Fatal(err)
	}
	if wt.BranchID == "" || wt.Name != "feature" {
		t.Fatalf("worktree=%+v", wt)
	}
}

func TestHandler_registerGlobalGitWorktree_duplicatePath409(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)
	wtPath := addHandlerGitWorktree(t, main, "dup-http")
	registerBody, _ := json.Marshal(gitWorktreeRegisterJSON{
		Path: wtPath,
		Name: "first",
		Branch: &gitWorktreeBranchBindJSON{
			Name: "dup-http",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/git/repositories/"+repoID+"/worktrees/register", bytes.NewReader(registerBody))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first register status=%d body=%s", rec.Code, rec.Body.String())
	}
	dupReq := httptest.NewRequest(http.MethodPost, "/git/repositories/"+repoID+"/worktrees/register", bytes.NewReader(registerBody))
	dupRec := httptest.NewRecorder()
	h.ServeHTTP(dupRec, dupReq)
	if dupRec.Code != http.StatusConflict {
		t.Fatalf("duplicate status=%d body=%s", dupRec.Code, dupRec.Body.String())
	}
	var errBody jsonCodedErrorBody
	if err := json.Unmarshal(dupRec.Body.Bytes(), &errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Code != domain.GitCodePathExists {
		t.Fatalf("code=%q want path_exists", errBody.Code)
	}
}

func TestHandler_reconcileGlobalGitRepository(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)
	renamed := filepath.Join(filepath.Dir(main), "reconcile-gone")
	if err := os.Rename(main, renamed); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(renamed); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/git/repositories/"+repoID+"/reconcile", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("reconcile status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out gitReconcileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "needs_bootstrap_path" {
		t.Fatalf("status=%q want needs_bootstrap_path", out.Status)
	}
}

func TestHandler_reconcileGlobalGitRepository_dryRunOK(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)
	req := httptest.NewRequest(http.MethodPost, "/git/repositories/"+repoID+"/reconcile", strings.NewReader(`{"dry_run":true}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("reconcile status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out gitReconcileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "ok" {
		t.Fatalf("status=%q want ok", out.Status)
	}
}

func TestHandler_relocateGlobalGitRepository(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)
	renamed := filepath.Join(filepath.Dir(main), "relocate-http")
	if err := os.Rename(main, renamed); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(renamed, main) })

	body, _ := json.Marshal(map[string]string{"path": renamed})
	req := httptest.NewRequest(http.MethodPost, "/git/repositories/"+repoID+"/relocate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("relocate status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out gitReconcileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "ok" || !out.Report.RepoPathUpdated {
		t.Fatalf("relocate response=%+v", out)
	}
}

func TestHandler_deleteGlobalGitWorktree(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)
	wtPath := addHandlerGitWorktree(t, main, "delete-http")
	regBody, _ := json.Marshal(gitWorktreeRegisterJSON{
		Path: wtPath,
		Name: "delete-me",
		Branch: &gitWorktreeBranchBindJSON{
			Name: "delete-http",
		},
	})
	regReq := httptest.NewRequest(http.MethodPost, "/git/repositories/"+repoID+"/worktrees/register", bytes.NewReader(regBody))
	regRec := httptest.NewRecorder()
	h.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", regRec.Code, regRec.Body.String())
	}
	var wt gitWorktreeJSON
	if err := json.Unmarshal(regRec.Body.Bytes(), &wt); err != nil {
		t.Fatal(err)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/git/worktrees/"+wt.ID+"?force=true", nil)
	delRec := httptest.NewRecorder()
	h.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", delRec.Code, delRec.Body.String())
	}
}

func worktreePathKeyHandler(path string) string {
	return strings.ToLower(strings.TrimSuffix(strings.ReplaceAll(filepath.Clean(path), `\`, `/`), `/`))
}

func TestHandler_gitStoreErrorsReturnStableCode(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{
			name:   "repository not found",
			err:    domain.NewGitErr(domain.GitCodeRepositoryNotFound, "repository not found"),
			status: http.StatusNotFound,
			code:   domain.GitCodeRepositoryNotFound,
		},
		{
			name:   "path exists",
			err:    domain.NewGitErr(domain.GitCodePathExists, "worktree path already registered"),
			status: http.StatusConflict,
			code:   domain.GitCodePathExists,
		},
		{
			name:   "bootstrap mismatch",
			err:    domain.NewGitErr(domain.GitCodeBootstrapMismatch, "bootstrap mismatch"),
			status: http.StatusConflict,
			code:   domain.GitCodeBootstrapMismatch,
		},
		{
			name:   "has running task",
			err:    domain.NewGitErr(domain.GitCodeHasRunningTask, "has running task"),
			status: http.StatusConflict,
			code:   domain.GitCodeHasRunningTask,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, msg := gitErrHTTP(tt.err)
			if status != tt.status || code != tt.code || msg == "" {
				t.Fatalf("status=%d code=%q msg=%q", status, code, msg)
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			writeGitStoreError(rec, req, "test.op", tt.err)
			if rec.Code != tt.status {
				t.Fatalf("write status=%d want %d body=%s", rec.Code, tt.status, rec.Body.String())
			}
			var body jsonCodedErrorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Code != tt.code {
				t.Fatalf("body code=%q want %q", body.Code, tt.code)
			}
		})
	}
}

func TestHandler_probeGlobalGitWorktree_missingPath400(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)
	req := httptest.NewRequest(http.MethodGet, "/git/repositories/"+repoID+"/worktrees/probe", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
