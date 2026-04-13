package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

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

func newTaskTestServerWithRepo(t *testing.T, repoDir string) *httptest.Server {
	t.Helper()
	db := tasktestdb.OpenSQLite(t)
	r, err := repo.OpenRoot(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(store.NewStore(db), NewSSEHub(), r)
	return httptest.NewServer(h)
}
