package reports

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseCriteriaReport_missingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := ParseCriteriaReport(dir, "cycle-1", map[string]struct{}{"a": {}})
	if !errors.Is(err, ErrCriteriaReportMissing) {
		t.Fatalf("got %v, want %v", err, ErrCriteriaReportMissing)
	}
}

func TestParseCriteriaReport_valid(t *testing.T) {
	dir := t.TempDir()
	cycleID := "cycle-1"
	if err := EnsureReportCycleDir(dir, cycleID); err != nil {
		t.Fatal(err)
	}
	body := `{"schema_version":1,"criteria":[{"id":"a","claimed_done":true,"evidence":"did the thing"}]}`
	if err := os.WriteFile(CriteriaReportPath(dir, cycleID), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := ParseCriteriaReport(dir, cycleID, map[string]struct{}{"a": {}})
	if err != nil {
		t.Fatal(err)
	}
	if !out["a"].ClaimedDone || out["a"].Evidence == "" {
		t.Fatalf("unexpected entry: %+v", out["a"])
	}
}

func TestParseCriteriaReport_goldenV1(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("testdata", "criteria_report_v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cycleID := "cycle-golden"
	if err := EnsureReportCycleDir(dir, cycleID); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(CriteriaReportPath(dir, cycleID), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := ParseCriteriaReport(dir, cycleID, map[string]struct{}{"criterion-a": {}})
	if err != nil {
		t.Fatal(err)
	}
	if !out["criterion-a"].ClaimedDone {
		t.Fatalf("unexpected entry: %+v", out["criterion-a"])
	}
}

func TestParseVerifyReport_goldenV1(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("testdata", "verify_report_v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cycleID := "cycle-golden"
	if err := EnsureReportCycleDir(dir, cycleID); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(VerifyReportPath(dir, cycleID), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := ParseVerifyReport(dir, cycleID, map[string]struct{}{"criterion-a": {}})
	if err != nil {
		t.Fatal(err)
	}
	if !out["criterion-a"].Verified {
		t.Fatalf("unexpected entry: %+v", out["criterion-a"])
	}
}

func TestParseCriteriaReport_rejectsFutureSchemaVersion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cycleID := "cycle-1"
	if err := EnsureReportCycleDir(dir, cycleID); err != nil {
		t.Fatal(err)
	}
	body := `{"schema_version":99,"criteria":[{"id":"a","claimed_done":true,"evidence":"x"}]}`
	if err := os.WriteFile(CriteriaReportPath(dir, cycleID), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseCriteriaReport(dir, cycleID, map[string]struct{}{"a": {}})
	if !errors.Is(err, ErrCriteriaReportInvalid) {
		t.Fatalf("got %v, want invalid", err)
	}
}

func TestParseVerifyReport_rejectsFutureSchemaVersion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cycleID := "cycle-1"
	if err := EnsureReportCycleDir(dir, cycleID); err != nil {
		t.Fatal(err)
	}
	body := `{"schema_version":99,"criteria":[{"id":"a","verified":true,"reasoning":"This reasoning is long enough to pass validation checks."}]}`
	if err := os.WriteFile(VerifyReportPath(dir, cycleID), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseVerifyReport(dir, cycleID, map[string]struct{}{"a": {}})
	if !errors.Is(err, ErrVerifyReportInvalid) {
		t.Fatalf("got %v, want invalid", err)
	}
}

func TestParseCriteriaReport_duplicateID(t *testing.T) {
	dir := t.TempDir()
	cycleID := "cycle-1"
	if err := EnsureReportCycleDir(dir, cycleID); err != nil {
		t.Fatal(err)
	}
	rep := criteriaReport{Criteria: []CriteriaEntry{
		{ID: "a", ClaimedDone: true, Evidence: "x"},
		{ID: "a", ClaimedDone: true, Evidence: "y"},
	}}
	b, _ := json.Marshal(rep)
	if err := os.WriteFile(CriteriaReportPath(dir, cycleID), b, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseCriteriaReport(dir, cycleID, map[string]struct{}{"a": {}})
	if !errors.Is(err, ErrCriteriaReportInvalid) {
		t.Fatalf("got %v, want invalid", err)
	}
}
