package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
	"github.com/google/uuid"
)

const (
	maxListIntQueryParamBytes          = 32
	maxListAfterIDParamBytes           = 128
	maxTemplateInstantiateCountPerItem = 25
	maxTemplateInstantiateTotalCreates = 100
)

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.create")
	const op = "tasks.create"
	r = calltrace.WithRequestRoot(r, op)
	var body taskCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	by := actorFromRequest(r)
	debugHTTPRequest(r, op, taskCreateInputFields(&body, string(by))...)
	task, err := h.createTaskFromComposeJSON(r.Context(), r, op, taskCreateJSONToCompose(body), createTaskComposeOpts{
		ID:      body.ID,
		DraftID: body.DraftID,
		Gate:    body.Gate,
	}, by)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusCreated, task)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.get")
	const op = "tasks.get"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "task_id", id)
	t, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSONWithETag(w, r, op, http.StatusOK, t)
}

// list serves GET /tasks — the hottest read path in taskapi (SPA initial load
// and SSE-driven refetch). Failure contract for operators:
//   - Invalid query params → 400 {"error":"..."} with failure_stage=parse_list_params
//     and raw limit_q/offset_q/after_id_q on the warn-level "request failed" log.
//   - Store/persistence errors (closed DB, driver faults) → 500 with
//     failure_stage=store_list plus resolved limit/offset/after_id/pagination_mode.
//   - Request context canceled or deadline exceeded → 408/504 via storeErrHTTPResponse.
//   - JSON encode failures → 500 with error log msg=response encode failed and
//     failure_stage=response_encode (includes request_id and route when available).
//   - Response-body write failures after headers → truncated body with
//     msg=response write failed and failure_stage body or newline (never silent).
//
// Successful responses never publish SSE events.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.list")
	const op = "tasks.list"
	r = calltrace.WithRequestRoot(r, op)
	q := r.URL.Query()
	limit, offset, afterID, err := parseListParams(r.Context(), q)
	if err != nil {
		debugHTTPRequest(r, op, "list_params_invalid", true)
		writeStoreError(w, r, op, err,
			"failure_stage", "parse_list_params",
			"limit_q", q.Get("limit"),
			"offset_q", q.Get("offset"),
			"after_id_q", q.Get("after_id"),
		)
		return
	}
	debugHTTPRequest(r, op, "limit", limit, "offset", offset, "after_id", afterID)
	var tasks []domain.Task
	var hasMore bool
	if afterID != "" {
		tasks, hasMore, err = h.store.ListFlatAfter(r.Context(), limit, afterID)
		offset = 0
	} else {
		tasks, hasMore, err = h.store.ListFlatPage(r.Context(), limit, offset, nil)
	}
	if err != nil {
		mode := "offset"
		if afterID != "" {
			mode = "keyset"
		}
		writeStoreError(w, r, op, err,
			"failure_stage", "store_list",
			"limit", limit,
			"offset", offset,
			"after_id", afterID,
			"pagination_mode", mode,
		)
		return
	}
	writeJSONWithETag(w, r, op, http.StatusOK, buildListResponse(tasks, limit, offset, hasMore))
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.stats")
	const op = "tasks.stats"
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)
	stats, err := h.store.TaskStats(r.Context())
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSONWithETag(w, r, op, http.StatusOK, taskStatsResponseFromStore(stats))
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patch")
	const op = "tasks.patch"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body taskPatchJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "task_id", id, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	debugHTTPRequest(r, op, append(append([]any{}, "task_id", id), taskPatchInputFields(&body)...)...)
	var dependsOnPatch *[]domain.DependencyEdge
	if body.DependsOn != nil && body.DependsOn.set {
		dependsOnPatch = &body.DependsOn.value
	}
	in := store.UpdateTaskInput{
		Title:                 body.Title,
		InitialPrompt:         body.InitialPrompt,
		Status:                body.Status,
		Priority:              body.Priority,
		Project:               projectFieldPatchToStore(body.ProjectID),
		ProjectContextItemIDs: body.ProjectContextItemIDs,
		PickupNotBefore:       pickupNotBeforePatchToStore(body.PickupNotBefore),
		CursorModel:           body.CursorModel,
		Tags:                  body.Tags,
		Milestone:             body.Milestone,
		Gate:                  gateFieldPatchToStore(body.Gate),
		DependsOn:             dependsOnPatch,
	}
	if body.InitialPrompt != nil {
		if err := h.validatePromptMentionsIfRepo(r, *body.InitialPrompt); err != nil {
			writeStoreError(w, r, op, err)
			return
		}
	}
	by := actorFromRequest(r)
	_, err = h.store.Update(r.Context(), id, in, by)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	task, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyTaskChanged(TaskUpdated, id, task)
	if body.Gate.Defined {
		h.notifyChange(TaskGateChanged, id)
	}
	if body.DependsOn != nil && body.DependsOn.set {
		h.notifyChange(TaskDependencyChanged, id)
	}
	taskapiDomainTasksUpdatedTotal.Inc()
	writeJSON(w, r, op, http.StatusOK, task)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.delete")
	const op = "tasks.delete"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "task_id", id)
	by := actorFromRequest(r)
	deletedIDs, err := h.store.Delete(r.Context(), id, by)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	for _, deletedID := range deletedIDs {
		h.notifyChange(TaskDeleted, deletedID)
		taskapiDomainTasksDeletedTotal.Inc()
	}
	debugHTTPOut(r.Context(), op, http.StatusNoContent,
		"task_id", id,
		"deleted_count", len(deletedIDs),
		"response_empty", true)
	w.WriteHeader(http.StatusNoContent)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func projectFieldPatchToStore(p patchProjectField) *store.ProjectFieldPatch {
	if !p.Defined {
		return nil
	}
	if p.Clear {
		return &store.ProjectFieldPatch{Clear: true}
	}
	return &store.ProjectFieldPatch{ID: p.SetID}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func gateFieldPatchToStore(p patchGateField) **domain.TaskGate {
	if !p.Defined {
		return nil
	}
	if p.Clear {
		var cleared *domain.TaskGate
		return &cleared
	}
	return &p.Set
}

func parseListParams(ctx context.Context, q url.Values) (limit, offset int, afterID string, err error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseListParams")
	ctx = calltrace.Push(ctx, "parseListParams")
	calltrace.HelperIOIn(ctx, "parseListParams", "limit_q", q.Get("limit"), "offset_q", q.Get("offset"), "after_id_q", q.Get("after_id"))
	defer func() {
		calltrace.HelperIOOut(ctx, "parseListParams", "limit", limit, "offset", offset, "after_id", afterID, "err", err)
	}()
	limit = 50
	offset = 0
	afterID = strings.TrimSpace(q.Get("after_id"))
	if afterID != "" && len(afterID) > maxListAfterIDParamBytes {
		return 0, 0, "", fmt.Errorf("%w: after_id too long", domain.ErrInvalidInput)
	}
	if _, ok := q["offset"]; ok && afterID != "" {
		return 0, 0, "", fmt.Errorf("%w: offset cannot be used with after_id", domain.ErrInvalidInput)
	}
	if afterID != "" {
		if _, perr := uuid.Parse(afterID); perr != nil {
			return 0, 0, "", fmt.Errorf("%w: after_id must be a UUID", domain.ErrInvalidInput)
		}
	}
	if v := q.Get("limit"); v != "" {
		if len(v) > maxListIntQueryParamBytes {
			return 0, 0, "", fmt.Errorf("%w: limit value too long", domain.ErrInvalidInput)
		}
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 || n > 200 {
			return 0, 0, "", fmt.Errorf("%w: limit must be integer 0..200", domain.ErrInvalidInput)
		}
		limit = n
	}
	if limit <= 0 {
		limit = 50
	}
	if v := q.Get("offset"); v != "" {
		if len(v) > maxListIntQueryParamBytes {
			return 0, 0, "", fmt.Errorf("%w: offset value too long", domain.ErrInvalidInput)
		}
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 {
			return 0, 0, "", fmt.Errorf("%w: offset must be non-negative integer", domain.ErrInvalidInput)
		}
		offset = n
	}
	return limit, offset, afterID, nil
}

func (h *Handler) validatePromptMentionsIfRepo(r *http.Request, prompt string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.validatePromptMentionsIfRepo")
	if h.repoProv == nil {
		return nil
	}
	root, _, err := h.repoProv.Repo(r.Context())
	if err != nil || root == nil {
		return nil
	}
	return root.ValidatePromptMentions(prompt)
}
