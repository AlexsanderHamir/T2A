package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Handler) gitRepositoryJSON(r domain.GitRepository) gitRepositoryJSON {
	return gitRepositoryJSON{
		ID:            r.ID,
		Path:          r.Path,
		GitCommonDir:  r.GitCommonDir,
		HostPath:      h.pathMap.DisplayHostPath(r.Path),
		DefaultBranch: r.DefaultBranch,
		CreatedAt:     r.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     r.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (h *Handler) listGitRepositories(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.list"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.listGitRepositories")
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
		out = append(out, h.gitRepositoryJSON(row))
	}
	writeJSON(w, r, op, http.StatusOK, gitRepositoriesListResponse{Repositories: out})
}

func (h *Handler) createGitRepository(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.create"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.createGitRepository")
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
	writeJSON(w, r, op, http.StatusCreated, h.gitRepositoryJSON(repo))
}

func (h *Handler) getGitRepository(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.get"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.getGitRepository")
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
	writeJSON(w, r, op, http.StatusOK, h.gitRepositoryJSON(repo))
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
