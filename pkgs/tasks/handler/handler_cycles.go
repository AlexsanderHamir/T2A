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
	defaultCycleListLimit   = 50
	maxCycleListLimit       = 200
	defaultCycleStreamLimit = 100
	maxCycleStreamLimit     = 500
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
	beforeAttemptSeq, err := parseCycleListBeforeAttemptSeq(r.Context(), r.URL.Query())
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "task_id", taskID, "limit", limit, "before_attempt_seq", beforeAttemptSeq)
	rows, err := h.store.ListCyclesForTaskBefore(r.Context(), taskID, beforeAttemptSeq, limit+1)
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
	resp := taskCyclesListResponse{
		TaskID:  taskID,
		Cycles:  out,
		Limit:   limit,
		HasMore: hasMore,
	}
	if hasMore && len(out) > 0 {
		// Cursor for the next (older) page is the last (lowest attempt_seq)
		// row this response carries. Strict < in the store keeps the cursor
		// row from being repeated across pages, matching the /events
		// `before_seq` convention. Only emitted when a next page actually
		// exists so clients can use omitempty as the end-of-stream signal.
		next := out[len(out)-1].AttemptSeq
		resp.NextBeforeAttemptSeq = &next
	}
	writeJSON(w, r, op, http.StatusOK, resp)
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

