package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/systemhealth"
	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/prometheus/client_golang/prometheus"
)

// systemHealthRaw mirrors the documented `GET /system/health` 200
// envelope. The shape is checked here (not inside the systemhealth
// package) so a future field rename in
// internal/systemhealth.Snapshot fails this test in the same PR as
// the docs/API-HTTP.md update.
type systemHealthRaw struct {
	Build         systemHealthBuildRaw  `json:"build"`
	UptimeSeconds float64               `json:"uptime_seconds"`
	Now           string                `json:"now"`
	HTTP          systemHealthHTTPRaw   `json:"http"`
	SSE           systemHealthSSERaw    `json:"sse"`
	DBPool        systemHealthDBPoolRaw `json:"db_pool"`
	Agent         systemHealthAgentRaw  `json:"agent"`
}

type systemHealthBuildRaw struct {
	Version   string `json:"version"`
	Revision  string `json:"revision"`
	GoVersion string `json:"go_version"`
}

type systemHealthHTTPRaw struct {
	InFlight        int64                       `json:"in_flight"`
	RequestsTotal   uint64                      `json:"requests_total"`
	RequestsByClass map[string]uint64           `json:"requests_by_class"`
	DurationSeconds systemHealthHTTPDurationRaw `json:"duration_seconds"`
}

type systemHealthHTTPDurationRaw struct {
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	Count uint64  `json:"count"`
}

type systemHealthSSERaw struct {
	Subscribers        int64  `json:"subscribers"`
	DroppedFramesTotal uint64 `json:"dropped_frames_total"`
}

type systemHealthDBPoolRaw struct {
	MaxOpenConnections       int64   `json:"max_open_connections"`
	OpenConnections          int64   `json:"open_connections"`
	InUseConnections         int64   `json:"in_use_connections"`
	IdleConnections          int64   `json:"idle_connections"`
	WaitCountTotal           uint64  `json:"wait_count_total"`
	WaitDurationSecondsTotal float64 `json:"wait_duration_seconds_total"`
}

type systemHealthAgentRaw struct {
	QueueDepth           int64             `json:"queue_depth"`
	QueueCapacity        int64             `json:"queue_capacity"`
	RunsTotal            uint64            `json:"runs_total"`
	RunsByTerminalStatus map[string]uint64 `json:"runs_by_terminal_status"`
	Paused               bool              `json:"paused"`
}

func newSystemHealthTestServer(t *testing.T, g systemhealth.Gather) *httptest.Server {
	t.Helper()
	db := tasktestdb.OpenSQLite(t)
	h := NewHandler(store.NewStore(db), NewSSEHub(), nil, WithSystemHealthGatherer(g))
	return httptest.NewServer(h)
}

// TestHTTP_systemHealth_envelopeShape pins the documented eight
// top-level keys of GET /system/health (build, uptime_seconds, now,
// http, sse, db_pool, agent — note `now` is wall clock, not an
// uptime alias). Adding or renaming a key fails this test in the
// same PR as docs/API-HTTP.md.
func TestHTTP_systemHealth_envelopeShape(t *testing.T) {
	srv := newSystemHealthTestServer(t, prometheus.NewPedanticRegistry())
	defer srv.Close()

	raw, _ := mustGetJSON(t, srv.URL, "/system/health")
	assertSystemHealthEnvelopeKeys(t, raw)

	var got systemHealthRaw
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if got.Build.Version == "" || got.Build.GoVersion == "" {
		t.Errorf("Build.Version/GoVersion must be populated, got %+v", got.Build)
	}
	if got.HTTP.RequestsByClass == nil {
		t.Fatalf("RequestsByClass must be a non-nil map: %s", raw)
	}
	for _, k := range []string{"2xx", "3xx", "4xx", "5xx", "other"} {
		if _, ok := got.HTTP.RequestsByClass[k]; !ok {
			t.Errorf("RequestsByClass missing seeded key %q: %s", k, raw)
		}
	}
	if got.Agent.RunsByTerminalStatus == nil {
		t.Fatalf("RunsByTerminalStatus must be a non-nil map: %s", raw)
	}
	for _, k := range []string{"succeeded", "failed", "aborted"} {
		if _, ok := got.Agent.RunsByTerminalStatus[k]; !ok {
			t.Errorf("RunsByTerminalStatus missing seeded key %q: %s", k, raw)
		}
	}
}

