package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// jsonObjectMessageEmpty is the canonical "{}" RawMessage emitted by the
// response chokepoint when a JSON-object column is missing or corrupt.
var jsonObjectMessageEmpty = json.RawMessage(`{}`)

// normalizeJSONObjectForResponse mirrors the store-side normalizeJSONObject
// chokepoint on the response side. The store enforces the "always a JSON
// object" invariant on writes (see pkgs/tasks/store/store_cycles.go), but
// legacy rows from before that chokepoint — or any out-of-band write path
// (raw SQL, migrations, future bug) — can still carry nil / empty /
// whitespace / "null" / scalars / arrays / malformed bytes in meta_json,
// details_json, or data_json. Per docs/API-HTTP.md these columns must
// surface as a JSON object on every response, never as a JSON null or
// scalar (the SPA crashes on `Object.entries(null)`). Rather than 500
// for legacy data, the response builder defensively coerces anything
// non-object to "{}" so the client invariant always holds.
func normalizeJSONObjectForResponse(raw []byte) json.RawMessage {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.normalizeJSONObjectForResponse")
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return jsonObjectMessageEmpty
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return jsonObjectMessageEmpty
	}
	return json.RawMessage(trimmed)
}

// maxCycleListLimitParamBytes mirrors maxTaskEventSeqParamBytes — keep
// list-paging limit query strings short.
const maxCycleListLimitParamBytes = 32

// defaultCycleListLimit and maxCycleListLimit are the documented bounds for
// GET /tasks/{id}/cycles ?limit=. They follow the same 50/200 conventions
// used by GET /tasks and GET /tasks/{id}/events.
const (
	defaultCycleListLimit = 50
	maxCycleListLimit     = 200
)

// postTaskCycle handles POST /tasks/{id}/cycles.
func (h *Handler) postTaskCycle(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.postTaskCycle")
	const op = "tasks.cycle.create"
	r = calltrace.WithRequestRoot(r, op)
	taskID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body cycleStartJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "task_id", taskID, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	by := actorFromRequest(r)
	debugHTTPRequest(r, op, "task_id", taskID, "actor", string(by),
		"parent_cycle_id_set", body.ParentCycleID != nil,
		"meta_bytes", len(body.Meta))
	in := store.StartCycleInput{
		TaskID:        taskID,
		TriggeredBy:   by,
		ParentCycleID: body.ParentCycleID,
		Meta:          []byte(body.Meta),
	}
	cycle, err := h.store.StartCycle(r.Context(), in)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyCycleChange(taskID, cycle.ID)
	writeJSON(w, r, op, http.StatusCreated, taskCycleResponseFromDomain(cycle))
}

// getTaskCycles handles GET /tasks/{id}/cycles.
func (h *Handler) getTaskCycles(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.getTaskCycles")
	const op = "tasks.cycle.list"
	r = calltrace.WithRequestRoot(r, op)
	taskID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	limit, err := parseCycleListLimit(r.Context(), r.URL.Query())
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "task_id", taskID, "limit", limit)
	rows, err := h.store.ListCyclesForTask(r.Context(), taskID, limit+1)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	hasMore := false
	if len(rows) > limit {
		hasMore = true
		rows = rows[:limit]
	}
	out := make([]taskCycleResponse, 0, len(rows))
	for i := range rows {
		out = append(out, taskCycleResponseFromDomain(&rows[i]))
	}
	writeJSON(w, r, op, http.StatusOK, taskCyclesListResponse{
		TaskID:  taskID,
		Cycles:  out,
		Limit:   limit,
		HasMore: hasMore,
	})
}

// getTaskCycle handles GET /tasks/{id}/cycles/{cycleId}.
func (h *Handler) getTaskCycle(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.getTaskCycle")
	const op = "tasks.cycle.get"
	r = calltrace.WithRequestRoot(r, op)
	taskID, cycleID, err := parseCyclePathPair(r)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "task_id", taskID, "cycle_id", cycleID)
	cycle, err := h.store.GetCycle(r.Context(), cycleID)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if cycle.TaskID != taskID {
		writeStoreError(w, r, op, domain.ErrNotFound)
		return
	}
	phases, err := h.store.ListPhasesForCycle(r.Context(), cycleID)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, taskCycleDetailFromDomain(cycle, phases))
}

