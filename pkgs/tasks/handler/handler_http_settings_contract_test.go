package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// fakeAgentControl is the test stand-in for cmd/taskapi.agentWorkerSupervisor.
// Tracks call counts so contract tests can assert that PATCH /settings
// triggers Reload, POST /settings/cancel-current-run triggers
// CancelCurrentRun, and POST /settings/probe-cursor wires the binary
// path through to ProbeRunner.
type fakeAgentControl struct {
	cancelResult  atomic.Bool
	cancelCalls   atomic.Int32
	reloadCalls   atomic.Int32
	reloadErr     atomic.Pointer[error]
	probeCalls    atomic.Int32
	probeVersion  atomic.Pointer[string]
	probeResolved atomic.Pointer[string]
	probeErr      atomic.Pointer[error]
	lastRunner    atomic.Pointer[string]
	lastBinary    atomic.Pointer[string]
}

func (f *fakeAgentControl) CancelCurrentRun() bool {
	f.cancelCalls.Add(1)
	return f.cancelResult.Load()
}

func (f *fakeAgentControl) Reload(_ context.Context) error {
	f.reloadCalls.Add(1)
	if e := f.reloadErr.Load(); e != nil {
		return *e
	}
	return nil
}

func (f *fakeAgentControl) ProbeRunner(_ context.Context, runnerID, binaryPath string, _ time.Duration) (string, string, error) {
	f.probeCalls.Add(1)
	r := runnerID
	b := binaryPath
	f.lastRunner.Store(&r)
	f.lastBinary.Store(&b)
	resolved := ""
	if rp := f.probeResolved.Load(); rp != nil {
		resolved = *rp
	}
	if e := f.probeErr.Load(); e != nil {
		return "", resolved, *e
	}
	if v := f.probeVersion.Load(); v != nil {
		return *v, resolved, nil
	}
	return "", resolved, nil
}

// settingsTestServer wires the same handler the production binary
// builds, with our fake supervisor injected. Returns the server, the
// underlying store (so tests can seed AppSettings rows directly), the
// hub (so settings_changed / agent_run_cancelled SSE events can be
// asserted), and the fake control so tests can mutate its behaviour.
func settingsTestServer(t *testing.T) (*httptest.Server, *store.Store, *SSEHub, *fakeAgentControl) {
	t.Helper()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	hub := NewSSEHub()
	ctrl := &fakeAgentControl{}
	h := NewHandler(st, hub, nil, WithAgentWorkerControl(ctrl))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, st, hub, ctrl
}

func settingsTestServerNoAgent(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	hub := NewSSEHub()
	h := NewHandler(st, hub, nil)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, st
}

// TestHTTP_GetSettings_returnsSeededDefaults pins the documented
// "first GET seeds defaults" contract: a fresh DB returns a populated
// row so the SPA never has to render an empty form.
func TestHTTP_GetSettings_returnsSeededDefaults(t *testing.T) {
	srv, _, _, _ := settingsTestServer(t)

	body := mustGetSettingsJSON(t, srv.URL+"/settings", http.StatusOK)
	var resp settingsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if !resp.WorkerEnabled {
		t.Error("expected WorkerEnabled=true on first read")
	}
	if resp.Runner != "cursor" {
		t.Errorf("Runner=%q, want cursor", resp.Runner)
	}
	if resp.RepoRoot != "" {
		t.Errorf("RepoRoot=%q, want empty (operator must configure)", resp.RepoRoot)
	}
	if resp.MaxRunDurationSeconds != 0 {
		t.Errorf("MaxRunDurationSeconds=%d, want 0 (no limit)", resp.MaxRunDurationSeconds)
	}
}

