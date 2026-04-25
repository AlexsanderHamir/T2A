package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// mustCreateTaskForCycles is the cycles-suite POST /tasks helper. It is
// scoped to this file (other suites have their own narrower variants) so the
// cycles tests can stay focused on the cycle/phase round-trip.
func mustCreateTaskForCycles(t *testing.T, baseURL string) string {
	t.Helper()
	res, err := http.Post(baseURL+"/tasks", "application/json", strings.NewReader(`{"title":"cycles-task","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	if cerr := res.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create task: status %d body=%s", res.StatusCode, body)
	}
	var task domain.Task
	if err := json.Unmarshal(body, &task); err != nil {
		t.Fatalf("decode created task: %v body=%s", err, body)
	}
	return task.ID
}

// doCyclesRequest issues a request with X-Actor: agent (cycle/phase mutations
// are typically agent-driven) and returns the response and body for assertions.
func doCyclesRequest(t *testing.T, method, url, body string) (*http.Response, []byte) {
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
	req.Header.Set("X-Actor", "agent")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(res.Body)
	if cerr := res.Body.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	return res, raw
}

func TestHTTP_postTaskCycle_creates_running_cycle(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)

	res, raw := doCyclesRequest(t, http.MethodPost, srv.URL+"/tasks/"+taskID+"/cycles", `{"meta":{"runner":"cursor-cli"}}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content type %q", ct)
	}
	var got taskCycleResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if got.ID == "" || got.TaskID != taskID {
		t.Fatalf("identity got=%#v", got)
	}
	if got.AttemptSeq != 1 {
		t.Fatalf("attempt_seq=%d want 1", got.AttemptSeq)
	}
	if got.Status != domain.CycleStatusRunning {
		t.Fatalf("status=%s want running", got.Status)
	}
	if got.TriggeredBy != domain.ActorAgent {
		t.Fatalf("triggered_by=%s want agent (X-Actor header)", got.TriggeredBy)
	}
	if got.EndedAt != nil {
		t.Fatalf("ended_at should be nil for a running cycle, got %v", got.EndedAt)
	}
	var meta map[string]any
	if err := json.Unmarshal(got.Meta, &meta); err != nil {
		t.Fatalf("decode meta: %v raw=%s", err, got.Meta)
	}
	if meta["runner"] != "cursor-cli" {
		t.Fatalf("meta.runner=%v", meta["runner"])
	}
}

func TestHTTP_getTaskCycles_lists_in_attempt_desc(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)

	for i := 0; i < 3; i++ {
		res, raw := doCyclesRequest(t, http.MethodPost, srv.URL+"/tasks/"+taskID+"/cycles", `{}`)
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("seed cycle %d: status %d body=%s", i, res.StatusCode, raw)
		}
		var c taskCycleResponse
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatal(err)
		}
		_, term := doCyclesRequest(t, http.MethodPatch, srv.URL+"/tasks/"+taskID+"/cycles/"+c.ID, `{"status":"succeeded"}`)
		if !json.Valid(term) {
			t.Fatalf("terminate %d body invalid: %s", i, term)
		}
	}

	res, raw := doCyclesRequest(t, http.MethodGet, srv.URL+"/tasks/"+taskID+"/cycles", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list status %d body=%s", res.StatusCode, raw)
	}
	var got taskCyclesListResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if got.TaskID != taskID {
		t.Fatalf("task_id=%q want %q", got.TaskID, taskID)
	}
	if got.Limit != defaultCycleListLimit {
		t.Fatalf("limit=%d want %d (default echo)", got.Limit, defaultCycleListLimit)
	}
	if got.HasMore {
		t.Fatalf("has_more=true with only 3 cycles and default limit")
	}
	if len(got.Cycles) != 3 {
		t.Fatalf("got %d cycles want 3", len(got.Cycles))
	}
	if got.Cycles[0].AttemptSeq != 3 || got.Cycles[1].AttemptSeq != 2 || got.Cycles[2].AttemptSeq != 1 {
		t.Fatalf("attempt order = [%d,%d,%d] want [3,2,1]", got.Cycles[0].AttemptSeq, got.Cycles[1].AttemptSeq, got.Cycles[2].AttemptSeq)
	}
}

