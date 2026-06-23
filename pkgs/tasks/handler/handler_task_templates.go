package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func (h *Handler) listTaskTemplates(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.listTaskTemplates")
	const op = "task_templates.list"
	r = calltrace.WithRequestRoot(r, op)
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if len(raw) > maxListIntQueryParamBytes {
			writeJSONError(w, r, op, http.StatusBadRequest, "limit value too long")
			return
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > 100 {
			writeJSONError(w, r, op, http.StatusBadRequest, "limit must be integer 0..100")
			return
		}
		limit = n
	}
	if limit <= 0 {
		limit = 50
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	rows, err := h.store.ListTemplates(r.Context(), limit, q)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, map[string]any{"templates": rows})
}

func (h *Handler) saveTaskTemplate(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.saveTaskTemplate")
	const op = "task_templates.save"
	r = calltrace.WithRequestRoot(r, op)
	var body taskTemplateSaveJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	compose, err := decodeComposePayload(body.Payload)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	settings, err := h.store.GetSettings(r.Context())
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if err := h.validateComposePayload(r.Context(), compose, settings); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = strings.TrimSpace(compose.Title)
	}
	payloadRaw, err := composePayloadToRaw(compose)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	saved, err := h.store.SaveTemplate(r.Context(), body.ID, name, payloadRaw)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusCreated, saved)
}

func (h *Handler) getTaskTemplate(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.getTaskTemplate")
	const op = "task_templates.get"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	row, err := h.store.GetTemplate(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, row)
}

func (h *Handler) patchTaskTemplate(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patchTaskTemplate")
	const op = "task_templates.patch"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body taskTemplatePatchJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	var payloadRaw json.RawMessage
	if len(body.Payload) > 0 {
		compose, derr := decodeComposePayload(body.Payload)
		if derr != nil {
			writeStoreError(w, r, op, derr)
			return
		}
		settings, serr := h.store.GetSettings(r.Context())
		if serr != nil {
			writeStoreError(w, r, op, serr)
			return
		}
		if err := h.validateComposePayload(r.Context(), compose, settings); err != nil {
			writeStoreError(w, r, op, err)
			return
		}
		payloadRaw, err = composePayloadToRaw(compose)
		if err != nil {
			writeStoreError(w, r, op, err)
			return
		}
	}
	name := body.Name
	if name == nil && payloadRaw == nil {
		writeStoreError(w, r, op, fmt.Errorf("%w: no fields to update", domain.ErrInvalidInput))
		return
	}
	updated, err := h.store.PatchTemplate(r.Context(), id, name, payloadRaw)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, updated)
}

func (h *Handler) deleteTaskTemplate(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.deleteTaskTemplate")
	const op = "task_templates.delete"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if err := h.store.DeleteTemplate(r.Context(), id); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPOut(r.Context(), op, http.StatusNoContent, "template_id", id, "response_empty", true)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) instantiateTaskTemplates(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.instantiateTaskTemplates")
	const op = "task_templates.instantiate"
	r = calltrace.WithRequestRoot(r, op)
	var body taskTemplateInstantiateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	items, err := normalizeInstantiateItems(body)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	by := actorFromRequest(r)
	resp := taskTemplateInstantiateResponseJSON{
		Tasks:  make([]domain.Task, 0),
		Errors: make([]taskTemplateInstantiateErrorJSON, 0),
	}
	for _, item := range items {
		for range item.Count {
			detail, err := h.store.GetTemplate(r.Context(), item.TemplateID)
			if err != nil {
				resp.Errors = append(resp.Errors, taskTemplateInstantiateErrorJSON{
					TemplateID: item.TemplateID,
					Error:      err.Error(),
				})
				continue
			}
			compose, err := decodeComposePayload(detail.Payload)
			if err != nil {
				resp.Errors = append(resp.Errors, taskTemplateInstantiateErrorJSON{
					TemplateID: item.TemplateID,
					Error:      err.Error(),
				})
				continue
			}
			task, err := h.createTaskFromComposeJSON(r.Context(), r, op, compose, createTaskComposeOpts{
				StripDependsOn:          true,
				OmitPastPickupNotBefore: true,
				InstantiateFromTemplate: true,
			}, by)
			if err != nil {
				resp.Errors = append(resp.Errors, taskTemplateInstantiateErrorJSON{
					TemplateID: item.TemplateID,
					Error:      err.Error(),
				})
				continue
			}
			resp.Tasks = append(resp.Tasks, *task)
		}
	}
	writeJSON(w, r, op, http.StatusOK, resp)
}
