package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// newSSETriggerServer wires a real handler against an in-memory SQLite store and
// returns the SSE hub so the test can subscribe and observe published events
// produced by HTTP writes. It mirrors newTaskTestServerWithStore but exposes
// the hub for assertion.
func newSSETriggerServer(t *testing.T) (*httptest.Server, *store.Store, *SSEHub) {
	t.Helper()
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	hub := NewSSEHub()
	h := NewHandler(st, hub, nil)
	return httptest.NewServer(h), st, hub
}

// drainSSE collects up to want events from ch, returning whatever arrived
// within timeout. Returning early when len(out) == want keeps tests fast.
// The caller should assert on the slice rather than block forever.
func drainSSE(t *testing.T, ch <-chan string, want int, timeout time.Duration) []TaskChangeEvent {
	t.Helper()
	out := make([]TaskChangeEvent, 0, want)
	deadline := time.After(timeout)
	for len(out) < want {
		select {
		case s, ok := <-ch:
			if !ok {
				return out
			}
			var ev TaskChangeEvent
			if err := json.Unmarshal([]byte(s), &ev); err != nil {
				t.Fatalf("decode sse line %q: %v", s, err)
			}
			out = append(out, ev)
		case <-deadline:
			return out
		}
	}
	// Quick grace-window read to surface unexpected extras the test wasn't expecting.
	select {
	case s := <-ch:
		var ev TaskChangeEvent
		if err := json.Unmarshal([]byte(s), &ev); err == nil {
			out = append(out, ev)
		}
	case <-time.After(50 * time.Millisecond):
	}
	return out
}

// summarize collapses a TaskChangeEvent slice into a stable string set so
// tests can compare published events without relying on publish order. The
// format is "type:id" for task-only events and "type:id/cycle_id" for
// task_cycle_changed events so the cycle identity is asserted explicitly.
func summarize(events []TaskChangeEvent) []string {
	out := make([]string, 0, len(events))
	for _, ev := range events {
		if ev.CycleID != "" {
			out = append(out, fmt.Sprintf("%s:%s/%s", ev.Type, ev.ID, ev.CycleID))
			continue
		}
		out = append(out, fmt.Sprintf("%s:%s", ev.Type, ev.ID))
	}
	sort.Strings(out)
	return out
}

func mustEqualEvents(t *testing.T, route string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d events %v, want %d %v (docs/API-SSE.md trigger table)",
			route, len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s: event[%d]=%q want %q (full got=%v want=%v)", route, i, got[i], want[i], got, want)
		}
	}
}

