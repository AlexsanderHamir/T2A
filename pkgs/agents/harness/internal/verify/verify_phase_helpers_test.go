package verify_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/runnerfake"
)
// hookRunner wraps a runnerfake so integration tests can script report files.
type hookRunner struct {
	*runnerfake.Runner
	preRun func(req runner.Request)
}

func (h *hookRunner) Run(ctx context.Context, req runner.Request) (runner.Result, error) {
	if h.preRun != nil {
		h.preRun(req)
	}
	if req.OnProgress != nil {
		req.OnProgress(runner.ProgressEvent{Kind: "stream", Subtype: "tool_use", Message: "verify probe"})
	}
	return h.Runner.Run(ctx, req)
}

// writeCriteriaReport scripts the agent CLI side-effect: drop a
// criteria-report.json under the worker-managed scratch dir so the
// next parseCriteriaReport call succeeds. reportDir is the value the
// worker was given via Options.ReportDir; helpers do NOT prepend any
// `.legacy-scratch/` segment after PR1 — files live outside the operator's
// RepoRoot, so the path is just <reportDir>/<cycleID>/...
func writeCriteriaReport(t *testing.T, reportDir, cycleID string, ids []string) {
	writeCriteriaReportWithCommits(t, reportDir, cycleID, ids, nil)
}

func writeCriteriaReportWithCommits(t *testing.T, reportDir, cycleID string, ids []string, commits []struct {
	SHA    string `json:"sha"`
	Branch string `json:"branch"`
}) {
	t.Helper()
	cdir := filepath.Join(reportDir, cycleID)
	if err := os.MkdirAll(cdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	type entry struct {
		ID          string `json:"id"`
		ClaimedDone bool   `json:"claimed_done"`
		Evidence    string `json:"evidence"`
	}
	rep := struct {
		Criteria []entry `json:"criteria"`
		Commits  []struct {
			SHA    string `json:"sha"`
			Branch string `json:"branch"`
		} `json:"commits,omitempty"`
	}{Commits: commits}
	for _, id := range ids {
		rep.Criteria = append(rep.Criteria, entry{ID: id, ClaimedDone: true, Evidence: "execute did the thing"})
	}
	b, _ := json.Marshal(rep)
	if err := os.WriteFile(filepath.Join(cdir, "criteria-report.json"), b, 0o644); err != nil {
		t.Fatalf("write criteria: %v", err)
	}
}

func gitCommitWorkfile(t *testing.T, dir string) (sha, branch string) {
	t.Helper()
	name := fmt.Sprintf("work-%d.txt", time.Now().UnixNano())
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("work"), 0o644); err != nil {
		t.Fatalf("write work file: %v", err)
	}
	for _, args := range [][]string{
		{"add", name},
		{"-c", "user.email=t@e.local", "-c", "user.name=t", "commit", "-m", "test work"},
	} {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	headOut, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v\n%s", err, headOut)
	}
	branchOut, err := exec.Command("git", "-C", dir, "branch", "--show-current").CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --show-current: %v\n%s", err, branchOut)
	}
	return strings.TrimSpace(string(headOut)), strings.TrimSpace(string(branchOut))
}

func writeCriteriaReportWithGitWork(t *testing.T, reportDir, cycleID, workDir string, ids []string) {
	t.Helper()
	sha, branch := gitCommitWorkfile(t, workDir)
	writeCriteriaReportWithCommits(t, reportDir, cycleID, ids, []struct {
		SHA    string `json:"sha"`
		Branch string `json:"branch"`
	}{{SHA: sha, Branch: branch}})
}

func writeVerifyReport(t *testing.T, reportDir, cycleID string, ids []string) {
	t.Helper()
	cdir := filepath.Join(reportDir, cycleID)
	if err := os.MkdirAll(cdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	type entry struct {
		ID        string `json:"id"`
		Verified  bool   `json:"verified"`
		Reasoning string `json:"reasoning"`
	}
	rep := struct {
		Criteria []entry `json:"criteria"`
	}{}
	for _, id := range ids {
		rep.Criteria = append(rep.Criteria, entry{
			ID:        id,
			Verified:  true,
			Reasoning: "verifier confirmed via diff inspection and file content review of the change set under test",
		})
	}
	b, _ := json.Marshal(rep)
	if err := os.WriteFile(filepath.Join(cdir, "verify-report.json"), b, 0o644); err != nil {
		t.Fatalf("write verify: %v", err)
	}
}

func gitInitTestRepo(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed; skipping git-backed integrity test")
	}
	for _, args := range [][]string{
		{"init"},
		{"-c", "user.email=t@e.local", "-c", "user.name=t", "commit", "--allow-empty", "-m", "init"},
	} {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}
// writeCriteriaReportFor writes a criteria-report.json containing only
// the supplied IDs (each marked claimed_done). Used by the carry-across
// tests to script per-attempt agent behaviour (attempt 1 reports both,
// attempt 2 reports only the previously-failing ID).
func writeCriteriaReportFor(t *testing.T, dir, cycleID string, ids []string) {
	t.Helper()
	writeCriteriaReport(t, dir, cycleID, ids)
}

func writePartialVerifyReport(t *testing.T, reportDir, cycleID string, verdicts map[string]bool) {
	t.Helper()
	cdir := filepath.Join(reportDir, cycleID)
	if err := os.MkdirAll(cdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	type entry struct {
		ID        string `json:"id"`
		Verified  bool   `json:"verified"`
		Reasoning string `json:"reasoning"`
	}
	rep := struct {
		Criteria []entry `json:"criteria"`
	}{}
	for id, verified := range verdicts {
		reasoning := "verifier confirmed via diff inspection and detailed file content review of the change set under test"
		if !verified {
			reasoning = "verifier rejected: the implementation does not satisfy this criterion based on diff inspection"
		}
		rep.Criteria = append(rep.Criteria, entry{ID: id, Verified: verified, Reasoning: reasoning})
	}
	b, _ := json.Marshal(rep)
	if err := os.WriteFile(filepath.Join(cdir, "verify-report.json"), b, 0o644); err != nil {
		t.Fatalf("write verify: %v", err)
	}
}
