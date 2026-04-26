package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/google/uuid"
)

const (
	maxListIntQueryParamBytes = 32
	maxListAfterIDParamBytes  = 128
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
	if err := h.validatePromptMentionsIfRepo(r, body.InitialPrompt); err != nil {
		// repo.ValidatePromptMentions wraps every reject with
		// domain.ErrInvalidInput (via repo.wrapMention / wrapMentionMsg),
		// so route through writeStoreError to launder the
		// "tasks: invalid input: " prefix the same way every other
		// store-side error path does. Going through writeJSONError
		// with err.Error() leaked that internal prefix into the SPA
		// "fix the mention" banner — see
		// TestHTTP_repo_search_and_create_rejects_bad_file_mention.
		writeStoreError(w, r, op, err)
		return
	}
	inherit := false
	if body.ChecklistInherit != nil {
		inherit = *body.ChecklistInherit
	}
	settings, err := h.store.GetSettings(r.Context())
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	runner, cursorModel, err := resolveTaskRunnerModel(&body, settings)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	pickupNotBefore, err := resolvePickupNotBeforeForCreate(body.PickupNotBefore, body.Status, settings)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	t, err := h.store.Create(r.Context(), store.CreateTaskInput{
		ID:                    body.ID,
		DraftID:               body.DraftID,
		Title:                 body.Title,
		InitialPrompt:         body.InitialPrompt,
		Status:                body.Status,
		Priority:              body.Priority,
		TaskType:              body.TaskType,
		ProjectID:             body.ProjectID,
		ProjectContextItemIDs: body.ProjectContextItemIDs,
		ParentID:              body.ParentID,
		ChecklistInherit:      inherit,
		Runner:                runner,
		CursorModel:           cursorModel,
		PickupNotBefore:       pickupNotBefore,
	}, by)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	tree, err := h.store.GetTaskTree(r.Context(), t.ID)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskCreated, t.ID)
	if t.ParentID != nil && *t.ParentID != "" {
		h.notifyChange(TaskUpdated, *t.ParentID)
	}
	taskapiDomainTasksCreatedTotal.Inc()
	writeJSON(w, r, op, http.StatusCreated, tree)
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
	t, err := h.store.GetTaskTree(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, t)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.list")
	const op = "tasks.list"
	r = calltrace.WithRequestRoot(r, op)
	limit, offset, afterID, err := parseListParams(r.Context(), r.URL.Query())
	if err != nil {
		debugHTTPRequest(r, op, "list_params_invalid", true)
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "limit", limit, "offset", offset, "after_id", afterID)
	var tasks []store.TaskNode
	var hasMore bool
	if afterID != "" {
		tasks, hasMore, err = h.store.ListRootForestAfter(r.Context(), limit, afterID)
		offset = 0
	} else {
		tasks, hasMore, err = h.store.ListRootForest(r.Context(), limit, offset)
	}
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, listResponse{Tasks: tasks, Limit: limit, Offset: offset, HasMore: hasMore})
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
	writeJSON(w, r, op, http.StatusOK, taskStatsResponseFromStore(stats))
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
	in := store.UpdateTaskInput{
		Title:                 body.Title,
		InitialPrompt:         body.InitialPrompt,
		Status:                body.Status,
		Priority:              body.Priority,
		TaskType:              body.TaskType,
		Project:               projectFieldPatchToStore(body.ProjectID),
		ProjectContextItemIDs: body.ProjectContextItemIDs,
		ChecklistInherit:      body.ChecklistInherit,
		PickupNotBefore:       pickupNotBeforePatchToStore(body.PickupNotBefore),
		CursorModel:           body.CursorModel,
	}
	if body.ParentID.Defined {
		if body.ParentID.Clear {
			in.Parent = &store.ParentFieldPatch{Clear: true}
		} else {
			in.Parent = &store.ParentFieldPatch{ID: body.ParentID.SetID}
		}
	}
	if body.InitialPrompt != nil {
		if err := h.validatePromptMentionsIfRepo(r, *body.InitialPrompt); err != nil {
			// Same prefix-leak symmetry as POST /tasks above:
			// validatePromptMentionsIfRepo's wrap targets
			// domain.ErrInvalidInput, so writeStoreError gives the
			// same clean "mention @path: file does not exist" wire
			// shape clients already see for the create flow.
			writeStoreError(w, r, op, err)
			return
		}
	}
	by := actorFromRequest(r)
	if _, err := h.store.Update(r.Context(), id, in, by); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	tree, err := h.store.GetTaskTree(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskUpdated, id)
	taskapiDomainTasksUpdatedTotal.Inc()
	writeJSON(w, r, op, http.StatusOK, tree)
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
	deletedIDs, parentNotify, err := h.store.Delete(r.Context(), id, by)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	for _, deletedID := range deletedIDs {
		h.notifyChange(TaskDeleted, deletedID)
		taskapiDomainTasksDeletedTotal.Inc()
	}
	if parentNotify != "" {
		h.notifyChange(TaskUpdated, parentNotify)
	}
	debugHTTPOut(r.Context(), op, http.StatusNoContent,
		"task_id", id,
		"deleted_count", len(deletedIDs),
		"response_empty", true)
	w.WriteHeader(http.StatusNoContent)
}

func projectFieldPatchToStore(p patchProjectField) *store.ProjectFieldPatch {
	if !p.Defined {
		return nil
	}
	if p.Clear {
		return &store.ProjectFieldPatch{Clear: true}
	}
	return &store.ProjectFieldPatch{ID: p.SetID}
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

// validatePromptMentionsIfRepo runs ValidatePromptMentions only when
// the active workspace can be opened. When the repo root is not yet
// configured (or fails to open) we skip validation so task creation
// keeps working from an empty install; the agent worker enforces the
// same check at run-time once the SPA Settings page wires a root.
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
