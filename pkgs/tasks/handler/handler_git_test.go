package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func gitHandlerTest(t *testing.T) (http.Handler, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	main := initHandlerGitRepo(t)
	h := NewHandler(st, NewSSEHub(), nil)
	return h, main
}

func initHandlerGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runHandlerGit(t, dir, "init", "-b", "main")
	runHandlerGit(t, dir, "config", "user.email", "t@example.com")
	runHandlerGit(t, dir, "config", "user.name", "Test")
	runHandlerGit(t, dir, "commit", "--allow-empty", "-m", "init")
	return dir
}

func runHandlerGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	all := append([]string{"-C", dir}, args...)
	if out, err := exec.Command("git", all...).CombinedOutput(); err != nil {
		t.Fatalf("git: %v\n%s", err, out)
	}
}

func TestHandler_createGitRepository(t *testing.T) {
	h, main := gitHandlerTest(t)
	body, _ := json.Marshal(map[string]string{"path": main})
	req := httptest.NewRequest(http.MethodPost, "/projects/"+domain.DefaultProjectID+"/git/repositories", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_createGitRepository_notGit(t *testing.T) {
	h, _ := gitHandlerTest(t)
	dir := t.TempDir()
	body, _ := json.Marshal(map[string]string{"path": dir})
	req := httptest.NewRequest(http.MethodPost, "/projects/"+domain.DefaultProjectID+"/git/repositories", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409 body=%s", rec.Code, rec.Body.String())
	}
	var resp jsonCodedErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != domain.GitCodeNotARepository {
		t.Fatalf("code=%q", resp.Code)
	}
}

func TestHandler_listGitRepositories(t *testing.T) {
	h, main := gitHandlerTest(t)
	createBody, _ := json.Marshal(map[string]string{"path": main})
	create := httptest.NewRequest(http.MethodPost, "/projects/"+domain.DefaultProjectID+"/git/repositories", bytes.NewReader(createBody))
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, create)
	if createRec.Code != http.StatusCreated {
		t.Fatal(createRec.Body.String())
	}
	list := httptest.NewRequest(http.MethodGet, "/projects/"+domain.DefaultProjectID+"/git/repositories", nil)
	listRec := httptest.NewRecorder()
	h.ServeHTTP(listRec, list)
	if listRec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", listRec.Code, listRec.Body.String())
	}
}
