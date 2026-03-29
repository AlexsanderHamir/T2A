package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/internal/testdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestSSETestRoutes_POST_publish(t *testing.T) {
	db := testdb.OpenSQLite(t)
	st := store.NewStore(db)
	ctx := context.Background()
	created, err := st.Create(ctx, store.CreateTaskInput{Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	hub := NewSSEHub()
	mux := http.NewServeMux()
	RegisterSSETestRoutes(mux, st, hub, true)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ch, cancel := hub.Subscribe()
	defer cancel()

	body := fmt.Sprintf(`{"type":"task_updated","id":%q}`, created.ID)
	res, err := http.Post(srv.URL+"/dev/sse/publish", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d", res.StatusCode)
	}

	select {
	case line := <-ch:
		var ev TaskChangeEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type != TaskUpdated || ev.ID != created.ID {
			t.Fatalf("got %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for hub event")
	}
}

func TestSSETestRoutes_GET_ping(t *testing.T) {
	db := testdb.OpenSQLite(t)
	st := store.NewStore(db)
	ctx := context.Background()
	if _, err := st.Create(ctx, store.CreateTaskInput{Title: "p"}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	hub := NewSSEHub()
	mux := http.NewServeMux()
	RegisterSSETestRoutes(mux, st, hub, true)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ch, cancel := hub.Subscribe()
	defer cancel()

	res, err := http.Get(srv.URL + "/dev/sse/ping")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d", res.StatusCode)
	}

	select {
	case line := <-ch:
		var ev TaskChangeEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type != TaskUpdated || ev.ID == "" {
			t.Fatalf("got %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for hub event")
	}
}

func TestSSETestRoutes_POST_publish_invalidType(t *testing.T) {
	db := testdb.OpenSQLite(t)
	st := store.NewStore(db)
	hub := NewSSEHub()
	mux := http.NewServeMux()
	RegisterSSETestRoutes(mux, st, hub, true)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res, err := http.Post(srv.URL+"/dev/sse/publish", "application/json", strings.NewReader(`{"type":"nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}