// patchTaskCycle handles PATCH /tasks/{id}/cycles/{cycleId}.
func (h *Handler) patchTaskCycle(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patchTaskCycle")
	const op = "tasks.cycle.terminate"
	r = calltrace.WithRequestRoot(r, op)
	taskID, cycleID, err := parseCyclePathPair(r)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body cycleTerminateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "task_id", taskID, "cycle_id", cycleID, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	by := actorFromRequest(r)
	debugHTTPRequest(r, op, "task_id", taskID, "cycle_id", cycleID,
		"actor", string(by), "body_status", string(body.Status),
		"reason_len", len(body.Reason),
		"reason_preview", truncateRunes(body.Reason, maxHTTPLogTextRunes))
	if err := assertCycleBelongsToTask(r.Context(), h.store, taskID, cycleID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	cycle, err := h.store.TerminateCycle(r.Context(), cycleID, body.Status, body.Reason, by)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyCycleChange(taskID, cycleID)
	writeJSON(w, r, op, http.StatusOK, taskCycleResponseFromDomain(cycle))
}

// postTaskCyclePhase handles POST /tasks/{id}/cycles/{cycleId}/phases.
func (h *Handler) postTaskCyclePhase(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.postTaskCyclePhase")
	const op = "tasks.cycle.phase.create"
	r = calltrace.WithRequestRoot(r, op)
	taskID, cycleID, err := parseCyclePathPair(r)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body phaseStartJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "task_id", taskID, "cycle_id", cycleID, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	by := actorFromRequest(r)
	debugHTTPRequest(r, op, "task_id", taskID, "cycle_id", cycleID,
		"actor", string(by), "body_phase", string(body.Phase))
	if err := assertCycleBelongsToTask(r.Context(), h.store, taskID, cycleID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	phase, err := h.store.StartPhase(r.Context(), cycleID, body.Phase, by)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyCycleChange(taskID, cycleID)
	writeJSON(w, r, op, http.StatusCreated, taskCyclePhaseResponseFromDomain(phase))
}

// patchTaskCyclePhase handles PATCH /tasks/{id}/cycles/{cycleId}/phases/{phaseSeq}.
func (h *Handler) patchTaskCyclePhase(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patchTaskCyclePhase")
	const op = "tasks.cycle.phase.complete"
	r = calltrace.WithRequestRoot(r, op)
	taskID, cycleID, err := parseCyclePathPair(r)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	phaseSeq, err := parseTaskPathPhaseSeq(r.PathValue("phaseSeq"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body phasePatchJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "task_id", taskID, "cycle_id", cycleID, "phase_seq", phaseSeq, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	by := actorFromRequest(r)
	debugHTTPRequest(r, op, "task_id", taskID, "cycle_id", cycleID, "phase_seq", phaseSeq,
		"actor", string(by), "body_status", string(body.Status),
		"summary_set", body.Summary != nil, "details_bytes", len(body.Details))
	if err := assertCycleBelongsToTask(r.Context(), h.store, taskID, cycleID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	in := store.CompletePhaseInput{
		CycleID:  cycleID,
		PhaseSeq: phaseSeq,
		Status:   body.Status,
		Summary:  body.Summary,
		Details:  []byte(body.Details),
		By:       by,
	}
	ph, err := h.store.CompletePhase(r.Context(), in)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyCycleChange(taskID, cycleID)
	writeJSON(w, r, op, http.StatusOK, taskCyclePhaseResponseFromDomain(ph))
}

// parseCyclePathPair extracts both the {id} (task) and {cycleId} path
// segments with the same length and trim guards used elsewhere. Returns
// the first error so the handler can respond with one well-formed 400.
func parseCyclePathPair(r *http.Request) (string, string, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseCyclePathPair")
	taskID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		return "", "", err
	}
	cycleID, err := parseTaskPathCycleID(r.PathValue("cycleId"))
	if err != nil {
		return "", "", err
	}
	return taskID, cycleID, nil
}

// assertCycleBelongsToTask preflights write routes so a cycleId from a
// different task surfaces as 404 instead of mutating the wrong row. The
// store does not enforce this implicitly because cycleId is unique on its
// own, so the handler must check.
func assertCycleBelongsToTask(ctx context.Context, s *store.Store, taskID, cycleID string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.assertCycleBelongsToTask")
	c, err := s.GetCycle(ctx, cycleID)
	if err != nil {
		return err
	}
	if c.TaskID != taskID {
		return domain.ErrNotFound
	}
	return nil
}

// parseCycleListLimit is the GET /tasks/{id}/cycles equivalent of
// parseTaskEventsLimit. Same 0..200 cap and 32-byte abuse guard.
func parseCycleListLimit(ctx context.Context, q url.Values) (int, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseCycleListLimit")
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = calltrace.Push(ctx, "parseCycleListLimit")
	calltrace.HelperIOIn(ctx, "parseCycleListLimit", "limit_q", q.Get("limit"))
	var (
		limit = defaultCycleListLimit
		err   error
	)
	defer func() { calltrace.HelperIOOut(ctx, "parseCycleListLimit", "limit", limit, "err", err) }()
	v := strings.TrimSpace(q.Get("limit"))
	if v == "" {
		return limit, nil
	}
	if len(v) > maxCycleListLimitParamBytes {
		err = fmt.Errorf("%w: limit too long", domain.ErrInvalidInput)
		return 0, err
	}
	n, e := strconv.Atoi(v)
	if e != nil || n < 0 || n > maxCycleListLimit {
		err = fmt.Errorf("%w: limit must be integer 0..200", domain.ErrInvalidInput)
		return 0, err
	}
	if n == 0 {
		return defaultCycleListLimit, nil
	}
	limit = n
	return limit, nil
}

// taskCycleResponseFromDomain copies a TaskCycle GORM row into the JSON
// response shape. Meta is normalized to "{}" if the column came back as
// nil / empty / whitespace / null / a scalar / an array / malformed JSON,
// matching the docs/API-HTTP.md "always a JSON object" invariant.
func taskCycleResponseFromDomain(c *domain.TaskCycle) taskCycleResponse {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.taskCycleResponseFromDomain")
	meta := normalizeJSONObjectForResponse(c.MetaJSON)
	return taskCycleResponse{
		ID:            c.ID,
		TaskID:        c.TaskID,
		AttemptSeq:    c.AttemptSeq,
		Status:        c.Status,
		StartedAt:     c.StartedAt,
		EndedAt:       c.EndedAt,
		TriggeredBy:   c.TriggeredBy,
		ParentCycleID: c.ParentCycleID,
		Meta:          meta,
	}
}

// taskCyclePhaseResponseFromDomain copies a TaskCyclePhase row into the
// JSON response shape. Details is normalized via the same chokepoint as
// Meta — see taskCycleResponseFromDomain.
func taskCyclePhaseResponseFromDomain(p *domain.TaskCyclePhase) taskCyclePhaseResponse {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.taskCyclePhaseResponseFromDomain")
	details := normalizeJSONObjectForResponse(p.DetailsJSON)
	return taskCyclePhaseResponse{
		ID:        p.ID,
		CycleID:   p.CycleID,
		Phase:     p.Phase,
		PhaseSeq:  p.PhaseSeq,
		Status:    p.Status,
		StartedAt: p.StartedAt,
		EndedAt:   p.EndedAt,
		Summary:   p.Summary,
		Details:   details,
		EventSeq:  p.EventSeq,
	}
}

// taskCycleDetailFromDomain assembles the GET /tasks/{id}/cycles/{cycleId}
// envelope: cycle fields inlined plus phases in execution order.
func taskCycleDetailFromDomain(c *domain.TaskCycle, phases []domain.TaskCyclePhase) taskCycleDetailResponse {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.taskCycleDetailFromDomain")
	meta := normalizeJSONObjectForResponse(c.MetaJSON)
	out := taskCycleDetailResponse{
		ID:            c.ID,
		TaskID:        c.TaskID,
		AttemptSeq:    c.AttemptSeq,
		Status:        c.Status,
		StartedAt:     c.StartedAt,
		EndedAt:       c.EndedAt,
		TriggeredBy:   c.TriggeredBy,
		ParentCycleID: c.ParentCycleID,
		Meta:          meta,
		Phases:        make([]taskCyclePhaseResponse, 0, len(phases)),
	}
	for i := range phases {
		out.Phases = append(out.Phases, taskCyclePhaseResponseFromDomain(&phases[i]))
	}
	return out
}
