package handler

import (
	"encoding/json"
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

type jsonErrorBody struct {
	Error string `json:"error"`
}

func writeJSONError(w http.ResponseWriter, op string, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(jsonErrorBody{Error: msg}); err != nil {
		slog.Error("response encode failed", "cmd", httpLogCmd, "operation", op, "err", err)
	}
}

func (h *Handler) repoSearch(w http.ResponseWriter, r *http.Request) {
	const op = "repo.search"
	if r.Method != http.MethodGet {
		writeError(w, op, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	if h.repo == nil {
		writeJSONError(w, op, http.StatusServiceUnavailable, "repo is not configured (set REPO_ROOT)")
		return
	}
	q := r.URL.Query().Get("q")
	paths, err := h.repo.Search(q)
	if err != nil {
		slog.Error("repo operation failed", "cmd", httpLogCmd, "operation", op, "err", err)
		writeJSONError(w, op, http.StatusInternalServerError, "search failed")
		return
	}
	writeJSON(w, op, http.StatusOK, repoSearchResponse{Paths: paths})
}

func (h *Handler) repoValidateRange(w http.ResponseWriter, r *http.Request) {
	const op = "repo.validate_range"
	if r.Method != http.MethodGet {
		writeError(w, op, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}
	if h.repo == nil {
		writeJSONError(w, op, http.StatusServiceUnavailable, "repo is not configured (set REPO_ROOT)")
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	startStr := strings.TrimSpace(r.URL.Query().Get("start"))
	endStr := strings.TrimSpace(r.URL.Query().Get("end"))
	if path == "" || startStr == "" || endStr == "" {
		writeJSONError(w, op, http.StatusBadRequest, "path, start, and end query parameters are required")
		return
	}
	start, err1 := strconv.Atoi(startStr)
	end, err2 := strconv.Atoi(endStr)
	if err1 != nil || err2 != nil {
		writeJSONError(w, op, http.StatusBadRequest, "start and end must be integers")
		return
	}
	abs, err := h.repo.Resolve(path)
	if err != nil {
		writeJSON(w, op, http.StatusOK, repoValidateRangeResponse{
			OK:      false,
			Warning: err.Error(),
		})
		return
	}
	n, err := repo.LineCount(abs)
	if err != nil {
		writeJSON(w, op, http.StatusOK, repoValidateRangeResponse{
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
		writeJSON(w, op, http.StatusOK, repoValidateRangeResponse{
			OK:        false,
			LineCount: n,
			Warning:   msg,
		})
		return
	}
	writeJSON(w, op, http.StatusOK, repoValidateRangeResponse{OK: true, LineCount: n})
}

