package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/registry"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
)

// runnersListTimeout caps the time spent listing model schemas
// (pure in-memory; added for symmetry with the probe timeout).
const runnersListTimeout = 2 * time.Second

// runnerProbeTimeout caps POST /runners/{id}/probe.
const runnerProbeTimeout = 5 * time.Second

// runnerListModelsTimeout caps POST /runners/{id}/list-models.
const runnerListModelsTimeout = 30 * time.Second

// ---------------------------------------------------------------------------
// GET /runners
// ---------------------------------------------------------------------------

type runnerDescriptorWire struct {
	ID                string               `json:"id"`
	Label             string               `json:"label"`
	DefaultBinaryHint string               `json:"default_binary_hint"`
	ConfigSchema      *runner.ConfigSchema `json:"config_schema,omitempty"`
}

func (h *Handler) listRunners(w http.ResponseWriter, r *http.Request) {
	const op = "runners.list"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.listRunners")
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)

	descs := registry.List()
	out := make([]runnerDescriptorWire, 0, len(descs))
	for _, d := range descs {
		wire := runnerDescriptorWire{
			ID:                d.ID,
			Label:             d.Label,
			DefaultBinaryHint: d.DefaultBinaryHint,
		}
		built, err := registry.Build(d.ID, registry.BuildOptions{})
		if err == nil {
			if csp, ok := built.(runner.ConfigSchemaProvider); ok {
				schema := csp.ConfigSchema()
				wire.ConfigSchema = &schema
			}
		}
		out = append(out, wire)
	}
	writeJSON(w, r, op, http.StatusOK, out)
}

// ---------------------------------------------------------------------------
// POST /runners/{id}/probe
// ---------------------------------------------------------------------------

type runnerProbeResponse struct {
	OK         bool   `json:"ok"`
	Runner     string `json:"runner"`
	BinaryPath string `json:"binary_path,omitempty"`
	Version    string `json:"version,omitempty"`
	Error      string `json:"error,omitempty"`
}

