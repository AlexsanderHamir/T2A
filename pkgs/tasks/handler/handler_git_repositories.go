package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func toGitRepositoryJSON(r domain.GitRepository) gitRepositoryJSON {
	return gitRepositoryJSON{
		ID:            r.ID,
		ProjectID:     r.ProjectID,
		Path:          r.Path,
		HostPath:      r.HostPath,
		DefaultBranch: r.DefaultBranch,
		CreatedAt:     r.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     r.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (h *Handler) listGitRepositories(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.list"
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseGitProjectID(r)
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	rows, err := h.store.ListGitRepositories(r.Context(), projectID)
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	out := make([]gitRepositoryJSON, 0, len(rows))
	for _, row := range rows {
		out = append(out, toGitRepositoryJSON(row))
	}
	writeJSON(w, r, op, http.StatusOK, gitRepositoriesListResponse{Repositories: out})
}

func (h *Handler) createGitRepository(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.create"
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseGitProjectID(r)
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	var body gitRepositoryCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	repo, err := h.store.CreateGitRepository(r.Context(), projectID, store.CreateGitRepositoryInput{
		Path:          body.Path,
		HostPath:      body.HostPath,
		DefaultBranch: body.DefaultBranch,
	}, h.gitService())
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusCreated, toGitRepositoryJSON(repo))
}

func (h *Handler) getGitRepository(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.get"
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseGitProjectID(r)
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	repoID := r.PathValue("repoId")
	repo, err := h.store.GetGitRepository(r.Context(), projectID, repoID)
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, toGitRepositoryJSON(repo))
}

func (h *Handler) deleteGitRepository(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.delete"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.deleteGitRepository")
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseGitProjectID(r)
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	if err := h.store.DeleteGitRepository(r.Context(), projectID, r.PathValue("repoId")); err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) reconcileGitRepository(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.reconcile"
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseGitProjectID(r)
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	repoID := r.PathValue("repoId")
	if err := h.store.ReconcileGitRepository(r.Context(), projectID, repoID, h.gitService()); err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusAccepted, gitReconcileResponse{Status: "ok"})
}
