package runner_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// TestRequest_jsonShape pins the on-the-wire keys for Request. Adapters that
// serialise a Request must produce exactly these keys in any order.
func TestRequest_jsonShape(t *testing.T) {
	t.Parallel()

	req := runner.Request{
		TaskID:     "11111111-1111-4111-8111-111111111111",
		AttemptSeq: 3,
		Phase:      domain.PhaseExecute,
		Prompt:     "do the thing",
		WorkingDir: "/repo",
		Timeout:    5 * time.Second,
		Env:        map[string]string{"PATH": "/usr/bin"},
	}

	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal generic: %v", err)
	}

	wantKeys := []string{"task_id", "attempt_seq", "phase", "prompt", "working_dir", "timeout_ns", "env"}
	for _, k := range wantKeys {
		if _, ok := generic[k]; !ok {
			t.Errorf("missing JSON key %q in %s", k, raw)
		}
	}
	for k := range generic {
		if !contains(wantKeys, k) {
			t.Errorf("unexpected JSON key %q (full payload: %s)", k, raw)
		}
	}

	if got := generic["phase"].(string); got != "execute" {
		t.Errorf("phase wire value: got %q want %q", got, "execute")
	}
	if got := generic["timeout_ns"].(float64); got != float64(5*time.Second) {
		t.Errorf("timeout_ns: got %v want %v", got, float64(5*time.Second))
	}
}

// TestRequest_jsonRoundtrip checks Request survives a full round-trip with
// no field loss.
func TestRequest_jsonRoundtrip(t *testing.T) {
	t.Parallel()

	want := runner.Request{
		TaskID:     "22222222-2222-4222-8222-222222222222",
		AttemptSeq: 7,
		Phase:      domain.PhaseDiagnose,
		Prompt:     "diagnose the failure",
		WorkingDir: "/work",
		Timeout:    250 * time.Millisecond,
		Env:        map[string]string{"PATH": "/bin", "HOME": "/home/runner"},
	}

	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got runner.Request
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.TaskID != want.TaskID || got.AttemptSeq != want.AttemptSeq ||
		got.Phase != want.Phase || got.Prompt != want.Prompt ||
		got.WorkingDir != want.WorkingDir || got.Timeout != want.Timeout {
		t.Errorf("scalar mismatch: got %+v want %+v", got, want)
	}
	if len(got.Env) != len(want.Env) {
		t.Fatalf("env length: got %d want %d", len(got.Env), len(want.Env))
	}
	for k, v := range want.Env {
		if got.Env[k] != v {
			t.Errorf("env[%q]: got %q want %q", k, got.Env[k], v)
		}
	}
}

// TestResult_jsonShape_omitempty ensures optional fields drop out of the
// payload when zero. Audit-log consumers rely on this to keep cycle/phase
// rows compact.
func TestResult_jsonShape_omitempty(t *testing.T) {
	t.Parallel()

	res := runner.Result{Status: domain.PhaseStatusSucceeded}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := generic["status"]; !ok {
		t.Errorf("status must always serialise: %s", raw)
	}
	for _, k := range []string{"summary", "details", "raw_output", "truncated"} {
		if _, ok := generic[k]; ok {
			t.Errorf("zero-value field %q must omit from payload: %s", k, raw)
		}
	}
}

