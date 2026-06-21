package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/adapterkit"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

const maxCommandOutputBytes = 256 * 1024

const inlineCommandPreviewLines = 40

// CommandEvidence captures one verify-phase command run for LLM prompt and audit.
type CommandEvidence struct {
	CriterionID     string
	CommandSeq      int
	Command         string
	ExpectedOutcome string
	ExitCode        int
	DurationMS      int64
	StdoutPath      string
	StderrPath      string
	MetaPath        string
	Truncated       bool
	RunError        string
	StdoutPreview   string
}

type commandMetaFile struct {
	CriterionID     string `json:"criterion_id"`
	Seq             int    `json:"seq"`
	Command         string `json:"command"`
	ExpectedOutcome string `json:"expected_outcome"`
	ExitCode        int    `json:"exit_code"`
	DurationMS      int64  `json:"duration_ms"`
	StdoutBytes     int    `json:"stdout_bytes"`
	StderrBytes     int    `json:"stderr_bytes"`
	Truncated       bool   `json:"truncated"`
	Error           string `json:"error,omitempty"`
}

type shellExecFunc func(ctx context.Context, dir string, command string) (stdout, stderr []byte, exitCode int, err error)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func commandEvidenceDir(reportDir, cycleID, criterionID string) string {
	return filepath.Join(reportDir, cycleID, "checks", criterionID)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func commandArtifactBase(reportDir, cycleID, criterionID string, seq int) string {
	return filepath.Join(commandEvidenceDir(reportDir, cycleID, criterionID), fmt.Sprintf("%d", seq))
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func truncateCommandOutput(b []byte) ([]byte, bool) {
	if len(b) <= maxCommandOutputBytes {
		return b, false
	}
	return b[:maxCommandOutputBytes], true
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func previewCommandOutput(path string) string {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return ""
	}
	if len(b) <= 4096 {
		return string(b)
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) <= inlineCommandPreviewLines {
		return string(b[:4096])
	}
	return strings.Join(lines[:inlineCommandPreviewLines], "\n")
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func shellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/C", command}
	}
	return "sh", []string{"-c", command}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func defaultShellExec(ctx context.Context, dir, command string) ([]byte, []byte, int, error) {
	shell, args := shellCommand(command)
	return adapterkit.DefaultExec(ctx, dir, os.Environ(), nil, shell, args...)
}

// RunCriterionCommands executes checklist verify commands and records artifact paths.
func (s *Service) RunCriterionCommands(
	parentCtx context.Context,
	cycleID string,
	attemptSeq int64,
	snap Snapshot,
	selfReport map[string]reports.CriteriaEntry,
	execFn shellExecFunc,
) ([]CommandEvidence, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.RunCriterionCommands",
		"cycle_id", cycleID, "attempt_seq", attemptSeq)
	if execFn == nil {
		execFn = defaultShellExec
	}
	timeout := time.Duration(snap.VerifyCommandTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(domain.DefaultVerifyCommandTimeoutSeconds) * time.Second
	}
	var out []CommandEvidence
	var persist []store.CommandRunEntry
	for _, it := range snap.Criteria {
		entry, ok := selfReport[it.ID]
		if !ok || !entry.ClaimedDone || len(it.VerifyCommands) == 0 {
			continue
		}
		dir := commandEvidenceDir(s.reportDir, cycleID, it.ID)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("mkdir command evidence %s: %w", dir, err)
		}
		for _, cmd := range it.VerifyCommands {
			seq := cmd.SortOrder
			base := commandArtifactBase(s.reportDir, cycleID, it.ID, seq)
			stdoutPath := base + ".stdout"
			stderrPath := base + ".stderr"
			metaPath := base + ".meta.json"

			cmdCtx, cancel := context.WithTimeout(parentCtx, timeout)
			started := s.clock()
			stdout, stderr, exitCode, runErr := execFn(cmdCtx, s.workingDir, cmd.Command)
			cancel()
			durationMS := s.clock().Sub(started).Milliseconds()

			stdout, truncOut := truncateCommandOutput(stdout)
			stderr, truncErr := truncateCommandOutput(stderr)
			truncated := truncOut || truncErr

			if werr := os.WriteFile(stdoutPath, stdout, 0o600); werr != nil {
				return nil, fmt.Errorf("write stdout evidence: %w", werr)
			}
			if werr := os.WriteFile(stderrPath, stderr, 0o600); werr != nil {
				return nil, fmt.Errorf("write stderr evidence: %w", werr)
			}

			meta := commandMetaFile{
				CriterionID:     it.ID,
				Seq:             seq,
				Command:         cmd.Command,
				ExpectedOutcome: cmd.ExpectedOutcome,
				ExitCode:        exitCode,
				DurationMS:      durationMS,
				StdoutBytes:     len(stdout),
				StderrBytes:     len(stderr),
				Truncated:       truncated,
			}
			if runErr != nil {
				meta.Error = runErr.Error()
				meta.ExitCode = -1
				exitCode = -1
			}
			metaBytes, _ := json.Marshal(meta)
			if werr := os.WriteFile(metaPath, metaBytes, 0o600); werr != nil {
				return nil, fmt.Errorf("write meta evidence: %w", werr)
			}

			ev := CommandEvidence{
				CriterionID:     it.ID,
				CommandSeq:      seq,
				Command:         cmd.Command,
				ExpectedOutcome: cmd.ExpectedOutcome,
				ExitCode:        exitCode,
				DurationMS:      durationMS,
				StdoutPath:      stdoutPath,
				StderrPath:      stderrPath,
				MetaPath:        metaPath,
				Truncated:       truncated,
				StdoutPreview:   previewCommandOutput(stdoutPath),
			}
			if runErr != nil {
				ev.RunError = runErr.Error()
			}
			out = append(out, ev)
			persist = append(persist, store.CommandRunEntry{
				CriterionID: it.ID,
				CommandSeq:  int64(seq),
				ExitCode:    exitCode,
				MetaPath:    metaPath,
			})
		}
	}
	if len(persist) > 0 {
		if err := s.store.UpsertCommandRuns(parentCtx, cycleID, attemptSeq, persist); err != nil {
			slog.Warn("agent harness UpsertCommandRuns failed",
				"cmd", logCmd, "operation", "agent.harness.verify.RunCriterionCommands.upsert_err",
				"cycle_id", cycleID, "attempt_seq", attemptSeq, "err", err)
		}
	}
	return out, nil
}