// TestHTTP_GetSettings_worksWithoutAgentControl confirms read-only
// access stays available even when the supervisor isn't wired (e.g.
// during local devsim runs). Critical for the SPA's first paint.
func TestHTTP_GetSettings_worksWithoutAgentControl(t *testing.T) {
	srv, _ := settingsTestServerNoAgent(t)
	body := mustGetSettingsJSON(t, srv.URL+"/settings", http.StatusOK)
	if !strings.Contains(string(body), `"worker_enabled":true`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

// TestHTTP_PatchSettings_persistsAndReloads exercises the happy path:
// PATCH writes the row, supervisor.Reload is called exactly once, an
// SSE settings_changed event fans out, and the response echoes the
// new state. Without the SSE event the SPA would have to poll for
// changes; without the Reload call the worker would keep running on
// stale config until the next process restart.
func TestHTTP_PatchSettings_persistsAndReloads(t *testing.T) {
	srv, _, hub, ctrl := settingsTestServer(t)
	ch, cancel := hub.Subscribe()
	defer cancel()

	body := mustPatchSettingsJSON(t, srv.URL+"/settings",
		`{"repo_root":"/tmp/x","max_run_duration_seconds":120,"worker_enabled":false}`,
		http.StatusOK)
	var resp settingsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if resp.RepoRoot != "/tmp/x" || resp.MaxRunDurationSeconds != 120 || resp.WorkerEnabled {
		t.Errorf("response did not reflect patch: %+v", resp)
	}
	if got := ctrl.reloadCalls.Load(); got != 1 {
		t.Errorf("reload calls = %d, want 1", got)
	}

	got := summarize(drainSSE(t, ch, 1, 2*time.Second))
	want := []string{"settings_changed:"}
	mustEqualEvents(t, "PATCH /settings", got, want)
}

// TestHTTP_PatchSettings_emptyBodyRejected stops the SPA from
// accidentally clearing the row by sending {} (which used to be a
// no-op valid request). 400 with a clear message lets the SPA surface
// the error inline next to the Save button.
func TestHTTP_PatchSettings_emptyBodyRejected(t *testing.T) {
	srv, _, _, _ := settingsTestServer(t)
	resp := mustSettingsHTTP(t, http.MethodPatch, srv.URL+"/settings", `{}`, http.StatusBadRequest)
	if !strings.Contains(string(resp), "at least one field") {
		t.Fatalf("error body did not mention 'at least one field': %s", resp)
	}
}

// TestHTTP_PatchSettings_validationError ensures the store-level
// validation surface (negative timeout, unknown runner) is bubbled to
// the client as a 400 with a useful message rather than a 500. The
// SPA depends on this to show field-level errors.
func TestHTTP_PatchSettings_validationError(t *testing.T) {
	srv, _, _, _ := settingsTestServer(t)
	resp := mustSettingsHTTP(t, http.MethodPatch, srv.URL+"/settings",
		`{"max_run_duration_seconds":-1}`, http.StatusBadRequest)
	if len(resp) == 0 {
		t.Fatal("empty error body for invalid patch")
	}
}

// TestHTTP_PatchSettings_503WithoutAgent confirms the documented
// "agent control unavailable" branch: writes are blocked when no
// supervisor is wired, so we never persist a row the worker won't
// pick up.
func TestHTTP_PatchSettings_503WithoutAgent(t *testing.T) {
	srv, _ := settingsTestServerNoAgent(t)
	mustSettingsHTTP(t, http.MethodPatch, srv.URL+"/settings",
		`{"repo_root":"/tmp/x"}`, http.StatusServiceUnavailable)
}

// TestHTTP_PatchSettings_reloadFailureSurfaces500 protects the audit
// trail: if Reload fails after the row was written, the operator
// sees an error so they know the live worker is out of sync and can
// retry. Silent success here would mask divergence between settings
// and worker state.
func TestHTTP_PatchSettings_reloadFailureSurfaces500(t *testing.T) {
	srv, _, _, ctrl := settingsTestServer(t)
	e := errors.New("synthetic reload failure")
	ctrl.reloadErr.Store(&e)
	mustSettingsHTTP(t, http.MethodPatch, srv.URL+"/settings",
		`{"repo_root":"/tmp/x"}`, http.StatusInternalServerError)
}

// TestHTTP_ProbeCursor_returnsVersionFromControl pins the happy path
// for the SPA "Test cursor binary" button: the probe fan-outs runner
// id and binary path through to the supervisor and surfaces the
// version string verbatim.
func TestHTTP_ProbeCursor_returnsVersionFromControl(t *testing.T) {
	srv, _, _, ctrl := settingsTestServer(t)
	v := "cursor 0.42.1"
	ctrl.probeVersion.Store(&v)

	body := mustSettingsHTTP(t, http.MethodPost, srv.URL+"/settings/probe-cursor",
		`{"runner":"cursor","binary_path":"/usr/local/bin/cursor"}`, http.StatusOK)
	var resp probeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if !resp.OK || resp.Version != v {
		t.Errorf("resp = %+v, want OK=true Version=%q", resp, v)
	}
	if got := ctrl.lastBinary.Load(); got == nil || *got != "/usr/local/bin/cursor" {
		t.Errorf("binary path not forwarded: %v", got)
	}
}

// TestHTTP_ProbeCursor_returnsResolvedBinaryPath pins the contract
// surfaced by the SPA: the probe response carries the absolute path
// that was actually executed (PATH-resolved when the operator left
// the field blank), so the "Test cursor binary" success message can
// say "auto-detected at /usr/local/bin/cursor-agent" instead of just
// "OK". Without this field the operator has no way to tell what
// "auto-detect on PATH" actually resolved to.
func TestHTTP_ProbeCursor_returnsResolvedBinaryPath(t *testing.T) {
	srv, _, _, ctrl := settingsTestServer(t)
	v := "cursor 1.0"
	resolved := "/opt/local/bin/cursor-agent"
	ctrl.probeVersion.Store(&v)
	ctrl.probeResolved.Store(&resolved)

	body := mustSettingsHTTP(t, http.MethodPost, srv.URL+"/settings/probe-cursor",
		`{"runner":"cursor","binary_path":""}`, http.StatusOK)
	var resp probeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if !resp.OK {
		t.Fatalf("resp = %+v, want OK=true", resp)
	}
	if resp.BinaryPath != resolved {
		t.Errorf("BinaryPath = %q, want %q", resp.BinaryPath, resolved)
	}
}

// TestHTTP_ProbeCursor_failureReturnsOKfalseNot500 confirms the
// "best-effort surface" contract: a failing cursor binary returns
// 200 with ok=false so the SPA can show a friendly inline error
// instead of a generic toast.
func TestHTTP_ProbeCursor_failureReturnsOKfalseNot500(t *testing.T) {
	srv, _, _, ctrl := settingsTestServer(t)
	e := errors.New("cursor not installed")
	ctrl.probeErr.Store(&e)

	body := mustSettingsHTTP(t, http.MethodPost, srv.URL+"/settings/probe-cursor",
		`{}`, http.StatusOK)
	var resp probeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if resp.OK {
		t.Error("expected OK=false on probe failure")
	}
	if !strings.Contains(resp.Error, "cursor not installed") {
		t.Errorf("error not surfaced: %q", resp.Error)
	}
}

// TestHTTP_ProbeCursor_emptyBodyFallsBackToStoredValues guarantees
// the SPA can hit Test without retyping the stored binary path: an
// empty body is valid and the handler reads the current row to fill
// in the runner / binary fields.
func TestHTTP_ProbeCursor_emptyBodyFallsBackToStoredValues(t *testing.T) {
	srv, st, _, ctrl := settingsTestServer(t)
	if _, err := st.UpdateSettings(context.Background(), store.SettingsPatch{
		CursorBin: ptrStr("/seeded/bin/cursor"),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	v := "cursor 1.0"
	ctrl.probeVersion.Store(&v)

	mustSettingsHTTP(t, http.MethodPost, srv.URL+"/settings/probe-cursor", "", http.StatusOK)
	if got := ctrl.lastBinary.Load(); got == nil || *got != "/seeded/bin/cursor" {
		t.Errorf("did not fall back to stored binary: got=%v", got)
	}
	if got := ctrl.lastRunner.Load(); got == nil || *got != "cursor" {
		t.Errorf("did not fall back to stored runner: got=%v", got)
	}
}

// TestHTTP_ProbeCursor_chunkedBodyRespectsExplicitOverride pins the
// transport-encoding-agnostic decoding contract for
// POST /settings/probe-cursor: a JSON body delivered via HTTP/1.1
// chunked transfer-encoding (Transfer-Encoding: chunked, no
// Content-Length header — server-side r.ContentLength == -1) must be
// decoded just like a Content-Length-terminated body. Before the fix
// the handler gated its decode on `r.ContentLength > 0`, so a chunked
// POST silently dropped the body, fell through to the
// fall-back-to-stored-values branch, and probed whatever was sitting
// in app_settings instead of the explicit binary the caller asked for.
// Wrapping a strings.Reader in struct{ io.Reader }{...} hides the
// length-aware concrete type from net/http, which forces the client to
// emit chunked encoding (this is the documented contract on
// http.NewRequest).
func TestHTTP_ProbeCursor_chunkedBodyRespectsExplicitOverride(t *testing.T) {
	srv, _, _, ctrl := settingsTestServer(t)
	v := "cursor 9.9.9"
	ctrl.probeVersion.Store(&v)

	body := `{"runner":"cursor","binary_path":"/explicit/from/chunked"}`
	rdr := struct{ io.Reader }{strings.NewReader(body)}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/settings/probe-cursor", rdr)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if req.ContentLength != 0 {
		t.Fatalf("test setup: expected ContentLength==0 (chunked) on outgoing request, got %d", req.ContentLength)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	respBytes, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.StatusCode, respBytes)
	}
	if got := ctrl.lastBinary.Load(); got == nil || *got != "/explicit/from/chunked" {
		t.Errorf("chunked body ignored: lastBinary=%v want=%q", got, "/explicit/from/chunked")
	}
	if got := ctrl.lastRunner.Load(); got == nil || *got != "cursor" {
		t.Errorf("chunked runner ignored: lastRunner=%v want=%q", got, "cursor")
	}
}

// TestHTTP_CancelCurrentRun_publishesSSEWhenCancelled covers the
// documented "fan out so the SPA can flip the button" contract:
// returns the worker's cancel result and only publishes the SSE
// event when there was actually a run to cancel.
func TestHTTP_CancelCurrentRun_publishesSSEWhenCancelled(t *testing.T) {
	srv, _, hub, ctrl := settingsTestServer(t)
	ctrl.cancelResult.Store(true)
	ch, sub := hub.Subscribe()
	defer sub()

	body := mustSettingsHTTP(t, http.MethodPost, srv.URL+"/settings/cancel-current-run", "", http.StatusOK)
	var resp cancelRunResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if !resp.Cancelled {
		t.Error("expected cancelled=true")
	}
	got := summarize(drainSSE(t, ch, 1, 2*time.Second))
	mustEqualEvents(t, "POST /settings/cancel-current-run", got, []string{"agent_run_cancelled:"})
}

// TestHTTP_CancelCurrentRun_noopReturnsFalseAndNoSSE confirms the
// "nothing to cancel" branch: 200 with cancelled=false and no SSE
// noise. Without this the SPA would falsely flip the cancel UI on
// every click, even when the worker was idle.
func TestHTTP_CancelCurrentRun_noopReturnsFalseAndNoSSE(t *testing.T) {
	srv, _, hub, ctrl := settingsTestServer(t)
	ctrl.cancelResult.Store(false)
	ch, sub := hub.Subscribe()
	defer sub()

	body := mustSettingsHTTP(t, http.MethodPost, srv.URL+"/settings/cancel-current-run", "", http.StatusOK)
	if !strings.Contains(string(body), `"cancelled":false`) {
		t.Errorf("body=%s, want cancelled=false", body)
	}
	got := drainSSE(t, ch, 1, 200*time.Millisecond)
	if len(got) != 0 {
		t.Errorf("expected no SSE event when no run to cancel, got %d", len(got))
	}
}

// TestHTTP_CancelCurrentRun_503WithoutAgent matches the PATCH branch:
// no supervisor wired = endpoint disabled, never silently returns
// "cancelled=false" (which would lie to the operator).
func TestHTTP_CancelCurrentRun_503WithoutAgent(t *testing.T) {
	srv, _ := settingsTestServerNoAgent(t)
	mustSettingsHTTP(t, http.MethodPost, srv.URL+"/settings/cancel-current-run", "", http.StatusServiceUnavailable)
}

// helpers

func mustGetSettingsJSON(t *testing.T, url string, want int) []byte {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		t.Fatalf("GET %s status=%d want=%d body=%s", url, res.StatusCode, want, b)
	}
	return b
}

func mustPatchSettingsJSON(t *testing.T, url, body string, want int) []byte {
	t.Helper()
	return mustSettingsHTTP(t, http.MethodPatch, url, body, want)
}

func mustSettingsHTTP(t *testing.T, method, url, body string, want int) []byte {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, url, res.StatusCode, want, b)
	}
	return b
}

func ptrStr(s string) *string { return &s }
