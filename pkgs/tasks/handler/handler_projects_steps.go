package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func (h *Handler) listProjectSteps(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.listProjectSteps")
	const op = "projects.steps.list"
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	steps, err := h.store.ListProjectSteps(r.Context(), projectID)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, projectStepsListResponse{Steps: steps})
}

func (h *Handler) getProjectStep(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.getProjectStep")
	const op = "projects.steps.get"
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	stepID, err := parseTaskPathID(r.PathValue("stepId"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	step, err := h.store.GetProjectStep(r.Context(), projectID, stepID)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, step)
}

func (h *Handler) createProjectStep(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.createProjectStep")
	const op = "projects.steps.create"
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body projectStepCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	step, err := h.store.CreateProjectStep(r.Context(), projectID, store.CreateProjectStepInput{
		ID:          body.ID,
		Title:       body.Title,
		Description: body.Description,
		SortOrder:   body.SortOrder,
	})
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(ProjectStepCreated, projectID)
	writeJSON(w, r, op, http.StatusCreated, step)
}

func (h *Handler) patchProjectStep(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patchProjectStep")
	const op = "projects.steps.patch"
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	stepID, err := parseTaskPathID(r.PathValue("stepId"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body projectStepPatchJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	if body.isEmpty() {
		writeStoreError(w, r, op, fmt.Errorf("%w: no fields to update", domain.ErrInvalidInput))
		return
	}
	var gateAction *string
	if body.GateAction != nil {
		ga := strings.TrimSpace(*body.GateAction)
		if ga != "" {
			gateAction = &ga
		}
	}
	step, err := h.store.UpdateProjectStep(r.Context(), projectID, stepID, store.UpdateProjectStepInput{
		Title:       body.Title,
		Description: body.Description,
		SortOrder:   body.SortOrder,
		GateAction:  gateAction,
	})
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(ProjectStepUpdated, projectID)
	writeJSON(w, r, op, http.StatusOK, step)
}

func (h *Handler) deleteProjectStep(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.deleteProjectStep")
	const op = "projects.steps.delete"
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	stepID, err := parseTaskPathID(r.PathValue("stepId"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if err := h.store.DeleteProjectStep(r.Context(), projectID, stepID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(ProjectStepDeleted, projectID)
	w.WriteHeader(http.StatusNoContent)
}