// FormatCommandEvidenceSection renders worker command output for verify prompts.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FormatCommandEvidenceSection(evidence []CommandEvidence) string {
	if len(evidence) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## Command evidence (worker-executed)\n\n")
	for _, ev := range evidence {
		b.WriteString(fmt.Sprintf("### [%s] command %d\n", ev.CriterionID, ev.CommandSeq))
		b.WriteString(fmt.Sprintf("Command: %s\n", ev.Command))
		if ev.ExpectedOutcome != "" {
			b.WriteString(fmt.Sprintf("Expected outcome: %s\n", ev.ExpectedOutcome))
		}
		b.WriteString(fmt.Sprintf("exit_code=%d duration_ms=%d truncated=%v\n", ev.ExitCode, ev.DurationMS, ev.Truncated))
		if ev.RunError != "" {
			b.WriteString(fmt.Sprintf("run_error: %s\n", ev.RunError))
		}
		b.WriteString(fmt.Sprintf("stdout: `%s`\n", ev.StdoutPath))
		b.WriteString(fmt.Sprintf("stderr: `%s`\n", ev.StderrPath))
		b.WriteString(fmt.Sprintf("meta: `%s`\n", ev.MetaPath))
		if ev.StdoutPreview != "" {
			b.WriteString("stdout preview:\n```\n")
			b.WriteString(ev.StdoutPreview)
			if !strings.HasSuffix(ev.StdoutPreview, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("```\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}