func TestHTTP_getTaskCycleStream_listsPersistedEvents(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	cycle, phase := mustCreateCycleWithExecutePhase(t, st, context.Background(), taskID)

	for i := 0; i < 2; i++ {
		if _, err := st.AppendCycleStreamEvent(context.Background(), store.AppendCycleStreamEventInput{
			TaskID:   taskID,
			CycleID:  cycle.ID,
			PhaseSeq: phase.PhaseSeq,
			Source:   "cursor",
			Kind:     "message",
			Message:  "stream event " + strconv.Itoa(i+1),
			Payload:  []byte(`{"kind":"message"}`),
		}); err != nil {
			t.Fatalf("append stream event: %v", err)
		}
	}

	res, raw := doCyclesRequest(t, http.MethodGet, srv.URL+"/tasks/"+taskID+"/cycles/"+cycle.ID+"/stream?limit=1", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var got taskCycleStreamListResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if got.TaskID != taskID || got.CycleID != cycle.ID {
		t.Fatalf("identity got=%#v", got)
	}
	if !got.HasMore || got.NextAfterSeq == nil || *got.NextAfterSeq != 1 {
		t.Fatalf("paging got has_more=%v next=%v", got.HasMore, got.NextAfterSeq)
	}
	if len(got.Events) != 1 || got.Events[0].StreamSeq != 1 || got.Events[0].Message != "stream event 1" {
		t.Fatalf("events=%#v", got.Events)
	}
}

func TestHTTP_getTaskCycleStream_crossTaskCycleIsNotFound(t *testing.T) {
	srv, st := newTaskTestServerWithStore(t)
	defer srv.Close()
	taskA := mustCreateTaskForCycles(t, srv.URL)
	taskB := mustCreateTaskForCycles(t, srv.URL)
	cycle, _ := mustCreateCycleWithExecutePhase(t, st, context.Background(), taskA)

	res, raw := doCyclesRequest(t, http.MethodGet, srv.URL+"/tasks/"+taskB+"/cycles/"+cycle.ID+"/stream", "")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
}

func mustCreateCycleWithExecutePhase(t *testing.T, st *store.Store, ctx context.Context, taskID string) (*domain.TaskCycle, *domain.TaskCyclePhase) {
	t.Helper()
	cycle, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: taskID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	diag, err := st.StartPhase(ctx, cycle.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start diagnose: %v", err)
	}
	summary := "skip"
	if _, err := st.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID:  cycle.ID,
		PhaseSeq: diag.PhaseSeq,
		Status:   domain.PhaseStatusSkipped,
		Summary:  &summary,
		By:       domain.ActorAgent,
	}); err != nil {
		t.Fatalf("complete diagnose: %v", err)
	}
	phase, err := st.StartPhase(ctx, cycle.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start execute: %v", err)
	}
	return cycle, phase
}

func TestHTTP_getTaskCycles_has_more_when_overflow(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	for i := 0; i < 3; i++ {
		res, raw := doCyclesRequest(t, http.MethodPost, srv.URL+"/tasks/"+taskID+"/cycles", `{}`)
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("seed cycle %d: %d %s", i, res.StatusCode, raw)
		}
		var c taskCycleResponse
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatal(err)
		}
		doCyclesRequest(t, http.MethodPatch, srv.URL+"/tasks/"+taskID+"/cycles/"+c.ID, `{"status":"succeeded"}`)
	}

	res, raw := doCyclesRequest(t, http.MethodGet, srv.URL+"/tasks/"+taskID+"/cycles?limit=2", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var got taskCyclesListResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Limit != 2 || !got.HasMore || len(got.Cycles) != 2 {
		t.Fatalf("paging got limit=%d has_more=%v cycles=%d body=%s", got.Limit, got.HasMore, len(got.Cycles), raw)
	}
}

