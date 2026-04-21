package cycles

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestTerminatedPayload_includesFailureSummaryWhenSet(t *testing.T) {
	t.Parallel()
	c := &domain.TaskCycle{
		ID:         "c1",
		AttemptSeq: 1,
		Status:     domain.CycleStatusFailed,
	}
	raw, err := terminatedPayload(c, "runner_non_zero_exit", "Operator-facing text")
	if err != nil {
		t.Fatalf("terminatedPayload: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["reason"] != "runner_non_zero_exit" {
		t.Fatalf("reason: got %#v", got["reason"])
	}
	if got["failure_summary"] != "Operator-facing text" {
		t.Fatalf("failure_summary: got %#v", got["failure_summary"])
	}
}

func TestTerminatedPayload_omitsEmptyFailureSummary(t *testing.T) {
	t.Parallel()
	c := &domain.TaskCycle{
		ID:         "c1",
		AttemptSeq: 1,
		Status:     domain.CycleStatusFailed,
	}
	raw, err := terminatedPayload(c, "runner_timeout", "")
	if err != nil {
		t.Fatalf("terminatedPayload: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := got["failure_summary"]; ok {
		t.Fatalf("expected failure_summary omitted, got %#v", got)
	}
}

func TestPhaseTerminatedPayload_includesDetails(t *testing.T) {
	t.Parallel()

	ph := domain.TaskCyclePhase{
		Phase:       domain.PhaseExecute,
		PhaseSeq:    2,
		Status:      domain.PhaseStatusFailed,
		DetailsJSON: []byte(`{"stderr_tail":"boom","usage":{"x":1}}`),
	}
	s := "cursor: exit 1"
	ph.Summary = &s

	raw, err := phaseTerminatedPayload("cycle-1", &ph)
	if err != nil {
		t.Fatalf("phaseTerminatedPayload: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	details, ok := got["details"].(map[string]any)
	if !ok {
		t.Fatalf("details missing or wrong type: %v", got["details"])
	}
	if details["stderr_tail"] != "boom" {
		t.Fatalf("stderr_tail: got %#v", details["stderr_tail"])
	}
}

func TestTruncateStringRunes_addsEllipsisWhenOverLimit(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteRune('x')
	}
	out := truncateStringRunes(b.String(), 10)
	if utf8.RuneCountInString(out) != 11 {
		t.Fatalf("got %d runes want 11 (10 + ellipsis)", utf8.RuneCountInString(out))
	}
	if !strings.HasSuffix(out, "…") {
		t.Fatalf("expected ellipsis suffix: %q", out)
	}
}