func (h *Handler) probeRunner(w http.ResponseWriter, r *http.Request) {
	const op = "runners.probe"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.probeRunner")
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)

	runnerID := r.PathValue("id")
	if runnerID == "" {
		writeJSONError(w, r, op, http.StatusBadRequest, "runner id required")
		return
	}

	var body struct {
		BinaryPath string `json:"binary_path,omitempty"`
	}
	if r.ContentLength != 0 {
		if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
			if !errors.Is(err, io.EOF) {
				writeError(w, r, op, err, http.StatusBadRequest)
				return
			}
		}
	}
	body.BinaryPath = strings.TrimSpace(body.BinaryPath)

	if body.BinaryPath == "" {
		cfg, err := h.store.GetSettings(r.Context())
		if err != nil {
			writeStoreError(w, r, op, err)
			return
		}
		if runnerID == registry.CursorRunnerID {
			body.BinaryPath = cfg.CursorBin
		}
	}

	version, resolvedBin, err := registry.Probe(r.Context(), runnerID, body.BinaryPath, runnerProbeTimeout)
	resp := runnerProbeResponse{Runner: runnerID, BinaryPath: resolvedBin}
	if err != nil {
		resp.OK = false
		resp.Error = err.Error()
		if errors.Is(err, registry.ErrUnknownRunner) {
			writeJSON(w, r, op, http.StatusNotFound, resp)
			return
		}
		if errors.Is(err, runner.ErrCapabilityNotSupported) {
			writeJSON(w, r, op, http.StatusNotImplemented, resp)
			return
		}
		writeJSON(w, r, op, http.StatusOK, resp)
		return
	}
	resp.OK = true
	resp.Version = version
	writeJSON(w, r, op, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// POST /runners/{id}/list-models
// ---------------------------------------------------------------------------

type runnerListModelsResponse struct {
	OK         bool               `json:"ok"`
	Runner     string             `json:"runner"`
	BinaryPath string             `json:"binary_path,omitempty"`
	Models     []runner.ModelInfo `json:"models,omitempty"`
	Error      string             `json:"error,omitempty"`
}

func (h *Handler) listRunnerModels(w http.ResponseWriter, r *http.Request) {
	const op = "runners.list_models"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.listRunnerModels")
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)

	runnerID := r.PathValue("id")
	if runnerID == "" {
		writeJSONError(w, r, op, http.StatusBadRequest, "runner id required")
		return
	}

	var body struct {
		BinaryPath string `json:"binary_path,omitempty"`
	}
	if r.ContentLength != 0 {
		if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
			if !errors.Is(err, io.EOF) {
				writeError(w, r, op, err, http.StatusBadRequest)
				return
			}
		}
	}
	body.BinaryPath = strings.TrimSpace(body.BinaryPath)

	if body.BinaryPath == "" {
		cfg, err := h.store.GetSettings(r.Context())
		if err != nil {
			writeStoreError(w, r, op, err)
			return
		}
		if runnerID == registry.CursorRunnerID {
			body.BinaryPath = cfg.CursorBin
		}
	}

	models, resolvedBin, err := registry.ListModelsForRunner(r.Context(), runnerID, body.BinaryPath, runnerListModelsTimeout)
	resp := runnerListModelsResponse{Runner: runnerID, BinaryPath: resolvedBin}
	if err != nil {
		resp.OK = false
		resp.Error = err.Error()
		if errors.Is(err, registry.ErrUnknownRunner) {
			writeJSON(w, r, op, http.StatusNotFound, resp)
			return
		}
		if errors.Is(err, runner.ErrCapabilityNotSupported) {
			writeJSON(w, r, op, http.StatusNotImplemented, resp)
			return
		}
		writeJSON(w, r, op, http.StatusOK, resp)
		return
	}
	resp.OK = true
	resp.Models = models
	writeJSON(w, r, op, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// GET /runners/{id}/config-schema
// ---------------------------------------------------------------------------

func (h *Handler) runnerConfigSchema(w http.ResponseWriter, r *http.Request) {
	const op = "runners.config_schema"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.runnerConfigSchema")
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)

	runnerID := r.PathValue("id")
	if runnerID == "" {
		writeJSONError(w, r, op, http.StatusBadRequest, "runner id required")
		return
	}

	built, err := registry.Build(runnerID, registry.BuildOptions{})
	if err != nil {
		if errors.Is(err, registry.ErrUnknownRunner) {
			writeJSONError(w, r, op, http.StatusNotFound, "unknown runner")
			return
		}
		writeJSONError(w, r, op, http.StatusInternalServerError, err.Error())
		return
	}
	csp, ok := built.(runner.ConfigSchemaProvider)
	if !ok {
		writeJSONError(w, r, op, http.StatusNotImplemented, "runner does not expose a config schema")
		return
	}
	writeJSON(w, r, op, http.StatusOK, csp.ConfigSchema())
}

// ---------------------------------------------------------------------------
// POST /runners/{id}/validate-config
// ---------------------------------------------------------------------------

func (h *Handler) validateRunnerConfig(w http.ResponseWriter, r *http.Request) {
	const op = "runners.validate_config"
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.validateRunnerConfig")
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)

	runnerID := r.PathValue("id")
	if runnerID == "" {
		writeJSONError(w, r, op, http.StatusBadRequest, "runner id required")
		return
	}

	built, err := registry.Build(runnerID, registry.BuildOptions{})
	if err != nil {
		if errors.Is(err, registry.ErrUnknownRunner) {
			writeJSONError(w, r, op, http.StatusNotFound, "unknown runner")
			return
		}
		writeJSONError(w, r, op, http.StatusInternalServerError, err.Error())
		return
	}
	cv, ok := built.(runner.ConfigValidator)
	if !ok {
		writeJSONError(w, r, op, http.StatusNotImplemented, "runner does not support config validation")
		return
	}

	var blob json.RawMessage
	if err := decodeJSON(r.Context(), r.Body, &blob); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	if err := cv.ValidateConfig(blob); err != nil {
		writeJSON(w, r, op, http.StatusUnprocessableEntity, map[string]any{
			"valid": false,
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, r, op, http.StatusOK, map[string]any{"valid": true})
}
