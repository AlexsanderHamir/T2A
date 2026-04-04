package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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
	paths, err := h.repo.Search(q)
	if err != nil {
		slog.Log(r.Context(), slog.LevelError, "repo operation failed", "cmd", httpLogCmd, "operation", op, "err", err)
		writeJSONError(w, r, op, http.StatusInternalServerError, "search failed")
		return
	}
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
