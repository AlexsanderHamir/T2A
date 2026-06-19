package reports

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrCriteriaReportMissing = errors.New("criteria report missing")
	ErrCriteriaReportInvalid = errors.New("criteria report invalid")
	ErrVerifyReportMissing   = errors.New("verify report missing")
	ErrVerifyReportInvalid   = errors.New("verify report invalid")
)

const maxReportFileBytes = 256 * 1024
const maxFieldBytes = 16 * 1024
const minVerifyReasoning = 40

// CurrentSchemaVersion is the report JSON schema written by the worker and
// expected from agent side-channel files. Major version bumps require parser
// updates; minor fields may be added without a bump.
const CurrentSchemaVersion = 1

type criteriaReport struct {
	SchemaVersion int             `json:"schema_version"`
	Criteria      []CriteriaEntry `json:"criteria"`
	// Commits is worker-ingested at execute complete; ignored here so
	// ParseCriteriaReport stays compatible with ADR-0014 reports.
	Commits []struct {
		SHA    string `json:"sha"`
		Branch string `json:"branch"`
	} `json:"commits,omitempty"`
}

// CriteriaEntry is one row in criteria-report.json.
type CriteriaEntry struct {
	ID          string `json:"id"`
	ClaimedDone bool   `json:"claimed_done"`
	Evidence    string `json:"evidence"`
}

type verifyReport struct {
	SchemaVersion int           `json:"schema_version"`
	Criteria      []VerifyEntry `json:"criteria"`
}

