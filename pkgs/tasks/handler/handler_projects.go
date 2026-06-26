package handler

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func (h *Handler) createProject(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.createProject")
	const op = "projects.create"
	r = calltrace.WithRequestRoot(r, op)
	var body projectCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	project, err := h.store.CreateProject(r.Context(), store.CreateProjectInput{
		ID:             body.ID,
		Name:           body.Name,
		Description:    body.Description,
		ContextSummary: body.ContextSummary,
		RepositoryID:   body.RepositoryID,
	})
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(ProjectCreated, project.ID)
	writeJSON(w, r, op, http.StatusCreated, project)
}

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.listProjects")
	const op = "projects.list"
	r = calltrace.WithRequestRoot(r, op)
	limit, includeArchived, err := parseProjectListParams(r.URL.Query())
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	projects, err := h.store.ListProjects(r.Context(), includeArchived, limit)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSONWithETag(w, r, op, http.StatusOK, projectsListResponse{Projects: projects, Limit: limit})
}

func (h *Handler) getProject(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.getProject")
	const op = "projects.get"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	project, err := h.store.GetProject(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSONWithETag(w, r, op, http.StatusOK, project)
}

func (h *Handler) patchProject(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patchProject")
	const op = "projects.patch"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body projectPatchJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	if body.isEmpty() {
		writeStoreError(w, r, op, fmt.Errorf("%w: no fields to update", domain.ErrInvalidInput))
		return
	}
	project, err := h.store.UpdateProject(r.Context(), id, store.UpdateProjectInput{
		Name:           body.Name,
		Description:    body.Description,
		Status:         body.Status,
		ContextSummary: body.ContextSummary,
	})
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(ProjectUpdated, project.ID)
	writeJSON(w, r, op, http.StatusOK, project)
}

func (h *Handler) deleteProject(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.deleteProject")
	const op = "projects.delete"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if err := h.store.DeleteProject(r.Context(), id); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(ProjectDeleted, id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createProjectContext(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.createProjectContext")
	const op = "projects.context.create"
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body projectContextCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	item, err := h.store.CreateProjectContext(r.Context(), projectID, store.CreateProjectContextInput{
		ID:            body.ID,
		Kind:          body.Kind,
		Title:         body.Title,
		Body:          body.Body,
		SourceTaskID:  body.SourceTaskID,
		SourceCycleID: body.SourceCycleID,
		CreatedBy:     actorFromRequest(r),
		Pinned:        body.Pinned,
	})
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(ProjectContextChanged, projectID)
	writeJSON(w, r, op, http.StatusCreated, item)
}

func (h *Handler) listProjectContext(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.listProjectContext")
	const op = "projects.context.list"
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	limit, includeUnpinned, err := parseProjectContextListParams(r.URL.Query())
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	items, err := h.store.ListProjectContext(r.Context(), projectID, includeUnpinned, limit)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	itemIDs := make([]string, 0, len(items))
	for _, item := range items {
		itemIDs = append(itemIDs, item.ID)
	}
	edges, err := h.store.ListProjectContextEdges(r.Context(), projectID, itemIDs)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSONWithETag(w, r, op, http.StatusOK, projectContextListResponse{Items: items, Edges: edges, Limit: limit})
}

func (h *Handler) createProjectContextEdge(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.createProjectContextEdge")
	const op = "projects.context.edges.create"
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body projectContextEdgeCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	edge, err := h.store.CreateProjectContextEdge(r.Context(), projectID, store.CreateProjectContextEdgeInput{
		ID:              body.ID,
		SourceContextID: body.SourceContextID,
		TargetContextID: body.TargetContextID,
		Relation:        body.Relation,
		Strength:        body.Strength,
		Note:            body.Note,
	})
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(ProjectContextChanged, projectID)
	writeJSON(w, r, op, http.StatusCreated, edge)
}

func (h *Handler) patchProjectContextEdge(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patchProjectContextEdge")
	const op = "projects.context.edges.patch"
	r = calltrace.WithRequestRoot(r, op)
	projectID, edgeID, err := parseProjectContextEdgePath(r)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body projectContextEdgePatchJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	if body.isEmpty() {
		writeStoreError(w, r, op, fmt.Errorf("%w: no fields to update", domain.ErrInvalidInput))
		return
	}
	edge, err := h.store.UpdateProjectContextEdge(r.Context(), projectID, edgeID, store.UpdateProjectContextEdgeInput{
		Relation: body.Relation,
		Strength: body.Strength,
		Note:     body.Note,
	})
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(ProjectContextChanged, projectID)
	writeJSON(w, r, op, http.StatusOK, edge)
}

func (h *Handler) deleteProjectContextEdge(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.deleteProjectContextEdge")
	const op = "projects.context.edges.delete"
	r = calltrace.WithRequestRoot(r, op)
	projectID, edgeID, err := parseProjectContextEdgePath(r)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if err := h.store.DeleteProjectContextEdge(r.Context(), projectID, edgeID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(ProjectContextChanged, projectID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) patchProjectContext(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patchProjectContext")
	const op = "projects.context.patch"
	r = calltrace.WithRequestRoot(r, op)
	projectID, itemID, err := parseProjectContextPath(r)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body projectContextPatchJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	if body.isEmpty() {
		writeStoreError(w, r, op, fmt.Errorf("%w: no fields to update", domain.ErrInvalidInput))
		return
	}
	item, err := h.store.UpdateProjectContext(r.Context(), projectID, itemID, store.UpdateProjectContextInput{
		Kind:   body.Kind,
		Title:  body.Title,
		Body:   body.Body,
		Pinned: body.Pinned,
	})
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(ProjectContextChanged, projectID)
	writeJSON(w, r, op, http.StatusOK, item)
}

func (h *Handler) deleteProjectContext(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.deleteProjectContext")
	const op = "projects.context.delete"
	r = calltrace.WithRequestRoot(r, op)
	projectID, itemID, err := parseProjectContextPath(r)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if err := h.store.DeleteProjectContext(r.Context(), projectID, itemID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(ProjectContextChanged, projectID)
	w.WriteHeader(http.StatusNoContent)
}
