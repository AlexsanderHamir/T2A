package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func createProjectGitRepo(t *testing.T, h http.Handler, main string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"path": main})
	req := httptest.NewRequest(http.MethodPost, "/projects/"+domain.DefaultProjectID+"/git/repositories", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var repo gitRepositoryJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &repo); err != nil {
		t.Fatal(err)
	}
	return repo.ID
}

func TestHandler_projectGitWorktreeRoutes(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createProjectGitRepo(t, h, main)

	listReq := httptest.NewRequest(http.MethodGet, "/projects/"+domain.DefaultProjectID+"/git/repositories/"+repoID+"/worktrees", nil)
	listRec := httptest.NewRecorder()
	h.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	wtPath := filepath.Join(filepath.Dir(main), "wt-proj-http")
	createBody, _ := json.Marshal(gitWorktreeCreateJSON{
		Path:         wtPath,
		Branch:       "proj-http",
		CreateBranch: true,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/projects/"+domain.DefaultProjectID+"/git/repositories/"+repoID+"/worktrees", bytes.NewReader(createBody))
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create wt status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	var wt gitWorktreeJSON
	if err := json.Unmarshal(createRec.Body.Bytes(), &wt); err != nil {
		t.Fatal(err)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/projects/"+domain.DefaultProjectID+"/git/worktrees/"+wt.ID+"?force=true", nil)
	delRec := httptest.NewRecorder()
	h.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete wt status=%d body=%s", delRec.Code, delRec.Body.String())
	}
}

func TestHandler_projectGitBranchRoutes(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createProjectGitRepo(t, h, main)

	listReq := httptest.NewRequest(http.MethodGet, "/projects/"+domain.DefaultProjectID+"/git/repositories/"+repoID+"/branches", nil)
	listRec := httptest.NewRecorder()
	h.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list branches status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	createBody, _ := json.Marshal(gitBranchCreateJSON{Name: "proj-branch", StartPoint: "main"})
	createReq := httptest.NewRequest(http.MethodPost, "/projects/"+domain.DefaultProjectID+"/git/repositories/"+repoID+"/branches", bytes.NewReader(createBody))
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create branch status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	var br gitBranchJSON
	if err := json.Unmarshal(createRec.Body.Bytes(), &br); err != nil {
		t.Fatal(err)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/projects/"+domain.DefaultProjectID+"/git/branches/"+br.ID+"?force=true", nil)
	delRec := httptest.NewRecorder()
	h.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete branch status=%d body=%s", delRec.Code, delRec.Body.String())
	}
}

func TestHandler_projectGitRepositoryGetAndDelete(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createProjectGitRepo(t, h, main)

	getReq := httptest.NewRequest(http.MethodGet, "/projects/"+domain.DefaultProjectID+"/git/repositories/"+repoID, nil)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getRec.Code, getRec.Body.String())
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/projects/"+domain.DefaultProjectID+"/git/repositories/"+repoID, nil)
	delRec := httptest.NewRecorder()
	h.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", delRec.Code, delRec.Body.String())
	}
}

func TestHandler_projectGitReconcile_dryRun(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createProjectGitRepo(t, h, main)
	req := httptest.NewRequest(http.MethodPost, "/projects/"+domain.DefaultProjectID+"/git/repositories/"+repoID+"/reconcile", bytes.NewReader([]byte(`{"dry_run":true}`)))
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

func TestHandler_registerGlobalGitWorktree_missingBranch400(t *testing.T) {
	h, main := gitHandlerTest(t)
	repoID := createGlobalGitRepo(t, h, main)
	wtPath := addHandlerGitWorktree(t, main, "no-branch")
	runHandlerGit(t, wtPath, "checkout", "--detach")
	body, _ := json.Marshal(gitWorktreeRegisterJSON{Path: wtPath, Name: "detached"})
	req := httptest.NewRequest(http.MethodPost, "/git/repositories/"+repoID+"/worktrees/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_createGlobalGitRepository_notGit409(t *testing.T) {
	h, _ := gitHandlerTest(t)
	dir := t.TempDir()
	body, _ := json.Marshal(map[string]string{"path": dir})
	req := httptest.NewRequest(http.MethodPost, "/git/repositories", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var errBody jsonCodedErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Code != domain.GitCodeNotARepository {
		t.Fatalf("code=%q", errBody.Code)
	}
}
