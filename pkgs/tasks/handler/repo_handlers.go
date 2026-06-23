package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitexec"
	"github.com/AlexsanderHamir/Hamix/pkgs/repo"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
)

type repoSearchResponse struct {
	Paths []string `json:"paths"`
}

type repoValidateRangeResponse struct {
	OK        bool   `json:"ok"`
	LineCount int    `json:"line_count,omitempty"`
	Warning   string `json:"warning,omitempty"`
}

type repoFileResponse struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Binary    bool   `json:"binary"`
	Truncated bool   `json:"truncated"`
	SizeBytes int64  `json:"size_bytes"`
	LineCount int    `json:"line_count"`
	Warning   string `json:"warning,omitempty"`
}

// Substring search cost scales with len(q); cap keeps pathological queries from burning CPU.
const maxRepoSearchQueryBytes = 512

// Repo-relative paths in query strings should stay within normal filesystem limits; huge values waste work in Resolve/logging.
const maxRepoRelPathQueryBytes = 4096

// Line numbers fit in a small decimal string; huge query values waste CPU in strconv and slog fields.
const maxRepoLineQueryParamBytes = 32

// Commit SHAs are short hex strings; cap query length to reject abuse.
const maxRepoShaQueryBytes = 64

var repoShaQueryPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)

type repoDiffResponse struct {
	SHA          string `json:"sha"`
	Patch        string `json:"patch"`
	Truncated    bool   `json:"truncated"`
	SizeBytes    int    `json:"size_bytes"`
	Author       string `json:"author,omitempty"`
	AuthorEmail  string `json:"author_email,omitempty"`
	ParentSHA    string `json:"parent_sha,omitempty"`
	FilesChanged int    `json:"files_changed,omitempty"`
	Insertions   int    `json:"insertions,omitempty"`
	Deletions    int    `json:"deletions,omitempty"`
}

// repoUnavailableErrorBody is the JSON envelope the SPA expects when
// a /repo/* call can't reach a workspace. The reason field lets the
// SPA disambiguate "not configured" (link to Settings) vs "open
// failed" (show the OpenRoot error). Pinned by docs/configuration.md.
type repoUnavailableErrorBody struct {
	Error  string `json:"error"`
	Reason string `json:"reason"`
}

// repoErrUserMessage strips the internal "tasks: invalid input: " prefix
// that pkgs/repo wraps onto every Resolve / ValidateRange / LineCount
// return path so the wire body the SPA renders matches the clean phrasing
// other handlers already produce via storeErrorClientMessage. Resolve
// failures are always wrapped with domain.ErrInvalidInput so the prefix
// is always present; LineCount can also surface raw OS errors (e.g. file
// vanished mid-read), in which case invalidInputDetail returns "" and the
// helper falls back to err.Error() unchanged.
//
// Without this helper, GET /repo/file and the Resolve branch of GET
// /repo/validate-range echoed the raw "tasks: invalid input: <reason>"
// string into the wire body — a phrase the SPA had no logic to strip
// and which other 400-emitting handlers had been laundering for years
// (see invalidInputDetail above).
func repoErrUserMessage(err error) string {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.repoErrUserMessage")
	if d := invalidInputDetail(err); d != "" {
		return d
	}
	return err.Error()
}

// requireWorktreeRepo resolves the workspace for worktree_id and writes
// the canonical error response when the caller cannot continue.
func (h *Handler) requireWorktreeRepo(w http.ResponseWriter, r *http.Request, op string) (*repo.Root, bool) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.requireWorktreeRepo", "http_op", op)
	worktreeID := strings.TrimSpace(r.URL.Query().Get("worktree_id"))
	if worktreeID == "" {
		writeJSONError(w, r, op, http.StatusBadRequest, "worktree_id query parameter is required")
		return nil, false
	}
	if h.repoProv == nil {
		writeJSON(w, r, op, http.StatusNotFound, repoUnavailableErrorBody{
			Error:  "worktree not found",
			Reason: RepoReasonWorktreeNotFound,
		})
		return nil, false
	}
	root, reason, err := h.repoProv.OpenWorktreeRoot(r.Context(), worktreeID)
	if err != nil {
		slog.Log(r.Context(), slog.LevelError, "repo provider failed",
			"cmd", calltrace.LogCmd, "operation", op, "reason", reason, "err", err)
		writeJSON(w, r, op, http.StatusInternalServerError, repoUnavailableErrorBody{
			Error: err.Error(), Reason: reason,
		})
		return nil, false
	}
	if root == nil {
		if reason == RepoReasonWorktreeNotFound {
			writeJSON(w, r, op, http.StatusNotFound, repoUnavailableErrorBody{
				Error:  "worktree not found",
				Reason: reason,
			})
			return nil, false
		}
		writeJSONError(w, r, op, http.StatusBadRequest, "worktree_id query parameter is required")
		return nil, false
	}
	return root, true
}