// getTaskCycleStream handles GET /tasks/{id}/cycles/{cycleId}/stream.
func (h *Handler) getTaskCycleStream(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.getTaskCycleStream")
	const op = "tasks.cycle.stream.list"
	r = calltrace.WithRequestRoot(r, op)
	taskID, cycleID, err := parseCyclePathPair(r)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	limit, err := parseCycleStreamLimit(r.Context(), r.URL.Query())
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	afterSeq, err := parseCycleStreamAfterSeq(r.Context(), r.URL.Query())
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if err := assertCycleBelongsToTask(r.Context(), h.store, taskID, cycleID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	rows, err := h.store.ListCycleStreamEvents(r.Context(), cycleID, afterSeq, limit+1)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	hasMore := false
	if len(rows) > limit {
		hasMore = true
		rows = rows[:limit]
	}
	out := make([]taskCycleStreamEventResponse, 0, len(rows))
	for i := range rows {
		out = append(out, taskCycleStreamEventResponseFromDomain(&rows[i]))
	}
	resp := taskCycleStreamListResponse{
		TaskID:  taskID,
		CycleID: cycleID,
		Events:  out,
		Limit:   limit,
		HasMore: hasMore,
	}
	if hasMore && len(out) > 0 {
		next := out[len(out)-1].StreamSeq
		resp.NextAfterSeq = &next
	}
	writeJSON(w, r, op, http.StatusOK, resp)
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

// parseCycleListBeforeAttemptSeq parses the optional ?before_attempt_seq=
// keyset cursor for GET /tasks/{id}/cycles. Mirrors the validation used
// by ?before_seq= on /tasks/{id}/events: 32-byte abuse guard, must be a
// strictly positive int64. Returns 0 (no cursor / first page) when the
// param is absent or empty after trim.
func parseCycleListBeforeAttemptSeq(ctx context.Context, q url.Values) (before int64, err error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseCycleListBeforeAttemptSeq")
	ctx = calltrace.Push(ctx, "parseCycleListBeforeAttemptSeq")
	calltrace.HelperIOIn(ctx, "parseCycleListBeforeAttemptSeq", "before_q", q.Get("before_attempt_seq"))
	defer func() {
		calltrace.HelperIOOut(ctx, "parseCycleListBeforeAttemptSeq", "before_attempt_seq", before, "err", err)
	}()
	v := strings.TrimSpace(q.Get("before_attempt_seq"))
	if v == "" {
		return 0, nil
	}
	if len(v) > maxCycleListLimitParamBytes {
		return 0, fmt.Errorf("%w: before_attempt_seq too long", domain.ErrInvalidInput)
	}
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil || n < 1 {
		return 0, fmt.Errorf("%w: before_attempt_seq must be a positive integer", domain.ErrInvalidInput)
	}
	return n, nil
}

// parseCycleListLimit is the GET /tasks/{id}/cycles equivalent of
// parseTaskEventsLimit. Same 0..200 cap and 32-byte abuse guard.
func parseCycleListLimit(ctx context.Context, q url.Values) (int, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseCycleListLimit")
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

func parseCycleStreamAfterSeq(ctx context.Context, q url.Values) (after int64, err error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseCycleStreamAfterSeq")
	ctx = calltrace.Push(ctx, "parseCycleStreamAfterSeq")
	calltrace.HelperIOIn(ctx, "parseCycleStreamAfterSeq", "after_q", q.Get("after_seq"))
	defer func() { calltrace.HelperIOOut(ctx, "parseCycleStreamAfterSeq", "after_seq", after, "err", err) }()
	v := strings.TrimSpace(q.Get("after_seq"))
	if v == "" {
		return 0, nil
	}
	if len(v) > maxCycleListLimitParamBytes {
		return 0, fmt.Errorf("%w: after_seq too long", domain.ErrInvalidInput)
	}
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil || n < 1 {
		return 0, fmt.Errorf("%w: after_seq must be a positive integer", domain.ErrInvalidInput)
	}
	return n, nil
}

func parseCycleStreamLimit(ctx context.Context, q url.Values) (int, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseCycleStreamLimit")
	ctx = calltrace.Push(ctx, "parseCycleStreamLimit")
	calltrace.HelperIOIn(ctx, "parseCycleStreamLimit", "limit_q", q.Get("limit"))
	var (
		limit = defaultCycleStreamLimit
		err   error
	)
	defer func() { calltrace.HelperIOOut(ctx, "parseCycleStreamLimit", "limit", limit, "err", err) }()
	v := strings.TrimSpace(q.Get("limit"))
	if v == "" {
		return limit, nil
	}
	if len(v) > maxCycleListLimitParamBytes {
		err = fmt.Errorf("%w: limit too long", domain.ErrInvalidInput)
		return 0, err
	}
	n, e := strconv.Atoi(v)
	if e != nil || n < 0 || n > maxCycleStreamLimit {
		err = fmt.Errorf("%w: limit must be integer 0..500", domain.ErrInvalidInput)
		return 0, err
	}
	if n == 0 {
		return defaultCycleStreamLimit, nil
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
		CycleMeta:     projectCycleMeta(meta),
	}
}

// projectCycleMeta extracts the typed runner / model / prompt-hash
// fields from a normalized meta object (always a valid JSON object,
// per normalizeJSONObjectForResponse). Unknown keys are ignored;
// missing keys decode to "" — that empty string is meaningful and
// MUST be preserved end-to-end (see cycleMetaProjection doc). Errors
// from json.Unmarshal are not possible in practice (input is a
// valid object) but we defensively log + return the zero projection
// so a corrupt row never breaks the cycles endpoint.
func projectCycleMeta(meta json.RawMessage) cycleMetaProjection {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.projectCycleMeta")
	var out cycleMetaProjection
	if len(meta) == 0 {
		return out
	}
	if err := json.Unmarshal(meta, &out); err != nil {
		slog.Warn("cycle meta projection failed",
			"cmd", calltrace.LogCmd,
			"operation", "handler.projectCycleMeta.err",
			"err", err)
		return cycleMetaProjection{}
	}
	return out
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
		CycleMeta:     projectCycleMeta(meta),
		Phases:        make([]taskCyclePhaseResponse, 0, len(phases)),
	}
	for i := range phases {
		out.Phases = append(out.Phases, taskCyclePhaseResponseFromDomain(&phases[i]))
	}
	return out
}

func taskCycleStreamEventResponseFromDomain(ev *domain.TaskCycleStreamEvent) taskCycleStreamEventResponse {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.taskCycleStreamEventResponseFromDomain")
	return taskCycleStreamEventResponse{
		ID:        ev.ID,
		TaskID:    ev.TaskID,
		CycleID:   ev.CycleID,
		PhaseSeq:  ev.PhaseSeq,
		StreamSeq: ev.StreamSeq,
		At:        ev.At,
		Source:    ev.Source,
		Kind:      ev.Kind,
		Subtype:   ev.Subtype,
		Message:   ev.Message,
		Tool:      ev.Tool,
		Payload:   normalizeJSONObjectForResponse(ev.PayloadJSON),
	}
}