// VerifyEntry is one row in verify-report.json.
type VerifyEntry struct {
	ID        string `json:"id"`
	Verified  bool   `json:"verified"`
	Reasoning string `json:"reasoning"`
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ReportCycleDir is the worker-managed scratch directory for one
// cycle's report files. Lives under Options.ReportDir (defaulted by
// NewWorker to <os.TempDir()>/t2a-worker) so the operator's RepoRoot
// is never touched. Cleaned up at terminateCycle time via
// CleanupReportDir; cycle subdirectories from a previous worker run
// are scrubbed at startExecutePhase via ScrubCycleArtifacts so a
// stale file from an aborted-without-cleanup cycle never poisons
// ParseCriteriaReport.
func ReportCycleDir(reportDir, cycleID string) string {
	return filepath.Join(reportDir, cycleID)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func CriteriaReportPath(reportDir, cycleID string) string {
	return filepath.Join(ReportCycleDir(reportDir, cycleID), "criteria-report.json")
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func VerifyReportPath(reportDir, cycleID string) string {
	return filepath.Join(ReportCycleDir(reportDir, cycleID), "verify-report.json")
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// EnsureReportCycleDir creates <reportDir>/<cycleID>/ with a permissive
// directory mode so the agent CLI can write its report into it.
// Idempotent — repeated calls within a cycle are no-ops. The directory
// lives outside any git repo, so unlike the prior .t2a/ helper there
// is no .gitignore to write here; a stray entry would be a bug.
func EnsureReportCycleDir(reportDir, cycleID string) error {
	return os.MkdirAll(ReportCycleDir(reportDir, cycleID), 0o755)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ScrubCycleArtifacts removes the per-cycle report subdirectory before
// the next execute attempt writes into it. Used at the top of every
// execute phase so a stale criteria-report.json from a previous
// attempt cannot satisfy ParseCriteriaReport against this attempt's
// expected-IDs set.
func ScrubCycleArtifacts(reportDir, cycleID string) error {
	return os.RemoveAll(ReportCycleDir(reportDir, cycleID))
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// CleanupReportDir removes <reportDir>/<cycleID>/ at cycle terminate
// time. Closes the unbounded-disk-growth gap that existed when files
// were written under .t2a/ — there was no per-cycle GC. Called from
// terminateCycle and the cleanup paths (handleShutdownAfterRun,
// recoverFromPanic, bestEffortTerminate) so every exit point clears
// its scratch.
func CleanupReportDir(reportDir, cycleID string) error {
	return os.RemoveAll(ReportCycleDir(reportDir, cycleID))
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func readJSONFile(path string, dest any) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrCriteriaReportMissing
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: symlink not permitted", ErrCriteriaReportInvalid)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, maxReportFileBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("%w: %v", ErrCriteriaReportInvalid, err)
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func validateSchemaVersion(v int) error {
	if v == 0 {
		return nil
	}
	if v > CurrentSchemaVersion {
		return fmt.Errorf("%w: unsupported schema_version %d", ErrCriteriaReportInvalid, v)
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func validateCriteriaReportSchema(rep *criteriaReport) error {
	return validateSchemaVersion(rep.SchemaVersion)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func validateVerifyReportSchema(rep *verifyReport) error {
	if err := validateSchemaVersion(rep.SchemaVersion); err != nil {
		if errors.Is(err, ErrCriteriaReportInvalid) {
			return ErrVerifyReportInvalid
		}
		return err
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ParseCriteriaReportPartial(reportDir, cycleID string) (map[string]CriteriaEntry, error) {
	path := CriteriaReportPath(reportDir, cycleID)
	var rep criteriaReport
	if err := readJSONFile(path, &rep); err != nil {
		return nil, err
	}
	if err := validateCriteriaReportSchema(&rep); err != nil {
		return nil, err
	}
	out := make(map[string]CriteriaEntry, len(rep.Criteria))
	for _, e := range rep.Criteria {
		id := strings.TrimSpace(e.ID)
		if id == "" {
			continue
		}
		out[id] = e
	}
	return out, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ParseCriteriaReport(reportDir, cycleID string, expectedIDs map[string]struct{}) (map[string]CriteriaEntry, error) {
	path := CriteriaReportPath(reportDir, cycleID)
	var rep criteriaReport
	if err := readJSONFile(path, &rep); err != nil {
		return nil, err
	}
	if err := validateCriteriaReportSchema(&rep); err != nil {
		return nil, err
	}
	out := make(map[string]CriteriaEntry, len(rep.Criteria))
	for _, e := range rep.Criteria {
		id := strings.TrimSpace(e.ID)
		if id == "" {
			return nil, fmt.Errorf("%w: empty criterion id", ErrCriteriaReportInvalid)
		}
		if _, dup := out[id]; dup {
			return nil, fmt.Errorf("%w: duplicate criterion id %s", ErrCriteriaReportInvalid, id)
		}
		if len(e.Evidence) > maxFieldBytes {
			return nil, fmt.Errorf("%w: evidence too long", ErrCriteriaReportInvalid)
		}
		out[id] = e
	}
	for id := range expectedIDs {
		if _, ok := out[id]; !ok {
			return nil, fmt.Errorf("%w: missing criterion %s", ErrCriteriaReportInvalid, id)
		}
	}
	return out, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ParseVerifyReport(reportDir, cycleID string, expectedIDs map[string]struct{}) (map[string]VerifyEntry, error) {
	path := VerifyReportPath(reportDir, cycleID)
	var rep verifyReport
	if err := readJSONFile(path, &rep); err != nil {
		if errors.Is(err, ErrCriteriaReportMissing) {
			return nil, ErrVerifyReportMissing
		}
		return nil, err
	}
	if err := validateVerifyReportSchema(&rep); err != nil {
		return nil, err
	}
	out := make(map[string]VerifyEntry, len(rep.Criteria))
	for _, e := range rep.Criteria {
		id := strings.TrimSpace(e.ID)
		if id == "" {
			return nil, fmt.Errorf("%w: empty criterion id", ErrVerifyReportInvalid)
		}
		if _, dup := out[id]; dup {
			return nil, fmt.Errorf("%w: duplicate criterion id %s", ErrVerifyReportInvalid, id)
		}
		if e.Verified && len(strings.TrimSpace(e.Reasoning)) < minVerifyReasoning {
			return nil, fmt.Errorf("%w: reasoning too short for verified criterion %s", ErrVerifyReportInvalid, id)
		}
		if len(e.Reasoning) > maxFieldBytes {
			return nil, fmt.Errorf("%w: reasoning too long", ErrVerifyReportInvalid)
		}
		out[id] = e
	}
	for id := range expectedIDs {
		if _, ok := out[id]; !ok {
			return nil, fmt.Errorf("%w: missing criterion %s", ErrVerifyReportInvalid, id)
		}
	}
	return out, nil
}