func TestHTTP_getTaskCycle_embeds_phases(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)

	_, createdRaw := doCyclesRequest(t, http.MethodPost, srv.URL+"/tasks/"+taskID+"/cycles", `{}`)
	var cycle taskCycleResponse
	if err := json.Unmarshal(createdRaw, &cycle); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{"diagnose", "execute"} {
		res, raw := doCyclesRequest(t, http.MethodPost,
			srv.URL+"/tasks/"+taskID+"/cycles/"+cycle.ID+"/phases", `{"phase":"`+p+`"}`)
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("start phase %s: %d %s", p, res.StatusCode, raw)
		}
		var ph taskCyclePhaseResponse
		if err := json.Unmarshal(raw, &ph); err != nil {
			t.Fatal(err)
		}
		patchBody := `{"status":"succeeded","summary":"ok"}`
		_, prRaw := doCyclesRequest(t, http.MethodPatch,
			srv.URL+"/tasks/"+taskID+"/cycles/"+cycle.ID+"/phases/"+strItoa(ph.PhaseSeq), patchBody)
		if !json.Valid(prRaw) {
			t.Fatalf("phase patch body invalid: %s", prRaw)
		}
	}

	res, raw := doCyclesRequest(t, http.MethodGet, srv.URL+"/tasks/"+taskID+"/cycles/"+cycle.ID, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var got taskCycleDetailResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if got.ID != cycle.ID || got.TaskID != taskID {
		t.Fatalf("identity got=%#v", got)
	}
	if len(got.Phases) != 2 {
		t.Fatalf("phases=%d want 2 body=%s", len(got.Phases), raw)
	}
	if got.Phases[0].Phase != domain.PhaseDiagnose || got.Phases[0].PhaseSeq != 1 {
		t.Fatalf("phases[0] %#v want diagnose seq 1", got.Phases[0])
	}
	if got.Phases[1].Phase != domain.PhaseExecute || got.Phases[1].PhaseSeq != 2 {
		t.Fatalf("phases[1] %#v want execute seq 2", got.Phases[1])
	}
	if got.Phases[0].Status != domain.PhaseStatusSucceeded || got.Phases[0].Summary == nil || *got.Phases[0].Summary != "ok" {
		t.Fatalf("phases[0] terminal state %#v", got.Phases[0])
	}
	if got.Phases[0].EventSeq == nil || got.Phases[1].EventSeq == nil {
		t.Fatalf("event_seq backlinks must be populated, got %v / %v", got.Phases[0].EventSeq, got.Phases[1].EventSeq)
	}
}

func TestHTTP_patchTaskCycle_terminates_and_returns_terminal_state(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	_, createdRaw := doCyclesRequest(t, http.MethodPost, srv.URL+"/tasks/"+taskID+"/cycles", `{}`)
	var cycle taskCycleResponse
	if err := json.Unmarshal(createdRaw, &cycle); err != nil {
		t.Fatal(err)
	}

	res, raw := doCyclesRequest(t, http.MethodPatch,
		srv.URL+"/tasks/"+taskID+"/cycles/"+cycle.ID, `{"status":"failed","reason":"timeout in execute"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var got taskCycleResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.CycleStatusFailed {
		t.Fatalf("status=%s want failed", got.Status)
	}
	if got.EndedAt == nil {
		t.Fatalf("ended_at should be set on terminal cycle")
	}
}

func TestHTTP_postTaskCyclePhase_starts_running_phase(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	_, createdRaw := doCyclesRequest(t, http.MethodPost, srv.URL+"/tasks/"+taskID+"/cycles", `{}`)
	var cycle taskCycleResponse
	if err := json.Unmarshal(createdRaw, &cycle); err != nil {
		t.Fatal(err)
	}
	res, raw := doCyclesRequest(t, http.MethodPost,
		srv.URL+"/tasks/"+taskID+"/cycles/"+cycle.ID+"/phases", `{"phase":"diagnose"}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var got taskCyclePhaseResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Phase != domain.PhaseDiagnose || got.PhaseSeq != 1 || got.Status != domain.PhaseStatusRunning {
		t.Fatalf("phase create %#v", got)
	}
	if got.EndedAt != nil {
		t.Fatalf("ended_at should be nil for running phase")
	}
	if got.EventSeq == nil {
		t.Fatalf("event_seq must be backfilled at start")
	}
}

func TestHTTP_patchTaskCyclePhase_completes_with_summary_and_details(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)
	_, createdRaw := doCyclesRequest(t, http.MethodPost, srv.URL+"/tasks/"+taskID+"/cycles", `{}`)
	var cycle taskCycleResponse
	if err := json.Unmarshal(createdRaw, &cycle); err != nil {
		t.Fatal(err)
	}
	_, phRaw := doCyclesRequest(t, http.MethodPost,
		srv.URL+"/tasks/"+taskID+"/cycles/"+cycle.ID+"/phases", `{"phase":"diagnose"}`)
	var ph taskCyclePhaseResponse
	if err := json.Unmarshal(phRaw, &ph); err != nil {
		t.Fatal(err)
	}
	startEventSeq := ph.EventSeq

	body := `{"status":"succeeded","summary":"diagnosed root cause","details":{"file":"a.go","line":42}}`
	res, raw := doCyclesRequest(t, http.MethodPatch,
		srv.URL+"/tasks/"+taskID+"/cycles/"+cycle.ID+"/phases/"+strItoa(ph.PhaseSeq), body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", res.StatusCode, raw)
	}
	var got taskCyclePhaseResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.PhaseStatusSucceeded {
		t.Fatalf("status=%s want succeeded", got.Status)
	}
	if got.Summary == nil || *got.Summary != "diagnosed root cause" {
		t.Fatalf("summary=%v", got.Summary)
	}
	if got.EventSeq == nil || startEventSeq == nil || *got.EventSeq <= *startEventSeq {
		t.Fatalf("event_seq must advance past start: start=%v end=%v", startEventSeq, got.EventSeq)
	}
	var details map[string]any
	if err := json.Unmarshal(got.Details, &details); err != nil {
		t.Fatalf("decode details: %v raw=%s", err, got.Details)
	}
	if details["file"] != "a.go" || details["line"].(float64) != 42 {
		t.Fatalf("details=%v", details)
	}
}