// TestResult_jsonShape_full pins the keys when every field is populated.
func TestResult_jsonShape_full(t *testing.T) {
	t.Parallel()

	res := runner.Result{
		Status:    domain.PhaseStatusFailed,
		Summary:   "exit 1",
		Details:   json.RawMessage(`{"exit_code":1}`),
		RawOutput: "boom",
		Truncated: true,
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	wantKeys := []string{"status", "summary", "details", "raw_output", "truncated"}
	for _, k := range wantKeys {
		if _, ok := generic[k]; !ok {
			t.Errorf("missing JSON key %q in %s", k, raw)
		}
	}
	for k := range generic {
		if !contains(wantKeys, k) {
			t.Errorf("unexpected JSON key %q (full payload: %s)", k, raw)
		}
	}
}

// TestNewResult_passesSmallValuesThrough is the happy path: nothing is
// clipped and Truncated stays false.
func TestNewResult_passesSmallValuesThrough(t *testing.T) {
	t.Parallel()

	details := json.RawMessage(`{"ok":true}`)
	res := runner.NewResult(domain.PhaseStatusSucceeded, "short summary", details, "small output")

	if res.Truncated {
		t.Errorf("Truncated must be false for under-budget values")
	}
	if res.Summary != "short summary" {
		t.Errorf("Summary mutated: got %q", res.Summary)
	}
	if string(res.Details) != string(details) {
		t.Errorf("Details mutated: got %s want %s", res.Details, details)
	}
	if res.RawOutput != "small output" {
		t.Errorf("RawOutput mutated: got %q", res.RawOutput)
	}
	if res.Status != domain.PhaseStatusSucceeded {
		t.Errorf("Status mutated: got %q", res.Status)
	}
}

// TestNewResult_clipsSummaryToMaxRunes asserts rune-correct clipping (no
// mid-codepoint slice on multi-byte input).
func TestNewResult_clipsSummaryToMaxRunes(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("é", runner.MaxSummaryRunes+50)
	res := runner.NewResult(domain.PhaseStatusSucceeded, long, nil, "")

	if !res.Truncated {
		t.Errorf("Truncated must be true when Summary is clipped")
	}
	gotRunes := utf8.RuneCountInString(res.Summary)
	if gotRunes != runner.MaxSummaryRunes {
		t.Errorf("Summary rune count: got %d want %d", gotRunes, runner.MaxSummaryRunes)
	}
	if !utf8.ValidString(res.Summary) {
		t.Errorf("clipped Summary is not valid UTF-8")
	}
}

// TestNewResult_clipsRawOutputToCap asserts the trailing-bytes policy and
// that the result is still valid UTF-8 even when the cap lands inside a
// multi-byte sequence.
func TestNewResult_clipsRawOutputToCap(t *testing.T) {
	t.Parallel()

	prefix := strings.Repeat("a", runner.MaxResultRawOutputBytes)
	tail := "TAIL_MARKER"
	full := prefix + tail
	res := runner.NewResult(domain.PhaseStatusFailed, "", nil, full)

	if !res.Truncated {
		t.Errorf("Truncated must be true when RawOutput is clipped")
	}
	if len(res.RawOutput) > runner.MaxResultRawOutputBytes {
		t.Errorf("RawOutput byte length %d exceeds cap %d",
			len(res.RawOutput), runner.MaxResultRawOutputBytes)
	}
	if !strings.HasSuffix(res.RawOutput, tail) {
		t.Errorf("trailing bytes lost: tail %q not in clipped output", tail)
	}
	if !utf8.ValidString(res.RawOutput) {
		t.Errorf("clipped RawOutput is not valid UTF-8")
	}
}

// TestNewResult_clipsRawOutputAtUTF8Boundary asserts that a multi-byte
// sequence straddling the cap is dropped cleanly rather than producing a
// half-rune.
func TestNewResult_clipsRawOutputAtUTF8Boundary(t *testing.T) {
	t.Parallel()

	// "é" is 2 bytes. Build a body that pushes a "é" across the boundary.
	body := strings.Repeat("é", runner.MaxResultRawOutputBytes/2+10)
	res := runner.NewResult(domain.PhaseStatusFailed, "", nil, body)

	if !res.Truncated {
		t.Errorf("Truncated must be true")
	}
	if !utf8.ValidString(res.RawOutput) {
		t.Errorf("RawOutput is not valid UTF-8 after boundary-snap clip")
	}
}

// TestNewResult_clipsDetailsToSentinel asserts oversized Details are
// replaced with a parseable sentinel, never raw bytes truncated mid-JSON.
func TestNewResult_clipsDetailsToSentinel(t *testing.T) {
	t.Parallel()

	bigPayload := make([]byte, 0, runner.MaxResultDetailsBytes+1024)
	bigPayload = append(bigPayload, '"')
	bigPayload = append(bigPayload, strings.Repeat("x", runner.MaxResultDetailsBytes+1000)...)
	bigPayload = append(bigPayload, '"')
	res := runner.NewResult(domain.PhaseStatusFailed, "", json.RawMessage(bigPayload), "")

	if !res.Truncated {
		t.Errorf("Truncated must be true when Details is clipped")
	}
	if len(res.Details) > runner.MaxResultDetailsBytes {
		t.Errorf("clipped Details exceeds cap: %d > %d",
			len(res.Details), runner.MaxResultDetailsBytes)
	}
	var parsed struct {
		Truncated     bool `json:"truncated"`
		OriginalBytes int  `json:"original_bytes"`
	}
	if err := json.Unmarshal(res.Details, &parsed); err != nil {
		t.Fatalf("sentinel must be valid JSON, got %s (err=%v)", res.Details, err)
	}
	if !parsed.Truncated {
		t.Errorf("sentinel must carry truncated=true: %s", res.Details)
	}
	if parsed.OriginalBytes != len(bigPayload) {
		t.Errorf("sentinel original_bytes: got %d want %d",
			parsed.OriginalBytes, len(bigPayload))
	}
}

// TestNewResult_clipsInvalidJSONDetailsToSentinel pins the
// JSON-validity contract on Result.Details. The doc on
// runner.NewResult says: "consumers always see well-formed JSON".
// Before the fix, clipDetails only checked size: an under-cap but
// MALFORMED Details payload (e.g. a third-party adapter that hands
// in a `}` short of a valid object, a payload with a trailing comma,
// a half-finished string literal — all things that can happen when
// an adapter assembles a payload from substring concatenation rather
// than encoding/json.Marshal) flowed through unchanged. Downstream
// consumers (worker dual-write into TaskCyclePhase.MetaJSON, the
// SPA reading /tasks/{id}/cycles/{cycleId}/phases/{seq}, any future
// log shipper that re-decodes the audit row) then either crashed
// with a JSON decode error or silently passed garbage through.
//
// The fix routes any input that is not json.Valid through the same
// sentinel as oversized payloads (truncated=true, original_bytes=N)
// so consumers can distinguish "no details" (nil) from "had details
// but they were lost to size or invalidity" (sentinel) without ever
// having to handle malformed JSON.
func TestNewResult_clipsInvalidJSONDetailsToSentinel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
	}{
		{"truncated_object", `{"a":1`},
		{"trailing_comma", `{"a":1,}`},
		{"unterminated_string", `{"a":"hello`},
		{"raw_garbage", `not json at all`},
		{"single_brace", `}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := runner.NewResult(domain.PhaseStatusFailed, "",
				json.RawMessage(tc.input), "")
			if !res.Truncated {
				t.Errorf("Truncated must be true when invalid Details is replaced with sentinel; got Details=%q", res.Details)
			}
			if !json.Valid(res.Details) {
				t.Errorf("Details after NewResult must be valid JSON; got %q", res.Details)
			}
			var parsed struct {
				Truncated     bool `json:"truncated"`
				OriginalBytes int  `json:"original_bytes"`
			}
			if err := json.Unmarshal(res.Details, &parsed); err != nil {
				t.Fatalf("sentinel must unmarshal: %v body=%s", err, res.Details)
			}
			if !parsed.Truncated {
				t.Errorf("sentinel must carry truncated=true: %s", res.Details)
			}
			if parsed.OriginalBytes != len(tc.input) {
				t.Errorf("sentinel original_bytes: got %d want %d (input=%q)",
					parsed.OriginalBytes, len(tc.input), tc.input)
			}
		})
	}
}

// TestNewResult_passesValidNonObjectDetailsThrough confirms the
// validity check intentionally permits valid non-object JSON values
// (top-level arrays, strings, numbers, null, true/false). The
// Details field is documented as "JSON-safe" / "well-formed JSON",
// not "always an object" — the per-field "object only" invariant
// belongs to handler.normalizeJSONObjectForResponse, not the runner.
// This test guards against an over-eager fix that would coerce
// `["a","b"]` or `null` to the sentinel.
func TestNewResult_passesValidNonObjectDetailsThrough(t *testing.T) {
	t.Parallel()

	cases := []string{
		`["a","b"]`,
		`null`,
		`42`,
		`"a string"`,
		`true`,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			res := runner.NewResult(domain.PhaseStatusSucceeded, "",
				json.RawMessage(in), "")
			if res.Truncated {
				t.Errorf("Truncated must be false for valid JSON %q", in)
			}
			if string(res.Details) != in {
				t.Errorf("Details mutated for valid JSON: got %q want %q",
					res.Details, in)
			}
		})
	}
}

// TestNewResult_atCapBoundary asserts a value of exactly MaxBytes bytes is
// NOT clipped. (Cap is inclusive of the boundary.)
func TestNewResult_atCapBoundary(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("a", runner.MaxResultRawOutputBytes)
	res := runner.NewResult(domain.PhaseStatusSucceeded, "", nil, body)
	if res.Truncated {
		t.Errorf("value of exactly MaxBytes must not be clipped")
	}
	if res.RawOutput != body {
		t.Errorf("RawOutput mutated at cap boundary")
	}
}

// TestErrSentinels_distinct guards against accidental sentinel collapse so
// errors.Is in the worker keeps differentiating timeout / non-zero / invalid.
func TestErrSentinels_distinct(t *testing.T) {
	t.Parallel()

	all := []error{runner.ErrTimeout, runner.ErrNonZeroExit, runner.ErrInvalidOutput}
	for i, a := range all {
		for j, b := range all {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("sentinel %d (%v) is wrongly matched by %d (%v)", i, a, j, b)
			}
		}
	}
}

// TestRunnerInterface_compileTime asserts at least one type satisfies the
// interface so signature drift surfaces here, not in callers.
func TestRunnerInterface_compileTime(t *testing.T) {
	t.Parallel()
	var _ runner.Runner = runnerfake.New()
}

// TestRunnerFake_returnsScriptedResult covers the success path of the fake
// (used by every later test in the worker plan).
func TestRunnerFake_returnsScriptedResult(t *testing.T) {
	t.Parallel()

	want := runner.NewResult(domain.PhaseStatusSucceeded, "ok", nil, "")
	r := runnerfake.New()
	r.Script("task-A", domain.PhaseExecute, want)

	got, err := r.Run(context.Background(), runner.Request{
		TaskID:     "task-A",
		AttemptSeq: 1,
		Phase:      domain.PhaseExecute,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Status != want.Status || got.Summary != want.Summary {
		t.Errorf("got %+v want %+v", got, want)
	}
	if calls := r.Calls(); len(calls) != 1 {
		t.Fatalf("Calls len: got %d want 1", len(calls))
	}
}

// TestRunnerFake_unknownScriptReturnsErrInvalidOutput keeps the fake honest:
// missing scripts must be loud failures so worker tests don't pass on the
// wrong code path.
func TestRunnerFake_unknownScriptReturnsErrInvalidOutput(t *testing.T) {
	t.Parallel()

	r := runnerfake.New()
	_, err := r.Run(context.Background(), runner.Request{
		TaskID: "missing", Phase: domain.PhaseDiagnose,
	})
	if !errors.Is(err, runner.ErrInvalidOutput) {
		t.Errorf("got %v want errors.Is(_, ErrInvalidOutput)", err)
	}
}

// TestRunnerFake_failWithErr propagates a typed error to the caller.
func TestRunnerFake_failWithErr(t *testing.T) {
	t.Parallel()

	r := runnerfake.New()
	r.Fail("task-B", domain.PhaseVerify, runner.ErrNonZeroExit)
	_, err := r.Run(context.Background(), runner.Request{TaskID: "task-B", Phase: domain.PhaseVerify})
	if !errors.Is(err, runner.ErrNonZeroExit) {
		t.Errorf("got %v want errors.Is(_, ErrNonZeroExit)", err)
	}
}

// TestRunnerFake_failWithResult covers the worker's "partial result + typed
// error" contract (e.g. ErrNonZeroExit alongside captured RawOutput).
func TestRunnerFake_failWithResult(t *testing.T) {
	t.Parallel()

	partial := runner.NewResult(domain.PhaseStatusFailed, "exit 2", nil, "stderr blob")
	r := runnerfake.New()
	r.FailWithResult("task-C", domain.PhaseExecute, partial, runner.ErrNonZeroExit)

	got, err := r.Run(context.Background(), runner.Request{TaskID: "task-C", Phase: domain.PhaseExecute})
	if !errors.Is(err, runner.ErrNonZeroExit) {
		t.Fatalf("err: got %v want ErrNonZeroExit", err)
	}
	if got.Status != domain.PhaseStatusFailed || got.RawOutput != "stderr blob" {
		t.Errorf("partial Result lost: got %+v", got)
	}
}

// TestRunnerFake_cancelledContextReturnsErrTimeout proves the fake honours
// context cancellation through the typed-error channel, so worker tests can
// exercise timeout paths without spinning a real CLI.
func TestRunnerFake_cancelledContextReturnsErrTimeout(t *testing.T) {
	t.Parallel()

	r := runnerfake.New()
	r.Script("task-D", domain.PhaseExecute, runner.Result{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := r.Run(ctx, runner.Request{TaskID: "task-D", Phase: domain.PhaseExecute})
	if !errors.Is(err, runner.ErrTimeout) {
		t.Errorf("got %v want errors.Is(_, ErrTimeout)", err)
	}
}

// TestRunnerFake_NameVersionDefaultsAndOverrides documents that Name and
// Version can be customised so adapter-conformance tests in later stages
// can pin MetaJSON values.
func TestRunnerFake_NameVersionDefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	r := runnerfake.New()
	if r.Name() != "fake" || r.Version() != "v0" {
		t.Errorf("defaults: name=%q version=%q", r.Name(), r.Version())
	}
	r.WithName("cursor-fake").WithVersion("0.42.0")
	if r.Name() != "cursor-fake" || r.Version() != "0.42.0" {
		t.Errorf("overrides not applied: name=%q version=%q", r.Name(), r.Version())
	}
}

// TestRunnerFake_Reset clears recorded calls and scripts.
func TestRunnerFake_Reset(t *testing.T) {
	t.Parallel()

	r := runnerfake.New()
	r.Script("task-E", domain.PhaseExecute, runner.Result{Status: domain.PhaseStatusSucceeded})
	if _, err := r.Run(context.Background(), runner.Request{TaskID: "task-E", Phase: domain.PhaseExecute}); err != nil {
		t.Fatalf("seed Run: %v", err)
	}
	if len(r.Calls()) != 1 {
		t.Fatalf("seed call count")
	}

	r.Reset()
	if len(r.Calls()) != 0 {
		t.Errorf("Calls not cleared by Reset")
	}
	_, err := r.Run(context.Background(), runner.Request{TaskID: "task-E", Phase: domain.PhaseExecute})
	if !errors.Is(err, runner.ErrInvalidOutput) {
		t.Errorf("script not cleared by Reset: got %v", err)
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
