package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func (h *Handler) listGlobalGitRepositories(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.list_global"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.listGlobalGitRepositories")
	r = calltrace.WithRequestRoot(r, op)
	rows, err := h.store.ListAllGitRepositories(r.Context())
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

func (h *Handler) createGlobalGitRepository(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.create_global"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.createGlobalGitRepository")
	r = calltrace.WithRequestRoot(r, op)
	var body gitRepositoryCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	repo, err := h.store.CreateGlobalGitRepository(r.Context(), store.CreateGitRepositoryInput{
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

func (h *Handler) getGlobalGitRepository(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.get_global"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.getGlobalGitRepository")
	r = calltrace.WithRequestRoot(r, op)
	repo, err := h.store.GetGitRepositoryByID(r.Context(), r.PathValue("repoId"))
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, h.gitRepositoryJSON(repo))
}

func (h *Handler) deleteGlobalGitRepository(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.delete_global"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.deleteGlobalGitRepository")
	r = calltrace.WithRequestRoot(r, op)
	if err := h.store.DeleteGlobalGitRepository(r.Context(), r.PathValue("repoId")); err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listGlobalGitWorktrees(w http.ResponseWriter, r *http.Request) {
	const op = "git.worktrees.list_global"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.listGlobalGitWorktrees")
	r = calltrace.WithRequestRoot(r, op)
	rows, err := h.store.ListGitWorktreesByRepo(r.Context(), r.PathValue("repoId"))
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

func (h *Handler) listGlobalGitWorktreesLive(w http.ResponseWriter, r *http.Request) {
	const op = "git.worktrees.list_live"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.listGlobalGitWorktreesLive")
	r = calltrace.WithRequestRoot(r, op)
	repo, err := h.store.GetGitRepositoryByID(r.Context(), r.PathValue("repoId"))
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	inventory, err := h.store.RepoWorktreeInventory(r.Context(), repo, h.gitService())
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	out := make([]gitLiveWorktreeJSON, 0, len(inventory))
	for _, wt := range inventory {
		out = append(out, gitLiveWorktreeJSON{
			Path:       wt.Path,
			Branch:     wt.Branch,
			IsMain:     wt.IsMain,
			Detached:   wt.Detached,
			Registered: wt.Registered,
		})
	}
	writeJSON(w, r, op, http.StatusOK, gitLiveWorktreesListResponse{Worktrees: out})
}

func (h *Handler) probeGlobalGitWorktree(w http.ResponseWriter, r *http.Request) {
	const op = "git.worktrees.probe"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.probeGlobalGitWorktree")
	r = calltrace.WithRequestRoot(r, op)
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		writeJSONError(w, r, op, http.StatusBadRequest, "path required")
		return
	}
	result, err := h.store.ProbeGitWorktree(r.Context(), r.PathValue("repoId"), path, h.gitService())
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, gitWorktreeProbeResponse{
		Path:       result.Path,
		Linked:     result.Linked,
		IsMain:     result.IsMain,
		Branch:     result.Branch,
		Registered: result.Registered,
	})
}

func (h *Handler) createGlobalGitWorktree(w http.ResponseWriter, r *http.Request) {
	const op = "git.worktrees.create_global"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.createGlobalGitWorktree")
	r = calltrace.WithRequestRoot(r, op)
	var body gitWorktreeCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	wt, err := h.store.CreateGitWorktreeForRepo(r.Context(), r.PathValue("repoId"), store.CreateGitWorktreeInput{
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

func (h *Handler) registerGlobalGitWorktree(w http.ResponseWriter, r *http.Request) {
	const op = "git.worktrees.register_global"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.registerGlobalGitWorktree")
	r = calltrace.WithRequestRoot(r, op)
	var body gitWorktreeRegisterJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	var bind store.BindBranchInput
	if body.Branch != nil {
		bind = store.BindBranchInput{
			Name:         body.Branch.Name,
			CreateBranch: body.Branch.CreateBranch,
			StartPoint:   body.Branch.StartPoint,
		}
	}
	wt, err := h.store.RegisterExistingGitWorktree(r.Context(), r.PathValue("repoId"), body.Path, body.Name, bind, h.gitService())
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusCreated, h.gitWorktreeJSON(wt))
}

func (h *Handler) deleteGlobalGitWorktree(w http.ResponseWriter, r *http.Request) {
	const op = "git.worktrees.delete_global"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.deleteGlobalGitWorktree")
	r = calltrace.WithRequestRoot(r, op)
	force := r.URL.Query().Get("force") == "true"
	if err := h.store.DeleteGitWorktreeByID(r.Context(), r.PathValue("worktreeId"), force, h.gitService()); err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listGlobalGitBranches(w http.ResponseWriter, r *http.Request) {
	const op = "git.branches.list_global"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.listGlobalGitBranches")
	r = calltrace.WithRequestRoot(r, op)
	rows, err := h.store.ListGitBranchesByRepo(r.Context(), r.PathValue("repoId"))
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	out := make([]gitBranchJSON, 0, len(rows))
	for _, row := range rows {
		out = append(out, toGitBranchJSON(row))
	}
	writeJSON(w, r, op, http.StatusOK, gitBranchesListResponse{Branches: out})
}

func (h *Handler) listGlobalGitBranchesLive(w http.ResponseWriter, r *http.Request) {
	const op = "git.branches.list_live"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.listGlobalGitBranchesLive")
	r = calltrace.WithRequestRoot(r, op)
	repo, err := h.store.GetGitRepositoryByID(r.Context(), r.PathValue("repoId"))
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	gitSvc := h.gitService()
	opened, err := gitSvc.OpenRepository(r.Context(), repo.Path)
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	live, err := gitSvc.ListBranches(r.Context(), opened)
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	out := make([]gitLiveBranchJSON, 0, len(live))
	for _, b := range live {
		out = append(out, gitLiveBranchJSON{Name: b.Name, HeadSHA: b.HeadSHA})
	}
	writeJSON(w, r, op, http.StatusOK, gitLiveBranchesListResponse{Branches: out})
}

func (h *Handler) listRepoProjects(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.projects.list"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.listRepoProjects")
	r = calltrace.WithRequestRoot(r, op)
	rows, err := h.store.ListProjectsByRepository(r.Context(), r.PathValue("repoId"))
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, projectsListResponse{Projects: rows, Limit: len(rows)})
}

func (h *Handler) reconcileGlobalGitRepository(w http.ResponseWriter, r *http.Request) {
	const op = "git.repositories.reconcile_global"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.reconcileGlobalGitRepository")
	r = calltrace.WithRequestRoot(r, op)
	repoID := r.PathValue("repoId")
	if err := h.store.ReconcileGitRepository(r.Context(), "", repoID, h.gitService()); err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusAccepted, gitReconcileResponse{Status: "ok"})
}
