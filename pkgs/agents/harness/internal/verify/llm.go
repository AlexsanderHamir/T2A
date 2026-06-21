package verify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/internal/git"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/internal/prompt"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

const (
	runStateProgressKind    = "run_state"
	runStateIdleSuspicious  = "idle_suspicious"
	runStateIdleKillPending = "idle_kill_pending"
)

func (s *Service) runLLMVerifyAgent(
	ctx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	phaseSeq int64,
	runCorrelationID string,
	snap Snapshot,
	previouslyPassed map[string]Verdict,
	selfReport map[string]reports.CriteriaEntry,
	feedback string,
	cmdEvidence []CommandEvidence,
	verifyAttempt int,
) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.runLLMVerifyAgent",
		"task_id", task.ID, "cycle_id", cycle.ID, "locked_passes", len(previouslyPassed))
	promptText := buildVerifyPrompt(ctx, s, task.ID, snap, cycle.ID, previouslyPassed, selfReport, feedback, cmdEvidence)
	resumeSessionID := ""
	if s.hooks.PlanVerifyRun != nil {
		plan, err := s.hooks.PlanVerifyRun(ctx, PlanVerifyRunInput{
			Task:             task,
			Cycle:            cycle,
			Snap:             snap,
			VerifyAttempt:    verifyAttempt,
			Feedback:         feedback,
			CmdEvidence:      cmdEvidence,
			SelfReport:       selfReport,
			PreviouslyPassed: previouslyPassed,
		})
		if err != nil {
			return err
		}
		promptText = plan.Prompt
		resumeSessionID = plan.ResumeSessionID
	}
	_, err := s.runVerifyCursor(ctx, task, cycle, phaseSeq, runCorrelationID, snap, promptText, resumeSessionID)
	if errors.Is(err, runner.ErrResumeSession) {
		full := buildVerifyPrompt(ctx, s, task.ID, snap, cycle.ID, previouslyPassed, selfReport, feedback, cmdEvidence)
		_, err = s.runVerifyCursor(ctx, task, cycle, phaseSeq, runCorrelationID, snap, full, "")
	}
	return err
}

// BuildVerifyPrompt exports the full verify prompt composer for harness fallback paths.
func (s *Service) BuildVerifyPrompt(
	ctx context.Context,
	taskID string,
	snap Snapshot,
	cycleID string,
	previouslyPassed map[string]Verdict,
	selfReport map[string]reports.CriteriaEntry,
	feedback string,
	cmdEvidence []CommandEvidence,
) string {
	return buildVerifyPrompt(ctx, s, taskID, snap, cycleID, previouslyPassed, selfReport, feedback, cmdEvidence)
}

func buildVerifyPrompt(
	ctx context.Context,
	s *Service,
	taskID string,
	snap Snapshot,
	cycleID string,
	previouslyPassed map[string]Verdict,
	selfReport map[string]reports.CriteriaEntry,
	feedback string,
	cmdEvidence []CommandEvidence,
) string {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.buildVerifyPrompt",
		"task_id", taskID, "cycle_id", cycleID, "locked_passes", len(previouslyPassed))
	commits := s.loadTaskCommits(ctx, taskID)
	var b strings.Builder
	b.WriteString("You are the verification agent. Do not modify source files.\n")
	b.WriteString(fmt.Sprintf("Write `%s` only.\n\n", reports.VerifyReportPath(s.reportDir, cycleID)))
	b.WriteString("Schema: {\"criteria\":[{\"id\":\"...\",\"verified\":true|false,\"reasoning\":\"...\"}]}\n\n")
	if len(previouslyPassed) > 0 {
		b.WriteString("## Locked passes (do not re-evaluate)\n\n")
		b.WriteString("These criteria were verified in earlier attempts. Do NOT include them in your report.\n\n")
		for id := range previouslyPassed {
			b.WriteString(fmt.Sprintf("- [%s]\n", id))
		}
		b.WriteString("\n")
	}
	for _, it := range snap.Criteria {
		if _, locked := previouslyPassed[it.ID]; locked {
			continue
		}
		e, ok := selfReport[it.ID]
		if !ok || !e.ClaimedDone {
			continue
		}
		b.WriteString(fmt.Sprintf("- [%s] %s\n  execute claimed_done: true (assertion only)\n  execute evidence: %s\n", it.ID, it.Text, e.Evidence))
	}
	b.WriteString(FormatCommandEvidenceSection(cmdEvidence))
	if gitBlock := git.FormatGitContextForPrompt(commits); gitBlock != "" {
		b.WriteString(gitBlock)
	}
	b.WriteString("\nDiff:\n")
	b.WriteString(DiffSection(s.workingDir))
	promptText := b.String()
	if feedback != "" {
		promptText = prompt.AppendVerifyFeedback(promptText, feedback)
	}
	return promptText
}

