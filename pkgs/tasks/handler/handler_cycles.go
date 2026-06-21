package handler

import (
	"log/slog"
	"net/http"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
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
	h.notifyCycleChangedFromStore(r.Context(), taskID, cycle.ID)
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
		next := out[len(out)-1].AttemptSeq
		resp.NextBeforeAttemptSeq = &next
	}
	writeJSONWithETag(w, r, op, http.StatusOK, resp)
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
	writeJSONWithETag(w, r, op, http.StatusOK, taskCycleDetailFromDomain(cycle, phases))
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

// getTaskCycleVerdicts handles GET /tasks/{id}/cycles/{cycleId}/verdicts.
func (h *Handler) getTaskCycleVerdicts(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.getTaskCycleVerdicts")
	const op = "tasks.cycle.verdicts.get"
	r = calltrace.WithRequestRoot(r, op)
	taskID, cycleID, err := parseCyclePathPair(r)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "task_id", taskID, "cycle_id", cycleID)
	if err := assertCycleBelongsToTask(r.Context(), h.store, taskID, cycleID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	criteriaRows, err := h.store.ListCriteriaReportsForCycle(r.Context(), cycleID)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	verifyRows, err := h.store.ListVerifyReportsForCycle(r.Context(), cycleID)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	commandRows, err := h.store.ListCommandRunsForCycle(r.Context(), cycleID)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	commitRows, err := h.store.ListCommitsForCycle(r.Context(), cycleID)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	resp := cycleVerdictsResponse{
		TaskID:          taskID,
		CycleID:         cycleID,
		Commits:         make([]cycleCommitEntry, 0, len(commitRows)),
		CriteriaReports: make([]cycleCriteriaReportEntry, 0, len(criteriaRows)),
		VerifyReports:   make([]cycleVerifyReportEntry, 0, len(verifyRows)),
		CommandRuns:     make([]cycleCommandRunEntry, 0, len(commandRows)),
	}
	for i := range commitRows {
		resp.Commits = append(resp.Commits, cycleCommitFromDomain(&commitRows[i]))
	}
	if len(commitRows) > 0 {
		resp.GitContext = cycleGitContextFromCommits(commitRows)
	}
	for i := range criteriaRows {
		resp.CriteriaReports = append(resp.CriteriaReports, cycleCriteriaReportFromDomain(&criteriaRows[i]))
	}
	for i := range verifyRows {
		resp.VerifyReports = append(resp.VerifyReports, cycleVerifyReportFromDomain(&verifyRows[i]))
	}
	for i := range commandRows {
		resp.CommandRuns = append(resp.CommandRuns, cycleCommandRunFromDomain(&commandRows[i]))
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
	h.notifyCycleChangedFromStore(r.Context(), taskID, cycleID)
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
	h.notifyCycleChangedFromStore(r.Context(), taskID, cycleID)
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
	h.notifyCycleChangedFromStore(r.Context(), taskID, cycleID)
	writeJSON(w, r, op, http.StatusOK, taskCyclePhaseResponseFromDomain(ph))
}
