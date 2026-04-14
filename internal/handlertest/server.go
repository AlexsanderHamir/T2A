package handlertest

import (
	"net/http/httptest"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// NewServer returns an httptest.Server wrapping handler.NewHandler with SQLite,
// SSE hub, and no workspace repo.
func NewServer(t *testing.T) *httptest.Server {
	t.Helper()
	db := tasktestdb.OpenSQLite(t)
	h := handler.NewHandler(store.NewStore(db), handler.NewSSEHub(), nil)
	return httptest.NewServer(h)
}

// NewServerWithStore is like [NewServer] but also returns the store for direct DB setup.
func NewServerWithStore(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := handler.NewHandler(st, handler.NewSSEHub(), nil)
	return httptest.NewServer(h), st
}

// NewServerWithRepo is like [NewServer] but mounts a workspace repo rooted at repoDir.
func NewServerWithRepo(t *testing.T, repoDir string) *httptest.Server {
	t.Helper()
	db := tasktestdb.OpenSQLite(t)
	r, err := repo.OpenRoot(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	h := handler.NewHandler(store.NewStore(db), handler.NewSSEHub(), r)
	return httptest.NewServer(h)
}
