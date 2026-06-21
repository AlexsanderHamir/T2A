package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/registry/all"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/registry"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// runnersTestServer creates a test handler wired with a real SQLite
// store and the fake agent control (which satisfies AgentWorkerControl).
func runnersTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	hub := NewSSEHub()
	ctrl := &fakeAgentControl{}
	h := NewHandler(st, hub, nil, WithAgentWorkerControl(ctrl))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func mustRunnersHTTP(t *testing.T, method, url, body string, wantStatus int) []byte {
	t.Helper()
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("status=%d want=%d body=%s", resp.StatusCode, wantStatus, b)
	}
	return b
}

// ---------------------------------------------------------------------------
// GET /runners
// ---------------------------------------------------------------------------

func TestHTTP_ListRunners_returnsCursorEntry(t *testing.T) {
	srv := runnersTestServer(t)
	body := mustRunnersHTTP(t, http.MethodGet, srv.URL+"/runners", "", http.StatusOK)

	var list []runnerDescriptorWire
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if len(list) == 0 {
		t.Fatal("expected at least one runner in the list")
	}
	found := false
	for _, r := range list {
		if r.ID == registry.CursorRunnerID {
			found = true
			if r.Label == "" {
				t.Error("cursor entry has empty label")
			}
			if r.ConfigSchema == nil {
				t.Error("cursor entry missing config_schema")
			} else if len(r.ConfigSchema.Fields) == 0 {
				t.Error("cursor config_schema has no fields")
			}
		}
	}
	if !found {
		t.Errorf("cursor runner not found in list: %s", body)
	}
}

// ---------------------------------------------------------------------------
// GET /runners/{id}/config-schema
// ---------------------------------------------------------------------------

func TestHTTP_RunnerConfigSchema_knownRunner(t *testing.T) {
	srv := runnersTestServer(t)
	body := mustRunnersHTTP(t, http.MethodGet, srv.URL+"/runners/cursor/config-schema", "", http.StatusOK)

	var schema struct {
		Version int `json:"version"`
		Fields  []struct {
			Key  string `json:"key"`
			Type string `json:"type"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &schema); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if schema.Version == 0 {
		t.Error("schema version should be > 0")
	}
	if len(schema.Fields) == 0 {
		t.Error("expected at least one field in cursor config schema")
	}
}

func TestHTTP_RunnerConfigSchema_unknownRunner(t *testing.T) {
	srv := runnersTestServer(t)
	mustRunnersHTTP(t, http.MethodGet, srv.URL+"/runners/unknown-cli/config-schema", "", http.StatusNotFound)
}

// ---------------------------------------------------------------------------
// POST /runners/{id}/validate-config
// ---------------------------------------------------------------------------

func TestHTTP_ValidateRunnerConfig_validBlob(t *testing.T) {
	srv := runnersTestServer(t)
	body := mustRunnersHTTP(t, http.MethodPost, srv.URL+"/runners/cursor/validate-config",
		`{"binary_path":"/usr/bin/cursor"}`, http.StatusOK)

	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if resp["valid"] != true {
		t.Errorf("expected valid=true, got %v", resp["valid"])
	}
}

func TestHTTP_ValidateRunnerConfig_invalidBlob(t *testing.T) {
	srv := runnersTestServer(t)
	body := mustRunnersHTTP(t, http.MethodPost, srv.URL+"/runners/cursor/validate-config",
		`{"unknown_key":"value"}`, http.StatusUnprocessableEntity)

	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if resp["valid"] != false {
		t.Errorf("expected valid=false, got %v", resp["valid"])
	}
	if resp["error"] == nil || resp["error"] == "" {
		t.Error("expected non-empty error message for invalid config")
	}
}

func TestHTTP_ValidateRunnerConfig_unknownRunner(t *testing.T) {
	srv := runnersTestServer(t)
	mustRunnersHTTP(t, http.MethodPost, srv.URL+"/runners/unknown-cli/validate-config",
		`{}`, http.StatusNotFound)
}

// ---------------------------------------------------------------------------
// POST /runners/{id}/probe — unknown runner → 404
// ---------------------------------------------------------------------------

func TestHTTP_ProbeRunner_unknownRunner(t *testing.T) {
	srv := runnersTestServer(t)
	body := mustRunnersHTTP(t, http.MethodPost, srv.URL+"/runners/unknown-cli/probe",
		"", http.StatusNotFound)

	var resp runnerProbeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if resp.OK {
		t.Error("expected OK=false for unknown runner")
	}
}

// ---------------------------------------------------------------------------
// POST /runners/{id}/list-models — unknown runner → 404
// ---------------------------------------------------------------------------

func TestHTTP_ListRunnerModels_unknownRunner(t *testing.T) {
	srv := runnersTestServer(t)
	body := mustRunnersHTTP(t, http.MethodPost, srv.URL+"/runners/unknown-cli/list-models",
		"", http.StatusNotFound)

	var resp runnerListModelsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if resp.OK {
		t.Error("expected OK=false for unknown runner")
	}
}
