package worker

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestParseCriteriaReport_missingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := parseCriteriaReport(dir, "cycle-1", map[string]struct{}{"a": {}})
	if !errors.Is(err, ErrCriteriaReportMissing) {
		t.Fatalf("got %v, want %v", err, ErrCriteriaReportMissing)
	}
}

func TestParseCriteriaReport_valid(t *testing.T) {
	dir := t.TempDir()
	cycleID := "cycle-1"
	if err := ensureReportCycleDir(dir, cycleID); err != nil {
		t.Fatal(err)
	}
	body := `{"criteria":[{"id":"a","claimed_done":true,"evidence":"did the thing"}]}`
	if err := os.WriteFile(criteriaReportPath(dir, cycleID), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := parseCriteriaReport(dir, cycleID, map[string]struct{}{"a": {}})
	if err != nil {
		t.Fatal(err)
	}
	if !out["a"].ClaimedDone || out["a"].Evidence == "" {
		t.Fatalf("unexpected entry: %+v", out["a"])
	}
}

func TestParseCriteriaReport_duplicateID(t *testing.T) {
	dir := t.TempDir()
	cycleID := "cycle-1"
	if err := ensureReportCycleDir(dir, cycleID); err != nil {
		t.Fatal(err)
	}
	rep := criteriaReport{Criteria: []criteriaReportEntry{
		{ID: "a", ClaimedDone: true, Evidence: "x"},
		{ID: "a", ClaimedDone: true, Evidence: "y"},
	}}
	b, _ := json.Marshal(rep)
	if err := os.WriteFile(criteriaReportPath(dir, cycleID), b, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := parseCriteriaReport(dir, cycleID, map[string]struct{}{"a": {}})
	if !errors.Is(err, ErrCriteriaReportInvalid) {
		t.Fatalf("got %v, want invalid", err)
	}
}