func (h *Handler) repoSearch(w http.ResponseWriter, r *http.Request) {
	const op = "repo.search"
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op, "search_q", truncateRunes(r.URL.Query().Get("q"), maxHTTPLogTitleRunes))
	if r.Method != http.MethodGet {
		writeError(w, r, op, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	root, ok := h.requireWorktreeRepo(w, r, op)
	if !ok {
		return
	}
	q := r.URL.Query().Get("q")
	if len(q) > maxRepoSearchQueryBytes {
		writeJSONError(w, r, op, http.StatusBadRequest, "search query too long")
		return
	}
	t0 := time.Now()
	paths, err := root.Search(q)
	dur := time.Since(t0)
	if err != nil {
		slog.Log(r.Context(), slog.LevelError, "repo operation failed", "cmd", calltrace.LogCmd, "operation", op, "duration_ms", dur.Milliseconds(), "err", err)
		writeJSONError(w, r, op, http.StatusInternalServerError, "search failed")
		return
	}
	slog.Info("repo search completed", "cmd", calltrace.LogCmd, "operation", op, "path_count", len(paths), "duration_ms", dur.Milliseconds(), "q_empty", strings.TrimSpace(q) == "")
	writeJSON(w, r, op, http.StatusOK, repoSearchResponse{Paths: paths})
}

func (h *Handler) repoValidateRange(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.repoValidateRange")
	const op = "repo.validate_range"
	r = calltrace.WithRequestRoot(r, op)
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	startStr := strings.TrimSpace(r.URL.Query().Get("start"))
	endStr := strings.TrimSpace(r.URL.Query().Get("end"))
	if r.Method != http.MethodGet {
		writeError(w, r, op, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	root, ok := h.requireWorktreeRepo(w, r, op)
	if !ok {
		return
	}
	if path == "" || startStr == "" || endStr == "" {
		writeJSONError(w, r, op, http.StatusBadRequest, "path, start, and end query parameters are required")
		return
	}
	if len(path) > maxRepoRelPathQueryBytes {
		writeJSONError(w, r, op, http.StatusBadRequest, "path too long")
		return
	}
	if len(startStr) > maxRepoLineQueryParamBytes || len(endStr) > maxRepoLineQueryParamBytes {
		writeJSONError(w, r, op, http.StatusBadRequest, "start or end too long")
		return
	}
	debugHTTPRequest(r, op, "validate_path", truncateRunes(path, maxHTTPLogTitleRunes), "validate_start", truncateRunes(startStr, maxHTTPLogTitleRunes), "validate_end", truncateRunes(endStr, maxHTTPLogTitleRunes))
	start, err1 := strconv.Atoi(startStr)
	end, err2 := strconv.Atoi(endStr)
	if err1 != nil || err2 != nil {
		writeJSONError(w, r, op, http.StatusBadRequest, "start and end must be integers")
		return
	}
	h.writeRepoValidateRangeOutcome(w, r, op, root, path, start, end)
}

func (h *Handler) writeRepoValidateRangeOutcome(w http.ResponseWriter, r *http.Request, op string, root *repo.Root, path string, start, end int) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.writeRepoValidateRangeOutcome")
	abs, err := root.Resolve(path)
	if err != nil {
		writeJSON(w, r, op, http.StatusOK, repoValidateRangeResponse{
			OK:      false,
			Warning: repoErrUserMessage(err),
		})
		return
	}
	n, err := repo.LineCount(abs)
	if err != nil {
		writeJSON(w, r, op, http.StatusOK, repoValidateRangeResponse{
			OK:      false,
			Warning: repoErrUserMessage(err),
		})
		return
	}
	if err := repo.ValidateRange(abs, start, end); err != nil {
		writeJSON(w, r, op, http.StatusOK, repoValidateRangeResponse{
			OK:        false,
			LineCount: n,
			Warning:   repoErrUserMessage(err),
		})
		return
	}
	writeJSON(w, r, op, http.StatusOK, repoValidateRangeResponse{OK: true, LineCount: n})
}

func (h *Handler) repoFile(w http.ResponseWriter, r *http.Request) {
	const op = "repo.file"
	r = calltrace.WithRequestRoot(r, op)
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	debugHTTPRequest(r, op, "file_path", truncateRunes(path, maxHTTPLogTitleRunes))
	if r.Method != http.MethodGet {
		writeError(w, r, op, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	root, ok := h.requireWorktreeRepo(w, r, op)
	if !ok {
		return
	}
	if path == "" {
		writeJSONError(w, r, op, http.StatusBadRequest, "path query parameter is required")
		return
	}
	if len(path) > maxRepoRelPathQueryBytes {
		writeJSONError(w, r, op, http.StatusBadRequest, "path too long")
		return
	}
	abs, err := root.Resolve(path)
	if err != nil {
		writeJSONError(w, r, op, http.StatusBadRequest, repoErrUserMessage(err))
		return
	}
	t0 := time.Now()
	fp, err := repo.ReadFilePreview(abs)
	dur := time.Since(t0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSONError(w, r, op, http.StatusNotFound, "file not found")
			return
		}
		slog.Log(r.Context(), slog.LevelError, "repo operation failed", "cmd", calltrace.LogCmd, "operation", op, "duration_ms", dur.Milliseconds(), "err", err)
		writeJSONError(w, r, op, http.StatusInternalServerError, "read failed")
		return
	}
	slog.Info("repo file preview", "cmd", calltrace.LogCmd, "operation", op, "duration_ms", dur.Milliseconds(), "binary", fp.Binary, "truncated", fp.Truncated, "size_bytes", fp.SizeBytes)
	resp := repoFileResponse{
		Path:      path,
		Content:   fp.Content,
		Binary:    fp.Binary,
		Truncated: fp.Truncated,
		SizeBytes: fp.SizeBytes,
		LineCount: fp.LineCount,
	}
	switch {
	case fp.Binary && fp.Truncated:
		resp.Warning = "This file looks binary or non-text, and the preview was truncated. You can insert the file reference without a line range."
	case fp.Binary:
		resp.Warning = "This file looks binary or non-text. You can insert the file reference without a line range."
	case fp.Truncated:
		resp.Warning = "Preview truncated to the first 32 MiB of the file. Line selection applies only to the visible text."
	}
	writeJSON(w, r, op, http.StatusOK, resp)
}

func (h *Handler) repoDiff(w http.ResponseWriter, r *http.Request) {
	const op = "repo.diff"
	r = calltrace.WithRequestRoot(r, op)
	sha := strings.TrimSpace(r.URL.Query().Get("sha"))
	debugHTTPRequest(r, op, "diff_sha", truncateRunes(sha, maxHTTPLogTitleRunes))
	if r.Method != http.MethodGet {
		writeError(w, r, op, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	root, ok := h.requireWorktreeRepo(w, r, op)
	if !ok {
		return
	}
	if sha == "" {
		writeJSONError(w, r, op, http.StatusBadRequest, "sha query parameter is required")
		return
	}
	if len(sha) > maxRepoShaQueryBytes {
		writeJSONError(w, r, op, http.StatusBadRequest, "sha too long")
		return
	}
	if !repoShaQueryPattern.MatchString(sha) {
		writeJSONError(w, r, op, http.StatusBadRequest, "invalid sha")
		return
	}
	t0 := time.Now()
	patch, truncated, err := gitexec.ShowCommitPatch(r.Context(), root.Abs(), sha, gitexec.DefaultMaxPatchBytes)
	meta, metaErr := gitexec.LoadCommitMeta(r.Context(), root.Abs(), sha)
	dur := time.Since(t0)
	if err != nil {
		if errors.Is(err, gitexec.ErrNotFound) {
			writeJSONError(w, r, op, http.StatusNotFound, "commit not found")
			return
		}
		slog.Log(r.Context(), slog.LevelError, "repo diff failed",
			"cmd", calltrace.LogCmd, "operation", op, "duration_ms", dur.Milliseconds(), "err", err)
		writeJSONError(w, r, op, http.StatusInternalServerError, "diff failed")
		return
	}
	if metaErr != nil && !errors.Is(metaErr, gitexec.ErrNotFound) {
		slog.Log(r.Context(), slog.LevelWarn, "repo diff meta failed",
			"cmd", calltrace.LogCmd, "operation", op, "duration_ms", dur.Milliseconds(), "err", metaErr)
	}
	slog.Info("repo diff completed", "cmd", calltrace.LogCmd, "operation", op,
		"duration_ms", dur.Milliseconds(), "truncated", truncated, "size_bytes", len(patch))
	writeJSON(w, r, op, http.StatusOK, repoDiffResponse{
		SHA:          sha,
		Patch:        patch,
		Truncated:    truncated,
		SizeBytes:    len(patch),
		Author:       meta.Author,
		AuthorEmail:  meta.AuthorEmail,
		ParentSHA:    meta.ParentSHA,
		FilesChanged: meta.FilesChanged,
		Insertions:   meta.Insertions,
		Deletions:    meta.Deletions,
	})
}