func (s *Service) runVerifyCursor(
	ctx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	phaseSeq int64,
	runCorrelationID string,
	snap Snapshot,
	promptText string,
	resumeSessionID string,
) (runner.Result, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.runVerifyCursor",
		"task_id", task.ID, "cycle_id", cycle.ID, "phase_seq", phaseSeq,
		"run_correlation_id", runCorrelationID)
	runCtx, cancelCause := context.WithCancelCause(ctx)
	cancel := func() { cancelCause(context.Canceled) }
	if s.hooks.SetRunCancel != nil {
		s.hooks.SetRunCancel(cancel)
		defer s.hooks.SetRunCancel(nil)
	}
	onProgress := func(ev runner.ProgressEvent) {
		if s.hooks.PersistProgress != nil {
			s.hooks.PersistProgress(ctx, task.ID, cycle.ID, phaseSeq, ev)
		}
	}
	streamIdleStuck := s.hooks.StreamIdleStuck
	var onStreamIdle func(runner.StreamIdleKind)
	if streamIdleStuck > 0 {
		onStreamIdle = func(kind runner.StreamIdleKind) {
			ev := streamIdleProgressEvent(kind, streamIdleStuck)
			onProgress(ev)
		}
	}
	return snap.VerifyRunner.Run(runCtx, runner.Request{
		TaskID:           task.ID,
		AttemptSeq:       cycle.AttemptSeq,
		Phase:            domain.PhaseVerify,
		Prompt:           promptText,
		WorkingDir:       s.workingDir,
		CursorModel:      snap.VerifyModel,
		RunCorrelationID: runCorrelationID,
		ResumeSessionID:  resumeSessionID,
		StreamIdleStuck:  streamIdleStuck,
		OnStreamIdle:     onStreamIdle,
		OnProgress:       onProgress,
	})
}

func streamIdleProgressEvent(kind runner.StreamIdleKind, stuck time.Duration) runner.ProgressEvent {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.streamIdleProgressEvent",
		"kind", int(kind), "stuck_ns", int64(stuck))
	switch kind {
	case runner.StreamIdleKillPending:
		lead := 5 * time.Second
		if stuck > lead {
			return runner.ProgressEvent{
				Kind:    runStateProgressKind,
				Subtype: runStateIdleKillPending,
				Message: fmt.Sprintf("Terminating agent in %s if no output", lead.Round(time.Second)),
			}
		}
		return runner.ProgressEvent{
			Kind:    runStateProgressKind,
			Subtype: runStateIdleKillPending,
			Message: "Terminating agent soon if no output",
		}
	default:
		half := stuck / 2
		if half <= 0 {
			half = 30 * time.Second
		}
		return runner.ProgressEvent{
			Kind:    runStateProgressKind,
			Subtype: runStateIdleSuspicious,
			Message: fmt.Sprintf("No agent output for %s — run may be stuck", half.Round(time.Second)),
		}
	}
}

func (s *Service) assembleVerdictsFromVerifyReport(
	cycleID string,
	expected map[string]struct{},
	verdicts []Verdict,
	selfReport map[string]reports.CriteriaEntry,
	previouslyPassed map[string]Verdict,
) ([]Verdict, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.assembleVerdictsFromVerifyReport",
		"cycle_id", cycleID, "expected", len(expected))
	vrep, err := reports.ParseVerifyReport(s.reportDir, cycleID, expected)
	if err != nil {
		return nil, err
	}
	next := make([]Verdict, 0, len(verdicts))
	for _, v := range verdicts {
		if _, locked := previouslyPassed[v.ID]; locked {
			next = append(next, v)
			continue
		}
		if v.Verifier == domain.VerifierAgentSelf {
			next = append(next, v)
			continue
		}
		entry := selfReport[v.ID]
		vr := vrep[v.ID]
		nv := Verdict{ID: v.ID, Evidence: entry.Evidence}
		if vr.Verified {
			nv.Passed = true
			nv.Verifier = domain.VerifierVerifyAgent
			nv.Reasoning = vr.Reasoning
		} else {
			nv.Passed = false
			nv.Verifier = domain.VerifierVerifyAgent
			nv.Reasoning = vr.Reasoning
		}
		next = append(next, nv)
		s.recordVerdict(domain.VerifierVerifyAgent, nv.Passed)
	}
	return next, nil
}

func verifyLLMRunError(runErr error, parseErr error) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.verifyLLMRunError",
		"has_run_err", runErr != nil, "has_parse_err", parseErr != nil)
	if runErr != nil && !errors.Is(runErr, runner.ErrStale) {
		return runErr
	}
	if parseErr != nil {
		if errors.Is(runErr, runner.ErrStale) {
			return fmt.Errorf("verify agent stream idle: %w", parseErr)
		}
		return parseErr
	}
	return nil
}