// TestHTTP_SSE_triggerSurface pins the SSE trigger table documented in
// docs/API-SSE.md. Each subtest exercises one HTTP write and asserts the exact
// set of {type,id} events published on the hub. If a future change adds or
// removes a publish, this test fails so docs/API-SSE.md is updated in the same
// PR.
func TestHTTP_SSE_triggerSurface(t *testing.T) {
	t.Run("POST /tasks (no parent) emits task_created", func(t *testing.T) {
		srv, _, hub := newSSETriggerServer(t)
		defer srv.Close()
		ch, cancel := hub.Subscribe()
		defer cancel()

		created := postTaskJSON(t, srv, `{"title":"root","priority":"medium"}`, http.StatusCreated)
		got := summarize(drainSSE(t, ch, 1, 2*time.Second))
		mustEqualEvents(t, "POST /tasks", got, []string{"task_created:" + created.ID})
	})

	t.Run("POST /tasks with parent emits task_created + parent task_updated", func(t *testing.T) {
		srv, _, hub := newSSETriggerServer(t)
		defer srv.Close()
		parent := postTaskJSON(t, srv, `{"title":"parent","priority":"medium"}`, http.StatusCreated)
		ch, cancel := hub.Subscribe()
		defer cancel()

		child := postTaskJSON(t, srv, `{"title":"child","priority":"medium","parent_id":"`+parent.ID+`"}`, http.StatusCreated)
		got := summarize(drainSSE(t, ch, 2, 2*time.Second))
		mustEqualEvents(t, "POST /tasks (with parent)", got, []string{
			"task_created:" + child.ID,
			"task_updated:" + parent.ID,
		})
	})

	t.Run("PATCH /tasks/{id} emits task_updated", func(t *testing.T) {
		srv, _, hub := newSSETriggerServer(t)
		defer srv.Close()
		task := postTaskJSON(t, srv, `{"title":"a","priority":"medium"}`, http.StatusCreated)
		ch, cancel := hub.Subscribe()
		defer cancel()

		patchTaskJSON(t, srv, task.ID, `{"title":"b"}`, http.StatusOK)
		got := summarize(drainSSE(t, ch, 1, 2*time.Second))
		mustEqualEvents(t, "PATCH /tasks/{id}", got, []string{"task_updated:" + task.ID})
	})

	t.Run("POST /tasks/{id}/checklist/items emits task_updated", func(t *testing.T) {
		srv, _, hub := newSSETriggerServer(t)
		defer srv.Close()
		task := postTaskJSON(t, srv, `{"title":"a","priority":"medium"}`, http.StatusCreated)
		ch, cancel := hub.Subscribe()
		defer cancel()

		mustDoJSON(t, http.MethodPost, srv.URL+"/tasks/"+task.ID+"/checklist/items",
			`{"text":"alpha"}`, "", http.StatusCreated)
		got := summarize(drainSSE(t, ch, 1, 2*time.Second))
		mustEqualEvents(t, "POST /tasks/{id}/checklist/items", got, []string{"task_updated:" + task.ID})
	})

	t.Run("PATCH /tasks/{id}/checklist/items/{itemId} emits task_updated", func(t *testing.T) {
		srv, st, hub := newSSETriggerServer(t)
		defer srv.Close()
		task := postTaskJSON(t, srv, `{"title":"a","priority":"medium"}`, http.StatusCreated)
		it, err := st.AddChecklistItem(context.Background(), task.ID, "alpha", domain.ActorUser)
		if err != nil {
			t.Fatal(err)
		}
		ch, cancel := hub.Subscribe()
		defer cancel()

		mustDoJSON(t, http.MethodPatch, srv.URL+"/tasks/"+task.ID+"/checklist/items/"+it.ID,
			`{"text":"beta"}`, "", http.StatusOK)
		got := summarize(drainSSE(t, ch, 1, 2*time.Second))
		mustEqualEvents(t, "PATCH /tasks/{id}/checklist/items/{itemId}", got, []string{"task_updated:" + task.ID})
	})

	t.Run("DELETE /tasks/{id}/checklist/items/{itemId} emits task_updated", func(t *testing.T) {
		srv, st, hub := newSSETriggerServer(t)
		defer srv.Close()
		task := postTaskJSON(t, srv, `{"title":"a","priority":"medium"}`, http.StatusCreated)
		it, err := st.AddChecklistItem(context.Background(), task.ID, "alpha", domain.ActorUser)
		if err != nil {
			t.Fatal(err)
		}
		ch, cancel := hub.Subscribe()
		defer cancel()

		mustDoJSON(t, http.MethodDelete, srv.URL+"/tasks/"+task.ID+"/checklist/items/"+it.ID,
			"", "", http.StatusNoContent)
		got := summarize(drainSSE(t, ch, 1, 2*time.Second))
		mustEqualEvents(t, "DELETE /tasks/{id}/checklist/items/{itemId}", got, []string{"task_updated:" + task.ID})
	})

	t.Run("PATCH /tasks/{id}/events/{seq} user response emits task_updated", func(t *testing.T) {
		srv, st, hub := newSSETriggerServer(t)
		defer srv.Close()
		task := postTaskJSON(t, srv, `{"title":"a","priority":"medium"}`, http.StatusCreated)
		// Append an approval_requested event so seq=2 is patchable with a user response.
		if err := st.AppendTaskEvent(context.Background(), task.ID, domain.EventApprovalRequested, domain.ActorAgent, []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
		ch, cancel := hub.Subscribe()
		defer cancel()

		mustDoJSON(t, http.MethodPatch, srv.URL+"/tasks/"+task.ID+"/events/2",
			`{"user_response":"ok"}`, "agent", http.StatusOK)
		got := summarize(drainSSE(t, ch, 1, 2*time.Second))
		mustEqualEvents(t, "PATCH /tasks/{id}/events/{seq}", got, []string{"task_updated:" + task.ID})
	})

	t.Run("DELETE /tasks/{id} (no parent) emits task_deleted", func(t *testing.T) {
		srv, _, hub := newSSETriggerServer(t)
		defer srv.Close()
		task := postTaskJSON(t, srv, `{"title":"a","priority":"medium"}`, http.StatusCreated)
		ch, cancel := hub.Subscribe()
		defer cancel()

		mustDoJSON(t, http.MethodDelete, srv.URL+"/tasks/"+task.ID, "", "", http.StatusNoContent)
		got := summarize(drainSSE(t, ch, 1, 2*time.Second))
		mustEqualEvents(t, "DELETE /tasks/{id}", got, []string{"task_deleted:" + task.ID})
	})

	t.Run("DELETE /tasks/{id} with parent emits task_deleted + parent task_updated", func(t *testing.T) {
		srv, _, hub := newSSETriggerServer(t)
		defer srv.Close()
		parent := postTaskJSON(t, srv, `{"title":"parent","priority":"medium"}`, http.StatusCreated)
		child := postTaskJSON(t, srv, `{"title":"child","priority":"medium","parent_id":"`+parent.ID+`"}`, http.StatusCreated)
		ch, cancel := hub.Subscribe()
		defer cancel()

		mustDoJSON(t, http.MethodDelete, srv.URL+"/tasks/"+child.ID, "", "", http.StatusNoContent)
		got := summarize(drainSSE(t, ch, 2, 2*time.Second))
		mustEqualEvents(t, "DELETE /tasks/{id} (with parent)", got, []string{
			"task_deleted:" + child.ID,
			"task_updated:" + parent.ID,
		})
	})

	t.Run("POST /tasks/{id}/cycles emits task_cycle_changed", func(t *testing.T) {
		srv, _, hub := newSSETriggerServer(t)
		defer srv.Close()
		task := postTaskJSON(t, srv, `{"title":"a","priority":"medium"}`, http.StatusCreated)
		ch, cancel := hub.Subscribe()
		defer cancel()

		cycleID := postCycleJSON(t, srv, task.ID, `{}`, http.StatusCreated)
		got := summarize(drainSSE(t, ch, 1, 2*time.Second))
		mustEqualEvents(t, "POST /tasks/{id}/cycles", got, []string{
			"task_cycle_changed:" + task.ID + "/" + cycleID,
		})
	})

	t.Run("PATCH /tasks/{id}/cycles/{cycleId} emits task_cycle_changed", func(t *testing.T) {
		srv, _, hub := newSSETriggerServer(t)
		defer srv.Close()
		task := postTaskJSON(t, srv, `{"title":"a","priority":"medium"}`, http.StatusCreated)
		cycleID := postCycleJSON(t, srv, task.ID, `{}`, http.StatusCreated)
		ch, cancel := hub.Subscribe()
		defer cancel()

		mustDoJSON(t, http.MethodPatch, srv.URL+"/tasks/"+task.ID+"/cycles/"+cycleID,
			`{"status":"succeeded"}`, "agent", http.StatusOK)
		got := summarize(drainSSE(t, ch, 1, 2*time.Second))
		mustEqualEvents(t, "PATCH /tasks/{id}/cycles/{cycleId}", got, []string{
			"task_cycle_changed:" + task.ID + "/" + cycleID,
		})
	})

	t.Run("POST /tasks/{id}/cycles/{cycleId}/phases emits task_cycle_changed", func(t *testing.T) {
		srv, _, hub := newSSETriggerServer(t)
		defer srv.Close()
		task := postTaskJSON(t, srv, `{"title":"a","priority":"medium"}`, http.StatusCreated)
		cycleID := postCycleJSON(t, srv, task.ID, `{}`, http.StatusCreated)
		ch, cancel := hub.Subscribe()
		defer cancel()

		mustDoJSON(t, http.MethodPost, srv.URL+"/tasks/"+task.ID+"/cycles/"+cycleID+"/phases",
			`{"phase":"diagnose"}`, "agent", http.StatusCreated)
		got := summarize(drainSSE(t, ch, 1, 2*time.Second))
		mustEqualEvents(t, "POST /tasks/{id}/cycles/{cycleId}/phases", got, []string{
			"task_cycle_changed:" + task.ID + "/" + cycleID,
		})
	})

	t.Run("PATCH /tasks/{id}/cycles/{cycleId}/phases/{phaseSeq} emits task_cycle_changed", func(t *testing.T) {
		srv, _, hub := newSSETriggerServer(t)
		defer srv.Close()
		task := postTaskJSON(t, srv, `{"title":"a","priority":"medium"}`, http.StatusCreated)
		cycleID := postCycleJSON(t, srv, task.ID, `{}`, http.StatusCreated)
		mustDoJSON(t, http.MethodPost, srv.URL+"/tasks/"+task.ID+"/cycles/"+cycleID+"/phases",
			`{"phase":"diagnose"}`, "agent", http.StatusCreated)
		ch, cancel := hub.Subscribe()
		defer cancel()

		mustDoJSON(t, http.MethodPatch, srv.URL+"/tasks/"+task.ID+"/cycles/"+cycleID+"/phases/1",
			`{"status":"succeeded"}`, "agent", http.StatusOK)
		got := summarize(drainSSE(t, ch, 1, 2*time.Second))
		mustEqualEvents(t, "PATCH /tasks/{id}/cycles/{cycleId}/phases/{phaseSeq}", got, []string{
			"task_cycle_changed:" + task.ID + "/" + cycleID,
		})
	})

	t.Run("read-only routes do not publish", func(t *testing.T) {
		srv, _, hub := newSSETriggerServer(t)
		defer srv.Close()
		task := postTaskJSON(t, srv, `{"title":"a","priority":"medium"}`, http.StatusCreated)
		cycleID := postCycleJSON(t, srv, task.ID, `{}`, http.StatusCreated)
		ch, cancel := hub.Subscribe()
		defer cancel()

		readOnly := []struct {
			method, url string
		}{
			{http.MethodGet, srv.URL + "/tasks"},
			{http.MethodGet, srv.URL + "/tasks/stats"},
			{http.MethodGet, srv.URL + "/tasks/" + task.ID},
			{http.MethodGet, srv.URL + "/tasks/" + task.ID + "/checklist"},
			{http.MethodGet, srv.URL + "/tasks/" + task.ID + "/events"},
			{http.MethodGet, srv.URL + "/tasks/" + task.ID + "/cycles"},
			{http.MethodGet, srv.URL + "/tasks/" + task.ID + "/cycles/" + cycleID},
		}
		for _, r := range readOnly {
			req, err := http.NewRequest(r.method, r.url, nil)
			if err != nil {
				t.Fatal(err)
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
		}
		got := summarize(drainSSE(t, ch, 1, 200*time.Millisecond))
		if len(got) != 0 {
			t.Fatalf("read-only routes published unexpectedly: %v", got)
		}
	})
}

// postCycleJSON issues POST /tasks/{taskID}/cycles with X-Actor: agent and
// returns the assigned cycle id. Mirrors postTaskJSON for the cycles surface.
func postCycleJSON(t *testing.T, srv *httptest.Server, taskID, body string, wantStatus int) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/tasks/"+taskID+"/cycles", strings.NewReader(body))
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
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != wantStatus {
		t.Fatalf("POST /tasks/%s/cycles status=%d want=%d body=%s", taskID, res.StatusCode, wantStatus, raw)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode created cycle: %v body=%s", err, raw)
	}
	if out.ID == "" {
		t.Fatalf("created cycle missing id: body=%s", raw)
	}
	return out.ID
}

func postTaskJSON(t *testing.T, srv *httptest.Server, body string, wantStatus int) domain.Task {
	t.Helper()
	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != wantStatus {
		t.Fatalf("POST /tasks status=%d want=%d body=%s", res.StatusCode, wantStatus, b)
	}
	var out domain.Task
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode created task: %v body=%s", err, b)
	}
	return out
}

func patchTaskJSON(t *testing.T, srv *httptest.Server, id, body string, wantStatus int) {
	t.Helper()
	mustDoJSON(t, http.MethodPatch, srv.URL+"/tasks/"+id, body, "", wantStatus)
}

func mustDoJSON(t *testing.T, method, url, body, actor string, wantStatus int) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		req.Header.Set("X-Actor", actor)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != wantStatus {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, url, res.StatusCode, wantStatus, b)
	}
}
