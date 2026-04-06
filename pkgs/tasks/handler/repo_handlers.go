package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
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

func (h *Handler) repoSearch(w http.ResponseWriter, r *http.Request) {
	const op = "repo.search"
	r = withCallRoot(r, op)
	debugHTTPRequest(r, op, "search_q", truncateRunes(r.URL.Query().Get("q"), maxHTTPLogTitleRunes))
	if r.Method != http.MethodGet {
		writeError(w, r, op, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	if h.repo == nil {
		writeJSONError(w, r, op, http.StatusServiceUnavailable, "repo is not configured (set REPO_ROOT)")
		return
	}
	q := r.URL.Query().Get("q")
	t0 := time.Now()
	paths, err := h.repo.Search(q)
	dur := time.Since(t0)
	if err != nil {
		slog.Log(r.Context(), slog.LevelError, "repo operation failed", "cmd", httpLogCmd, "operation", op, "duration_ms", dur.Milliseconds(), "err", err)
		writeJSONError(w, r, op, http.StatusInternalServerError, "search failed")
		return
	}
	slog.Info("repo search completed", "cmd", httpLogCmd, "operation", op, "path_count", len(paths), "duration_ms", dur.Milliseconds(), "q_empty", strings.TrimSpace(q) == "")
	writeJSON(w, r, op, http.StatusOK, repoSearchResponse{Paths: paths})
}

func (h *Handler) repoValidateRange(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.repoValidateRange")
	const op = "repo.validate_range"
	r = withCallRoot(r, op)
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	startStr := strings.TrimSpace(r.URL.Query().Get("start"))
	endStr := strings.TrimSpace(r.URL.Query().Get("end"))
	debugHTTPRequest(r, op, "validate_path", path, "validate_start", startStr, "validate_end", endStr)
	if r.Method != http.MethodGet {
		writeError(w, r, op, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	if h.repo == nil {
		writeJSONError(w, r, op, http.StatusServiceUnavailable, "repo is not configured (set REPO_ROOT)")
		return
	}
	if path == "" || startStr == "" || endStr == "" {
		writeJSONError(w, r, op, http.StatusBadRequest, "path, start, and end query parameters are required")
		return
	}
	start, err1 := strconv.Atoi(startStr)
	end, err2 := strconv.Atoi(endStr)
	if err1 != nil || err2 != nil {
		writeJSONError(w, r, op, http.StatusBadRequest, "start and end must be integers")
		return
	}
	abs, err := h.repo.Resolve(path)
	if err != nil {
		writeJSON(w, r, op, http.StatusOK, repoValidateRangeResponse{
			OK:      false,
			Warning: err.Error(),
		})
		return
	}
	n, err := repo.LineCount(abs)
	if err != nil {
		writeJSON(w, r, op, http.StatusOK, repoValidateRangeResponse{
			OK:      false,
			Warning: err.Error(),
		})
		return
	}
	if err := repo.ValidateRange(abs, start, end); err != nil {
		msg := err.Error()
		if errors.Is(err, domain.ErrInvalidInput) {
			msg = strings.TrimPrefix(msg, domain.ErrInvalidInput.Error())
			msg = strings.TrimPrefix(msg, ": ")
		}
		writeJSON(w, r, op, http.StatusOK, repoValidateRangeResponse{
			OK:        false,
			LineCount: n,
			Warning:   msg,
		})
		return
	}
	writeJSON(w, r, op, http.StatusOK, repoValidateRangeResponse{OK: true, LineCount: n})
}

func (h *Handler) repoFile(w http.ResponseWriter, r *http.Request) {
	const op = "repo.file"
	r = withCallRoot(r, op)
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	debugHTTPRequest(r, op, "file_path", truncateRunes(path, maxHTTPLogTitleRunes))
	if r.Method != http.MethodGet {
		writeError(w, r, op, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	if h.repo == nil {
		writeJSONError(w, r, op, http.StatusServiceUnavailable, "repo is not configured (set REPO_ROOT)")
		return
	}
	if path == "" {
		writeJSONError(w, r, op, http.StatusBadRequest, "path query parameter is required")
		return
	}
	abs, err := h.repo.Resolve(path)
	if err != nil {
		writeJSONError(w, r, op, http.StatusBadRequest, err.Error())
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
		slog.Log(r.Context(), slog.LevelError, "repo operation failed", "cmd", httpLogCmd, "operation", op, "duration_ms", dur.Milliseconds(), "err", err)
		writeJSONError(w, r, op, http.StatusInternalServerError, "read failed")
		return
	}
	slog.Info("repo file preview", "cmd", httpLogCmd, "operation", op, "duration_ms", dur.Milliseconds(), "binary", fp.Binary, "truncated", fp.Truncated, "size_bytes", fp.SizeBytes)
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
