package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

//funclogmeasure:skip category=hot-path reason="Pure request parser without I/O; operation trace is emitted by reconcile handlers."
func parseGitReconcileRequest(r *http.Request) (store.ReconcileGitInput, error) {
	var body gitReconcileRequest
	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&body); err != nil && err != io.EOF {
			return store.ReconcileGitInput{}, err
		}
	}
	return store.ReconcileGitInput{
		BootstrapPath: strings.TrimSpace(body.BootstrapPath),
		RepairGit:     body.Repair,
		DryRun:        body.DryRun,
		AllowRemove:   true,
		AllowDiscover: false,
	}, nil
}

//funclogmeasure:skip category=delegate-already-logs reason="Shared reconcile handler; public routes emit trace before calling this helper."
func (h *Handler) runGitReconcile(w http.ResponseWriter, r *http.Request, op, projectID, repoID string) {
	input, err := parseGitReconcileRequest(r)
	if err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	out, err := h.store.ReconcileGitRepository(r.Context(), projectID, repoID, input, h.gitService())
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusAccepted, toGitReconcileResponse(out))
}

func (h *Handler) reconcileGitRepository(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.reconcile"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.reconcileGitRepository")
	r = calltrace.WithRequestRoot(r, op)
	projectID, err := parseGitProjectID(r)
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	h.runGitReconcile(w, r, op, projectID, r.PathValue("repoId"))
}

func (h *Handler) reconcileGlobalGitRepository(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.reconcile_global"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.reconcileGlobalGitRepository")
	r = calltrace.WithRequestRoot(r, op)
	h.runGitReconcile(w, r, op, "", r.PathValue("repoId"))
}

func (h *Handler) relocateGlobalGitRepository(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.relocate_global"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.relocateGlobalGitRepository")
	r = calltrace.WithRequestRoot(r, op)
	repoID := r.PathValue("repoId")
	var body gitRelocateRepositoryRequest
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	out, err := h.store.RelocateGitRepository(r.Context(), "", repoID, body.Path, h.gitService())
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusAccepted, toGitReconcileResponse(out))
}

func (h *Handler) relocateGlobalGitWorktree(w http.ResponseWriter, r *http.Request) {
	const op = "git.worktrees.relocate_global"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.relocateGlobalGitWorktree")
	r = calltrace.WithRequestRoot(r, op)
	var body gitRelocateWorktreeRequest
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	wt, err := h.store.RelocateGitWorktree(r.Context(), r.PathValue("worktreeId"), body.Path, h.gitService())
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, h.gitWorktreeJSON(wt))
}
