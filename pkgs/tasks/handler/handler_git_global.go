package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func (h *Handler) worktreeBranchJSON(wb domain.WorktreeBranch) worktreeBranchJSON {
	return worktreeBranchJSON{
		ID:         wb.ID,
		WorktreeID: wb.WorktreeID,
		BranchID:   wb.BranchID,
		CreatedAt:  wb.CreatedAt.UTC().Format(time.RFC3339),
	}
}

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
	wt, err := h.store.RegisterExistingGitWorktree(r.Context(), r.PathValue("repoId"), body.Path, body.Name, h.gitService())
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

func (h *Handler) listWorktreeBranchAssociations(w http.ResponseWriter, r *http.Request) {
	const op = "git.worktree_branches.list"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.listWorktreeBranchAssociations")
	r = calltrace.WithRequestRoot(r, op)
	rows, err := h.store.ListWorktreeBranches(r.Context(), r.PathValue("worktreeId"))
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	out := make([]worktreeBranchJSON, 0, len(rows))
	for _, row := range rows {
		out = append(out, h.worktreeBranchJSON(row))
	}
	writeJSON(w, r, op, http.StatusOK, gitWorktreeBranchesListResponse{Associations: out})
}

func (h *Handler) associateWorktreeBranch(w http.ResponseWriter, r *http.Request) {
	const op = "git.worktree_branches.associate"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.associateWorktreeBranch")
	r = calltrace.WithRequestRoot(r, op)
	worktreeID := r.PathValue("worktreeId")
	var body gitWorktreeBranchAssociateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	branchID := strings.TrimSpace(body.BranchID)
	if branchID == "" && strings.TrimSpace(body.Name) != "" {
		wt, err := h.store.GetGitWorktreeByID(r.Context(), worktreeID)
		if err != nil {
			writeGitStoreError(w, r, op, err)
			return
		}
		br, err := h.store.CreateGitBranchForRepo(r.Context(), wt.RepositoryID, store.CreateGitBranchInput{
			Name:       body.Name,
			StartPoint: body.StartPoint,
		}, h.gitService())
		if err != nil {
			writeGitStoreError(w, r, op, err)
			return
		}
		branchID = br.ID
	}
	if branchID == "" {
		writeError(w, r, op, domain.ErrInvalidInput, http.StatusBadRequest)
		return
	}
	wb, err := h.store.AssociateWorktreeBranch(r.Context(), store.AssociateWorktreeBranchInput{
		WorktreeID: worktreeID,
		BranchID:   branchID,
	})
	if err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusCreated, h.worktreeBranchJSON(wb))
}

func (h *Handler) removeWorktreeBranchAssociation(w http.ResponseWriter, r *http.Request) {
	const op = "git.worktree_branches.remove"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.removeWorktreeBranchAssociation")
	r = calltrace.WithRequestRoot(r, op)
	if err := h.store.RemoveWorktreeBranch(r.Context(), r.PathValue("worktreeId"), r.PathValue("branchId")); err != nil {
		writeGitStoreError(w, r, op, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
