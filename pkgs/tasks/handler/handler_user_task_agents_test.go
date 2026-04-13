package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestUserCreatedTaskEnqueuesForAgents(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	q := agents.NewMemoryQueue(8)
	h := NewHandler(store.NewStore(db), NewSSEHub(), nil, WithUserTaskAgentNotifier(q))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"from-user","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d", res.StatusCode)
	}

	select {
	case got := <-q.Recv():
		q.AckAfterRecv(got.ID)
		if got.Title != "from-user" {
			t.Fatalf("title %q", got.Title)
		}
		if got.Priority != domain.PriorityMedium {
			t.Fatalf("priority %s", got.Priority)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for queued task")
	}
}

func TestAgentActorCreateDoesNotEnqueue(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	q := agents.NewMemoryQueue(8)
	h := NewHandler(store.NewStore(db), NewSSEHub(), nil, WithUserTaskAgentNotifier(q))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/tasks", strings.NewReader(`{"title":"from-agent","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Actor", "agent")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d", res.StatusCode)
	}

	select {
	case got := <-q.Recv():
		t.Fatalf("unexpected queue item %+v", got)
	case <-time.After(150 * time.Millisecond):
	}
}
