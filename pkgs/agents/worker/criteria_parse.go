package worker

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

type criteriaReport struct {
	Criteria []criteriaReportEntry `json:"criteria"`
}

type criteriaReportEntry struct {
	ID          string `json:"id"`
	ClaimedDone bool   `json:"claimed_done"`
	Evidence    string `json:"evidence"`
}

type verifyReport struct {
	Criteria []verifyReportEntry `json:"criteria"`
}

type verifyReportEntry struct {
	ID        string `json:"id"`
	Verified  bool   `json:"verified"`
	Reasoning string `json:"reasoning"`
}

// reportCycleDir is the worker-managed scratch directory for one
// cycle's report files. Lives under Options.ReportDir (defaulted by
// NewWorker to <os.TempDir()>/t2a-worker) so the operator's RepoRoot
// is never touched. Cleaned up at terminateCycle time via
// cleanupReportDir; cycle subdirectories from a previous worker run
// are scrubbed at startExecutePhase via scrubCycleArtifacts so a
// stale file from an aborted-without-cleanup cycle never poisons
// parseCriteriaReport.
func reportCycleDir(reportDir, cycleID string) string {
	return filepath.Join(reportDir, cycleID)
}

func criteriaReportPath(reportDir, cycleID string) string {
	return filepath.Join(reportCycleDir(reportDir, cycleID), "criteria-report.json")
}

func verifyReportPath(reportDir, cycleID string) string {
	return filepath.Join(reportCycleDir(reportDir, cycleID), "verify-report.json")
}

// ensureReportCycleDir creates <reportDir>/<cycleID>/ with a permissive
// directory mode so the agent CLI can write its report into it.
// Idempotent — repeated calls within a cycle are no-ops. The directory
// lives outside any git repo, so unlike the prior .t2a/ helper there
// is no .gitignore to write here; a stray entry would be a bug.
func ensureReportCycleDir(reportDir, cycleID string) error {
	return os.MkdirAll(reportCycleDir(reportDir, cycleID), 0o755)
}

// scrubCycleArtifacts removes the per-cycle report subdirectory before
// the next execute attempt writes into it. Used at the top of every
// execute phase so a stale criteria-report.json from a previous
// attempt cannot satisfy parseCriteriaReport against this attempt's
// expected-IDs set.
func scrubCycleArtifacts(reportDir, cycleID string) error {
	return os.RemoveAll(reportCycleDir(reportDir, cycleID))
}

// cleanupReportDir removes <reportDir>/<cycleID>/ at cycle terminate
// time. Closes the unbounded-disk-growth gap that existed when files
// were written under .t2a/ — there was no per-cycle GC. Called from
// terminateCycle and the cleanup paths (handleShutdownAfterRun,
// recoverFromPanic, bestEffortTerminate) so every exit point clears
// its scratch.
func cleanupReportDir(reportDir, cycleID string) error {
	return os.RemoveAll(reportCycleDir(reportDir, cycleID))
}

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

func parseCriteriaReport(reportDir, cycleID string, expectedIDs map[string]struct{}) (map[string]criteriaReportEntry, error) {
	path := criteriaReportPath(reportDir, cycleID)
	var rep criteriaReport
	if err := readJSONFile(path, &rep); err != nil {
		return nil, err
	}
	out := make(map[string]criteriaReportEntry, len(rep.Criteria))
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

func parseVerifyReport(reportDir, cycleID string, expectedIDs map[string]struct{}) (map[string]verifyReportEntry, error) {
	path := verifyReportPath(reportDir, cycleID)
	var rep verifyReport
	if err := readJSONFile(path, &rep); err != nil {
		if errors.Is(err, ErrCriteriaReportMissing) {
			return nil, ErrVerifyReportMissing
		}
		return nil, err
	}
	out := make(map[string]verifyReportEntry, len(rep.Criteria))
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
