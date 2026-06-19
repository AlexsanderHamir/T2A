package verify

import (
	"strconv"
	"strings"
	"testing"
)

func TestFormatFailedReason_NoFailures_BareReason(t *testing.T) {
	got := FormatFailedReason(nil, nil)
	if got != FailedReasonPrefix {
		t.Fatalf("empty verdicts -> bare reason; got %q", got)
	}
}

func TestFormatFailedReason_SortsAndDedupes(t *testing.T) {
	verdicts := []Verdict{
		{ID: "c-zebra", Passed: false},
		{ID: "c-alpha", Passed: false},
		{ID: "c-beta", Passed: true},
		{ID: "c-alpha", Passed: false},
	}
	got := FormatFailedReason(verdicts, nil)
	want := FailedReasonPrefix + ":c-alpha,c-zebra"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatFailedReason_ExcludesLockedPasses(t *testing.T) {
	verdicts := []Verdict{
		{ID: "c1", Passed: false},
		{ID: "c2", Passed: false},
	}
	locked := map[string]Verdict{
		"c1": {ID: "c1", Passed: true},
	}
	got := FormatFailedReason(verdicts, locked)
	want := FailedReasonPrefix + ":c2"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatFailedReason_TruncatesUnder256(t *testing.T) {
	var verdicts []Verdict
	for i := 0; i < 200; i++ {
		verdicts = append(verdicts, Verdict{
			ID:     "criterion-with-a-fairly-long-id-suffix-" + strconv.Itoa(i),
			Passed: false,
		})
	}
	got := FormatFailedReason(verdicts, nil)
	if len(got) > 256 {
		t.Fatalf("len(got) = %d, want <= 256; got=%q", len(got), got)
	}
	if !strings.HasPrefix(got, FailedReasonPrefix+":") {
		t.Fatalf("prefix must remain verification_failed:; got %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix on truncated reason; got %q", got)
	}
}

func TestFormatFailedReason_PrefixStabilityContract(t *testing.T) {
	cases := [][]Verdict{
		nil,
		{{ID: "c1", Passed: false}},
		{{ID: "c1", Passed: true}, {ID: "c2", Passed: false}},
	}
	for i, verdicts := range cases {
		got := FormatFailedReason(verdicts, nil)
		if !strings.HasPrefix(got, FailedReasonPrefix) {
			t.Errorf("case %d: %q must start with verification_failed", i, got)
		}
	}
}
