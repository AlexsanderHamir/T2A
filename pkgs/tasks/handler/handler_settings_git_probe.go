package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

type gitRepositoryProbeResponse struct {
	Path            string              `json:"path"`
	IsGitRepository bool                `json:"is_git_repository"`
	CurrentBranch   string              `json:"current_branch,omitempty"`
	Branches        []gitLiveBranchJSON `json:"branches"`
}

func (h *Handler) gitRepositoryProbe(w http.ResponseWriter, r *http.Request) {
	const op = "settings.git_repository_probe"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.gitRepositoryProbe")
	r = calltrace.WithRequestRoot(r, op)
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	debugHTTPRequest(r, op, "probe_path", truncateRunes(path, maxHTTPLogTitleRunes))
	if r.Method != http.MethodGet {
		writeError(w, r, op, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	if path == "" {
		writeJSONError(w, r, op, http.StatusBadRequest, "path required")
		return
	}
	if len(path) > maxRepoRelPathQueryBytes {
		writeJSONError(w, r, op, http.StatusBadRequest, "path too long")
		return
	}

	gitSvc := h.gitService()
	opened, err := gitSvc.OpenRepository(r.Context(), path)
	if err != nil {
		if errors.Is(err, gitwork.ErrNotARepository) {
			writeJSON(w, r, op, http.StatusOK, gitRepositoryProbeResponse{
				Path:            path,
				IsGitRepository: false,
				Branches:        []gitLiveBranchJSON{},
			})
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			writeJSONError(w, r, op, http.StatusBadRequest, repoErrUserMessage(err))
			return
		}
		slog.Log(r.Context(), slog.LevelError, "git repository probe failed",
			"cmd", calltrace.LogCmd, "operation", op, "err", err)
		writeJSONError(w, r, op, http.StatusInternalServerError, "git probe failed")
		return
	}

	live, err := gitSvc.ListBranches(r.Context(), opened)
	if err != nil {
		slog.Log(r.Context(), slog.LevelError, "list branches for probe failed",
			"cmd", calltrace.LogCmd, "operation", op, "err", err)
		writeJSONError(w, r, op, http.StatusInternalServerError, "git probe failed")
		return
	}

	out := make([]gitLiveBranchJSON, 0, len(live))
	current := ""
	for _, b := range live {
		out = append(out, gitLiveBranchJSON{Name: b.Name, HeadSHA: b.HeadSHA})
		if b.IsCurrent {
			current = b.Name
		}
	}
	writeJSON(w, r, op, http.StatusOK, gitRepositoryProbeResponse{
		Path:            opened.Root,
		IsGitRepository: true,
		CurrentBranch:   current,
		Branches:        out,
	})
}
