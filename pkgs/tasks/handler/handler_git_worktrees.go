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
func (h *Handler) gitWorktreeJSON(w domain.GitWorktree) gitWorktreeJSON {
	return gitWorktreeJSON{
		ID:           w.ID,
		RepositoryID: w.RepositoryID,
		Path:         w.Path,
		HostPath:     h.pathMap.DisplayHostPath(w.Path),
		Name:           w.Name,
		IsMain:         w.IsMain,
		ActiveBranchID: w.ActiveBranchID,
		CreatedAt:      w.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (h *Handler) listGitWorktrees(w http.ResponseWriter, r *http.Request) {
	const op = "git.worktrees.list"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.listGitWorktrees")
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseGitProjectID(r)
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	rows, err := h.store.ListGitWorktrees(r.Context(), projectID, r.PathValue("repoId"))
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	out := make([]gitWorktreeJSON, 0, len(rows))
	for _, row := range rows {
		out = append(out, h.gitWorktreeJSON(row))
	}
	writeJSON(w, r, op, http.StatusOK, gitWorktreesListResponse{Worktrees: out})
}

func (h *Handler) createGitWorktree(w http.ResponseWriter, r *http.Request) {
	const op = "git.worktrees.create"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.createGitWorktree")
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseGitProjectID(r)
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	var body gitWorktreeCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	wt, err := h.store.CreateGitWorktree(r.Context(), projectID, r.PathValue("repoId"), store.CreateGitWorktreeInput{
		Path:         body.Path,
		Name:         body.Name,
		Branch:       body.Branch,
		CreateBranch: body.CreateBranch,
		StartPoint:   body.StartPoint,
		ForceRemove:  body.ForceRemove,
	}, h.gitService())
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusCreated, h.gitWorktreeJSON(wt))
}

func (h *Handler) deleteGitWorktree(w http.ResponseWriter, r *http.Request) {
	const op = "git.worktrees.delete"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.deleteGitWorktree")
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseGitProjectID(r)
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	force := r.URL.Query().Get("force") == "true"
	if err := h.store.DeleteGitWorktree(r.Context(), projectID, r.PathValue("worktreeId"), force, h.gitService()); err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
