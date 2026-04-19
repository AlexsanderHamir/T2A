package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// settingsResponse is the on-the-wire shape of GET /settings and the
// PATCH /settings response. Defined separately from
// store.AppSettings so the wire format stays stable independent of
// any future DB schema renames or extra non-public columns.
//
// updated_at is RFC3339 so the SPA can render "last changed N seconds
// ago" without a parser; nil-safe because the row always exists once
// GET seeds defaults on first boot.
type settingsResponse struct {
	WorkerEnabled         bool   `json:"worker_enabled"`
	Runner                string `json:"runner"`
	RepoRoot              string `json:"repo_root"`
	CursorBin             string `json:"cursor_bin"`
	MaxRunDurationSeconds int    `json:"max_run_duration_seconds"`
	UpdatedAt             string `json:"updated_at,omitempty"`
}

// settingsPatchBody is the JSON body accepted by PATCH /settings.
// Pointer fields distinguish "not provided" from an explicit zero —
// for example *MaxRunDurationSeconds = 0 means "no limit", while
// nil leaves the previous value untouched. The contract mirrors
// store.SettingsPatch one-for-one so the handler can map fields
// directly without any field-by-field adapter logic.
type settingsPatchBody struct {
	WorkerEnabled         *bool   `json:"worker_enabled,omitempty"`
	Runner                *string `json:"runner,omitempty"`
	RepoRoot              *string `json:"repo_root,omitempty"`
	CursorBin             *string `json:"cursor_bin,omitempty"`
	MaxRunDurationSeconds *int    `json:"max_run_duration_seconds,omitempty"`
}

// probeRequest is the JSON body for POST /settings/probe-cursor. Both
// fields are optional: empty Runner falls back to the value already
// stored in app_settings, and empty BinaryPath asks the registry to
// auto-detect from PATH (the same logic as boot probing).
type probeRequest struct {
	Runner     string `json:"runner,omitempty"`
	BinaryPath string `json:"binary_path,omitempty"`
}

type probeResponse struct {
	OK      bool   `json:"ok"`
	Runner  string `json:"runner"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

type cancelRunResponse struct {
	Cancelled bool `json:"cancelled"`
}

// settingsProbeTimeout caps how long POST /settings/probe-cursor will
// spend invoking the runner binary. Matches the supervisor's per-boot
// probe budget so a flaky cursor install behaves identically whether
// the operator hits "Test" or restarts the process.
const settingsProbeTimeout = 5 * time.Second

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	const op = "settings.get"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.getSettings")
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)

	cfg, err := h.store.GetSettings(r.Context())
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, settingsResponseFrom(cfg))
}

func (h *Handler) patchSettings(w http.ResponseWriter, r *http.Request) {
	const op = "settings.patch"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patchSettings")
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)

	if h.agent == nil {
		writeJSONError(w, r, op, http.StatusServiceUnavailable, "agent worker control unavailable")
		return
	}

	var body settingsPatchBody
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	patch := store.SettingsPatch{
		WorkerEnabled:         body.WorkerEnabled,
		Runner:                body.Runner,
		RepoRoot:              body.RepoRoot,
		CursorBin:             body.CursorBin,
		MaxRunDurationSeconds: body.MaxRunDurationSeconds,
	}
	if patch.IsEmpty() {
		writeJSONError(w, r, op, http.StatusBadRequest, "patch body must include at least one field")
		return
	}

	updated, err := h.store.UpdateSettings(r.Context(), patch)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if reloadErr := h.agent.Reload(r.Context()); reloadErr != nil {
		slog.Error("settings patch persisted but supervisor reload failed",
			"cmd", calltrace.LogCmd, "operation", op, "err", reloadErr)
		writeJSONError(w, r, op, http.StatusInternalServerError, "settings saved but worker reload failed")
		return
	}
	h.hub.Publish(TaskChangeEvent{Type: SettingsChanged})
	writeJSON(w, r, op, http.StatusOK, settingsResponseFrom(updated))
}

func (h *Handler) probeCursor(w http.ResponseWriter, r *http.Request) {
	const op = "settings.probe_cursor"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.probeCursor")
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)

	if h.agent == nil {
		writeJSONError(w, r, op, http.StatusServiceUnavailable, "agent worker control unavailable")
		return
	}

	var body probeRequest
	// ContentLength == 0 ⇒ definitely no body (e.g. SPA "Test" with no
	// form input, falls back to stored values). ContentLength > 0 ⇒
	// length-prefixed body. ContentLength == -1 ⇒ length unknown
	// (HTTP/1.1 chunked transfer-encoding); we still need to attempt
	// the decode so explicit `runner` / `binary_path` overrides in
	// chunked POSTs are honored. Only an io.EOF (truly empty body
	// despite the unknown-length hint) is treated as "no body" and
	// falls through to the stored-values branch.
	if r.ContentLength != 0 {
		if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
			if !errors.Is(err, io.EOF) {
				writeError(w, r, op, err, http.StatusBadRequest)
				return
			}
		}
	}
	body.Runner = strings.TrimSpace(body.Runner)
	body.BinaryPath = strings.TrimSpace(body.BinaryPath)

	if body.Runner == "" || body.BinaryPath == "" {
		cfg, err := h.store.GetSettings(r.Context())
		if err != nil {
			writeStoreError(w, r, op, err)
			return
		}
		if body.Runner == "" {
			body.Runner = cfg.Runner
		}
		if body.BinaryPath == "" {
			body.BinaryPath = cfg.CursorBin
		}
	}

	version, err := h.agent.ProbeRunner(r.Context(), body.Runner, body.BinaryPath, settingsProbeTimeout)
	resp := probeResponse{Runner: body.Runner}
	if err != nil {
		resp.OK = false
		resp.Error = err.Error()
		writeJSON(w, r, op, http.StatusOK, resp)
		return
	}
	resp.OK = true
	resp.Version = version
	writeJSON(w, r, op, http.StatusOK, resp)
}

func (h *Handler) cancelCurrentRun(w http.ResponseWriter, r *http.Request) {
	const op = "settings.cancel_current_run"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.cancelCurrentRun")
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)

	if h.agent == nil {
		writeJSONError(w, r, op, http.StatusServiceUnavailable, "agent worker control unavailable")
		return
	}
	cancelled := h.agent.CancelCurrentRun()
	if cancelled {
		h.hub.Publish(TaskChangeEvent{Type: AgentRunCancelled})
	}
	writeJSON(w, r, op, http.StatusOK, cancelRunResponse{Cancelled: cancelled})
}

// settingsResponseFrom translates the persistence row into the wire
// shape so the handler never leaks GORM-specific quirks (zero-value
// time, ID column) to clients.
func settingsResponseFrom(cfg store.AppSettings) settingsResponse {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.settingsResponseFrom")
	resp := settingsResponse{
		WorkerEnabled:         cfg.WorkerEnabled,
		Runner:                cfg.Runner,
		RepoRoot:              cfg.RepoRoot,
		CursorBin:             cfg.CursorBin,
		MaxRunDurationSeconds: cfg.MaxRunDurationSeconds,
	}
	if !cfg.UpdatedAt.IsZero() {
		resp.UpdatedAt = cfg.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return resp
}
