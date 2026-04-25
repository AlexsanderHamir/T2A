package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
)

// rumEvent is one Real-User-Monitoring data point shipped by the SPA
// to POST /v1/rum. Timing fields use seconds (float64) so the wire is
// human-readable in logs and natively compatible with Prometheus
// histogram observation; the SPA divides by 1000 before sending.
//
// The discriminator is `type` (mirrors the SSE wire shape so the docs
// reuse the same vocabulary):
//
//   - mutation_started: a mutation hook fired (no timing yet).
//   - mutation_optimistic_applied: the optimistic onMutate produced
//     visible UI change. duration_seconds is the click→render
//     latency (start → setQueryData).
//   - mutation_settled: the server returned 2xx OR a known business
//     error. duration_seconds is the click→server-confirmed latency.
//     status_code is the HTTP status.
//   - mutation_rolled_back: onError ran; the cache snapshot was
//     restored. duration_seconds is the click→rollback latency.
//   - sse_reconnected: the EventSource transport reconnected
//     (browser auto-retry or our explicit reconnect after a
//     resync directive). duration_seconds is gap_to_reconnect.
//   - sse_resync_received: the client received a resync directive.
//     No duration.
//   - web_vitals: LCP / INP / CLS sample. The web-vitals lib emits
//     one event per metric so we forward the metric name in `name`
//     and the value in `value`.
//
// Unknown `type` values are accepted by the parser (forward-compat)
// and dropped server-side so deploying a new SPA event before the
// server knows about it does not 400 the whole batch.
type rumEvent struct {
	Type            string  `json:"type"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	MutationKind    string  `json:"mutation_kind,omitempty"`
	StatusCode      int     `json:"status_code,omitempty"`
	Name            string  `json:"name,omitempty"`
	Value           float64 `json:"value,omitempty"`
}

// rumBatch is the array body sent by the SPA. The endpoint is a
// "fire and forget" beacon: it never returns a payload, only a 204.
// `events` is documented as the only key so future fields (e.g. a
// session id) can be added without breaking the parser — extra fields
// are tolerated by the standard json decoder.
type rumBatch struct {
	Events []rumEvent `json:"events"`
}

// maxRUMBatchSize caps the number of events one beacon can carry.
// 100 is comfortably above the 10s flush window even under heavy
// activity (typical user emits <5 mutations per 10s) and keeps the
// JSON parse cost bounded if a buggy SPA sends a runaway batch.
const maxRUMBatchSize = 100

// maxRUMBatchBytes caps the body size. The middleware-level body cap
// covers this in production but we keep a defensive local check so
// tests that bypass the middleware still see consistent semantics.
const maxRUMBatchBytes = 64 * 1024

// validRUMTypes is the set of event types the server promotes to a
// metric. Extras are silently dropped (forward-compat); unknowns are
// counted in the dropped counter so "the SPA is sending something we
// don't understand" shows up as a metric instead of a guess.
var validRUMTypes = map[string]struct{}{
	"mutation_started":            {},
	"mutation_optimistic_applied": {},
	"mutation_settled":            {},
	"mutation_rolled_back":        {},
	"sse_reconnected":             {},
	"sse_resync_received":         {},
	"web_vitals":                  {},
}

// validWebVitalNames pins the metric names we accept on web_vitals
// events. Mirrors the web-vitals npm package output (LCP, INP, CLS,
// plus the older FID we keep accepting for transition reasons).
var validWebVitalNames = map[string]struct{}{
	"LCP":  {},
	"INP":  {},
	"CLS":  {},
	"FID":  {},
	"FCP":  {},
	"TTFB": {},
}

// postRUM accepts a batch of RUM events from the SPA, validates them
// without doing per-event I/O, and folds each one into a Prometheus
// counter / histogram. Always returns 204 on success — the SPA uses
// `navigator.sendBeacon`, which doesn't expose a response body, so
// returning JSON would be wasted bytes on the wire.
//
// Validation strategy: parse the whole batch, drop unknown types
// (count them in `taskapi_rum_events_dropped_total`), fold the rest.
// A single malformed event does NOT 400 the batch — we drop it and
// keep going so a future SPA shipping a new event type cannot
// silently take down the whole RUM pipeline.
func (h *Handler) postRUM(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.rum"
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op, "rum_post", "")

	if r.ContentLength > maxRUMBatchBytes {
		writeJSONError(w, r, op, http.StatusRequestEntityTooLarge,
			"rum batch too large")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxRUMBatchBytes+1))
	if err != nil {
		writeJSONError(w, r, op, http.StatusBadRequest, "rum read failed")
		return
	}
	if len(body) == 0 {
		writeJSONError(w, r, op, http.StatusBadRequest, "rum batch empty")
		return
	}
	if len(body) > maxRUMBatchBytes {
		writeJSONError(w, r, op, http.StatusRequestEntityTooLarge,
			"rum batch too large")
		return
	}

	var batch rumBatch
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&batch); err != nil {
		// Tolerate unknown top-level fields (forward-compat for
		// session ids / SPA build tags). Re-decode without the
		// strict flag.
		var lenient rumBatch
		if jerr := json.Unmarshal(body, &lenient); jerr != nil {
			writeJSONError(w, r, op, http.StatusBadRequest,
				"rum batch must be JSON {events:[…]}")
			return
		}
		batch = lenient
	}
	if len(batch.Events) == 0 {
		writeJSONError(w, r, op, http.StatusBadRequest,
			"rum batch must contain at least one event")
		return
	}
	if len(batch.Events) > maxRUMBatchSize {
		writeJSONError(w, r, op, http.StatusBadRequest,
			"rum batch too large (max 100 events)")
		return
	}

	accepted := 0
	dropped := 0
	for _, ev := range batch.Events {
		if !foldRUMEvent(ev) {
			dropped++
			continue
		}
		accepted++
	}
	middleware.RecordRUMAccepted(accepted)
	middleware.RecordRUMDropped(dropped)
	if accepted > 0 || dropped > 0 {
		slog.Debug("rum batch processed",
			"cmd", calltrace.LogCmd, "operation", op,
			"accepted", accepted, "dropped", dropped, "total", len(batch.Events))
	}
	w.WriteHeader(http.StatusNoContent)
}

// foldRUMEvent updates Prometheus state for one event. Returns false
// if the event was dropped (unknown type, invalid duration, unknown
// web vital name) so postRUM can count the drop separately.
func foldRUMEvent(ev rumEvent) bool {
	if _, ok := validRUMTypes[ev.Type]; !ok {
		return false
	}
	switch ev.Type {
	case "mutation_started":
		middleware.RecordRUMMutationStarted(ev.MutationKind)
		return true
	case "mutation_optimistic_applied":
		if !validDurationSeconds(ev.DurationSeconds) {
			return false
		}
		middleware.RecordRUMMutationOptimisticApplied(ev.MutationKind, ev.DurationSeconds)
		return true
	case "mutation_settled":
		if !validDurationSeconds(ev.DurationSeconds) {
			return false
		}
		middleware.RecordRUMMutationSettled(ev.MutationKind, rumStatusBucket(ev.StatusCode), ev.DurationSeconds)
		return true
	case "mutation_rolled_back":
		if !validDurationSeconds(ev.DurationSeconds) {
			return false
		}
		middleware.RecordRUMMutationRolledBack(ev.MutationKind, ev.DurationSeconds)
		return true
	case "sse_reconnected":
		// Allow zero duration (browser-initiated reconnect that
		// happened too fast to time precisely).
		if ev.DurationSeconds < 0 || ev.DurationSeconds > 600 {
			return false
		}
		middleware.RecordRUMSSEReconnected(ev.DurationSeconds)
		return true
	case "sse_resync_received":
		middleware.RecordRUMSSEResyncReceived()
		return true
	case "web_vitals":
		if _, ok := validWebVitalNames[ev.Name]; !ok {
			return false
		}
		// Web-vitals values: LCP/FCP/TTFB/INP/FID are ms; CLS is
		// unitless layout-shift score. We pass the raw value to a
		// vector so dashboards can pick the right unit per metric.
		middleware.RecordRUMWebVital(ev.Name, ev.Value)
		return true
	}
	return false
}

// validDurationSeconds rejects negative or absurdly-long observations.
// 600 s upper bound covers even pathologically slow networks; anything
// above is almost certainly a clock skew / browser tab-suspended bug.
func validDurationSeconds(v float64) bool {
	return v >= 0 && v <= 600
}

// rumStatusBucket collapses the HTTP status code into a low-cardinality
// bucket label for the histogram. Without this we'd get one histogram
// series per (kind, exact status) — fine for 200/400/404/500 but bad
// when a misbehaving server returns 999 ad-hoc codes.
func rumStatusBucket(code int) string {
	switch {
	case code == 0:
		return "unknown"
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500 && code < 600:
		return "5xx"
	default:
		return "unknown"
	}
}
