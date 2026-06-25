package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/repo"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

type workspaceRootsResponse struct {
	Roots       []repo.BrowseRoot      `json:"roots"`
	Environment repo.BrowseEnvironment `json:"environment"`
}

type browseDirsResponse struct {
	Path       string                `json:"path,omitempty"`
	ParentPath string                `json:"parent_path,omitempty"`
	IsGitRepo  bool                  `json:"is_git_repo,omitempty"`
	Entries    []repo.BrowseDirEntry `json:"entries"`
}

func (h *Handler) workspaceRoots(w http.ResponseWriter, r *http.Request) {
	const op = "settings.workspace_roots"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.workspaceRoots")
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)
	if r.Method != http.MethodGet {
		writeError(w, r, op, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	wd, err := os.Getwd()
	if err != nil {
		writeJSONError(w, r, op, http.StatusInternalServerError, "working directory unavailable")
		return
	}
	gitRepos, err := h.store.ListAllGitRepositories(r.Context())
	if err != nil {
		slog.Log(r.Context(), slog.LevelError, "list git repositories failed",
			"cmd", calltrace.LogCmd, "operation", op, "err", err)
		writeJSONError(w, r, op, http.StatusInternalServerError, "workspace roots unavailable")
		return
	}
	roots, env, err := repo.ResolveWorkspacePickerRoots(wd, gitRepos)
	if err != nil {
		slog.Log(r.Context(), slog.LevelError, "workspace roots failed",
			"cmd", calltrace.LogCmd, "operation", op, "err", err)
		writeJSONError(w, r, op, http.StatusInternalServerError, "browse roots unavailable")
		return
	}
	writeJSON(w, r, op, http.StatusOK, workspaceRootsResponse{Roots: roots, Environment: env})
}

func (h *Handler) browseDirs(w http.ResponseWriter, r *http.Request) {
	const op = "settings.browse_dirs"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.browseDirs")
	r = calltrace.WithRequestRoot(r, op)
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	debugHTTPRequest(r, op, "browse_path", truncateRunes(path, maxHTTPLogTitleRunes))
	if r.Method != http.MethodGet {
		writeError(w, r, op, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	if len(path) > maxRepoRelPathQueryBytes {
		writeJSONError(w, r, op, http.StatusBadRequest, "path too long")
		return
	}

	var listing repo.BrowseDirListing
	var listErr error
	if repo.CustomBrowseRootsConfigured() {
		// Ops override: constrain browsing to the configured roots.
		wd, err := os.Getwd()
		if err != nil {
			writeJSONError(w, r, op, http.StatusInternalServerError, "working directory unavailable")
			return
		}
		roots, _, err := repo.ResolveBrowseRoots(wd)
		if err != nil {
			writeJSONError(w, r, op, http.StatusInternalServerError, "browse roots unavailable")
			return
		}
		listing, listErr = repo.ListBrowseDirs(roots, path)
	} else {
		// Full-disk browse for register-repo bootstrap: no containment restriction.
		listing, listErr = repo.ListBrowseDirsUnrestricted(path)
	}
	if listErr != nil {
		if errors.Is(listErr, domain.ErrInvalidInput) {
			writeJSONError(w, r, op, http.StatusBadRequest, repoErrUserMessage(listErr))
			return
		}
		slog.Log(r.Context(), slog.LevelError, "browse dirs failed",
			"cmd", calltrace.LogCmd, "operation", op, "err", listErr)
		writeJSONError(w, r, op, http.StatusInternalServerError, "browse failed")
		return
	}
	writeJSON(w, r, op, http.StatusOK, browseDirsResponse{
		Path:       listing.Path,
		ParentPath: listing.ParentPath,
		IsGitRepo:  listing.IsGitRepo,
		Entries:    listing.Entries,
	})
}