// TestHTTP_systemHealth_populated wires real metrics through the
// injected registry and verifies they surface verbatim in the JSON
// envelope. This is the end-to-end pin: a counter rename in any
// upstream metric will fail here.
func TestHTTP_systemHealth_populated(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()

	subs := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi", Name: "sse_subscribers", Help: ".",
	})
	queueDepth := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi", Name: "agent_queue_depth", Help: ".",
	})
	queueCap := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "taskapi", Name: "agent_queue_capacity", Help: ".",
	})
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "taskapi", Name: "http_requests_total", Help: ".",
	}, []string{"method", "route", "code"})
	reg.MustRegister(subs, queueDepth, queueCap, requests)

	subs.Set(2)
	queueDepth.Set(1)
	queueCap.Set(8)
	requests.WithLabelValues("GET", "/tasks", "200").Add(42)
	requests.WithLabelValues("GET", "/tasks/{id}", "404").Add(3)

	srv := newSystemHealthTestServer(t, reg)
	defer srv.Close()

	raw, res := mustGetJSON(t, srv.URL, "/system/health")
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}

	var got systemHealthRaw
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if got.SSE.Subscribers != 2 {
		t.Errorf("SSE.Subscribers: got %d want 2 (raw=%s)", got.SSE.Subscribers, raw)
	}
	if got.Agent.QueueDepth != 1 || got.Agent.QueueCapacity != 8 {
		t.Errorf("Agent: got depth=%d cap=%d want 1/8 (raw=%s)", got.Agent.QueueDepth, got.Agent.QueueCapacity, raw)
	}
	if got.HTTP.RequestsTotal != 45 {
		t.Errorf("HTTP.RequestsTotal: got %d want 45 (raw=%s)", got.HTTP.RequestsTotal, raw)
	}
	if got.HTTP.RequestsByClass["2xx"] != 42 || got.HTTP.RequestsByClass["4xx"] != 3 {
		t.Errorf("HTTP.RequestsByClass: got %+v (raw=%s)", got.HTTP.RequestsByClass, raw)
	}
}

// TestHTTP_systemHealth_agentPaused pins that the operator-facing
// AgentPaused flag from app_settings surfaces verbatim under
// `agent.paused` in GET /system/health. The flag is sourced from
// the singleton settings row (not Prometheus) precisely because the
// supervisor goes idle when paused, which means a counter-based
// signal would lag the real state. This test fails if the handler
// stops layering the field on top of the metric snapshot.
func TestHTTP_systemHealth_agentPaused(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := store.NewStore(db)
	h := NewHandler(s, NewSSEHub(), nil, WithSystemHealthGatherer(prometheus.NewPedanticRegistry()))
	srv := httptest.NewServer(h)
	defer srv.Close()

	rawDefault, _ := mustGetJSON(t, srv.URL, "/system/health")
	var beforeFlip systemHealthRaw
	if err := json.Unmarshal(rawDefault, &beforeFlip); err != nil {
		t.Fatalf("decode default: %v body=%s", err, rawDefault)
	}
	if beforeFlip.Agent.Paused {
		t.Fatalf("default app_settings.agent_paused should be false; got envelope %s", rawDefault)
	}

	paused := true
	if _, err := s.UpdateSettings(t.Context(), store.SettingsPatch{AgentPaused: &paused}); err != nil {
		t.Fatalf("flip AgentPaused=true: %v", err)
	}

	rawPaused, _ := mustGetJSON(t, srv.URL, "/system/health")
	var afterFlip systemHealthRaw
	if err := json.Unmarshal(rawPaused, &afterFlip); err != nil {
		t.Fatalf("decode paused: %v body=%s", err, rawPaused)
	}
	if !afterFlip.Agent.Paused {
		t.Fatalf("after AgentPaused=true the envelope should report paused=true; got %s", rawPaused)
	}
}

// TestHTTP_systemHealth_methodNotAllowed pins that only GET is
// accepted (the route is `GET /system/health` on Go 1.22 ServeMux).
func TestHTTP_systemHealth_methodNotAllowed(t *testing.T) {
	srv := newSystemHealthTestServer(t, prometheus.NewPedanticRegistry())
	defer srv.Close()

	res, err := http.Post(srv.URL+"/system/health", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST /system/health: status %d, want %d", res.StatusCode, http.StatusMethodNotAllowed)
	}
}

// TestHTTP_systemHealth_doesNotPublishSSE pins that the read does
// not fan out an SSE event — the operator-poll endpoint must stay
// silent on the hub or the SPA's react-query SSE invalidation would
// loop forever (poll → invalidate → poll).
func TestHTTP_systemHealth_doesNotPublishSSE(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	hub := NewSSEHub()
	h := NewHandler(store.NewStore(db), hub, nil, WithSystemHealthGatherer(prometheus.NewPedanticRegistry()))
	srv := httptest.NewServer(h)
	defer srv.Close()

	ch, unsub := hub.Subscribe()
	defer unsub()

	res, err := http.Get(srv.URL + "/system/health")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d want 200", res.StatusCode)
	}

	got := summarize(drainSSE(t, ch, 1, 200*time.Millisecond))
	mustEqualEvents(t, "GET /system/health", got, []string{})
}

func assertSystemHealthEnvelopeKeys(t *testing.T, raw []byte) {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	want := map[string]struct{}{
		"build": {}, "uptime_seconds": {}, "now": {},
		"http": {}, "sse": {}, "db_pool": {}, "agent": {},
	}
	for k := range want {
		if _, ok := top[k]; !ok {
			t.Errorf("GET /system/health 200 missing key %q (docs/API-HTTP.md): %s", k, raw)
		}
	}
	for k := range top {
		if _, ok := want[k]; !ok {
			t.Errorf("GET /system/health 200 unexpected key %q (docs/API-HTTP.md): %s", k, raw)
		}
	}
}
