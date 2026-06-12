package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
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

func ensureParentHasCriterion(t *testing.T, st *store.Store, parentID string) {
	t.Helper()
	ctx := context.Background()
	items, err := st.ListChecklistForSubject(ctx, parentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) > 0 {
		return
	}
	if _, err := st.AddChecklistItem(ctx, parentID, "test criterion", domain.ActorUser); err != nil {
		t.Fatal(err)
	}
}

func ensureParentHasCriterionHTTP(t *testing.T, baseURL, parentID string) {
	t.Helper()
	resGet, err := http.Get(baseURL + "/tasks/" + parentID + "/checklist")
	if err != nil {
		t.Fatal(err)
	}
	bodyGet, _ := io.ReadAll(resGet.Body)
	_ = resGet.Body.Close()
	if resGet.StatusCode == http.StatusOK {
		var envelope struct {
			Items []json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(bodyGet, &envelope); err == nil && len(envelope.Items) > 0 {
			return
		}
	}
	res, err := http.Post(
		baseURL+"/tasks/"+parentID+"/checklist/items",
		"application/json",
		strings.NewReader(`{"text":"test criterion"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("seed parent checklist status %d body=%s", res.StatusCode, body)
	}
}

func mustCreateSubtask(t *testing.T, baseURL, parentID string) string {
	t.Helper()
	ensureParentHasCriterionHTTP(t, baseURL, parentID)
	return mustCreateTask(t, baseURL,
		`{"title":"c","priority":"medium","parent_id":"`+parentID+`"}`)
}