// TestHTTP_cycle_routes_appendMirrorEvents_into_audit_log proves the dual-write
// promise from Stage 3 still holds when the writes are issued via HTTP: each
// cycle/phase mutation must produce a matching task_events row visible from
// GET /tasks/{id}/events.
func TestHTTP_cycle_routes_appendMirrorEvents_into_audit_log(t *testing.T) {
	srv := newTaskTestServer(t)
	defer srv.Close()
	taskID := mustCreateTaskForCycles(t, srv.URL)

	_, createdRaw := doCyclesRequest(t, http.MethodPost, srv.URL+"/tasks/"+taskID+"/cycles", `{}`)
	var cycle taskCycleResponse
	if err := json.Unmarshal(createdRaw, &cycle); err != nil {
		t.Fatal(err)
	}
	_, phRaw := doCyclesRequest(t, http.MethodPost,
		srv.URL+"/tasks/"+taskID+"/cycles/"+cycle.ID+"/phases", `{"phase":"diagnose"}`)
	var ph taskCyclePhaseResponse
	if err := json.Unmarshal(phRaw, &ph); err != nil {
		t.Fatal(err)
	}
	doCyclesRequest(t, http.MethodPatch,
		srv.URL+"/tasks/"+taskID+"/cycles/"+cycle.ID+"/phases/"+strItoa(ph.PhaseSeq),
		`{"status":"succeeded"}`)
	doCyclesRequest(t, http.MethodPatch, srv.URL+"/tasks/"+taskID+"/cycles/"+cycle.ID, `{"status":"succeeded"}`)

	res, raw := doCyclesRequest(t, http.MethodGet, srv.URL+"/tasks/"+taskID+"/events", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("events status %d body=%s", res.StatusCode, raw)
	}
	var got taskEventsResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode events: %v body=%s", err, raw)
	}
	wantTypes := []domain.EventType{
		domain.EventTaskCreated,
		domain.EventCycleStarted,
		domain.EventPhaseStarted,
		domain.EventPhaseCompleted,
		domain.EventCycleCompleted,
	}
	if len(got.Events) != len(wantTypes) {
		t.Fatalf("events=%d want %d body=%s", len(got.Events), len(wantTypes), raw)
	}
	for i, want := range wantTypes {
		if got.Events[i].Type != want {
			t.Fatalf("events[%d].type=%s want %s (full=%v)", i, got.Events[i].Type, want, eventTypes(got.Events))
		}
	}
}

// strItoa is a tiny int64→string helper for path segments. Wraps strconv so
// the assertion sites stay short and call sites read top-down.
func strItoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

// eventTypes is a small debug helper used by the audit-log test to surface a
// readable diff when mirror types drift.
func eventTypes(evs []taskEventLine) []domain.EventType {
	out := make([]domain.EventType, 0, len(evs))
	for _, e := range evs {
		out = append(out, e.Type)
	}
	return out
}
