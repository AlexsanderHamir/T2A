package verify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func (s *Service) runLLMVerifyAgent(
	ctx context.Context,
	task *domain.Task,
	cycle *domain.TaskCycle,
	phaseSeq int64,
	snap Snapshot,
	previouslyPassed map[string]Verdict,
	selfReport map[string]reports.CriteriaEntry,
	feedback string,
	cmdEvidence []CommandEvidence,
) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.runLLMVerifyAgent",
		"task_id", task.ID, "cycle_id", cycle.ID, "locked_passes", len(previouslyPassed))
	commits := s.loadEligibleCommits(ctx, cycle.ID)
	var b strings.Builder
	b.WriteString("You are the verification agent. Do not modify source files.\n")
	b.WriteString(fmt.Sprintf("Write `%s` only.\n\n", reports.VerifyReportPath(s.reportDir, cycle.ID)))
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
	_, err := snap.VerifyRunner.Run(ctx, runner.Request{
		TaskID:      task.ID,
		AttemptSeq:  cycle.AttemptSeq,
		Phase:       domain.PhaseVerify,
		Prompt:      promptText,
		WorkingDir:  s.workingDir,
		CursorModel: snap.VerifyModel,
		OnProgress: func(ev runner.ProgressEvent) {
			if s.hooks.PersistProgress != nil {
				s.hooks.PersistProgress(ctx, task.ID, cycle.ID, phaseSeq, ev)
			}
		},
	})
	return err
}
