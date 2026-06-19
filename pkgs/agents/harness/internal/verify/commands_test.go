package verify

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestRunCriterionCommands_writesEvidenceAndPromptSection(t *testing.T) {
	t.Parallel()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()

	task, err := st.Create(ctx, store.CreateTaskInput{
		Title:         "verify-cmd",
		InitialPrompt: "do work",
		Status:        domain.StatusReady,
		Priority:      domain.PriorityMedium,
		ChecklistItems: []store.CreateChecklistItemInput{{
			Text: "tests pass",
			VerifyCommands: []store.VerifyCommandInput{{
				Command:         "echo hello",
				ExpectedOutcome: "prints hello",
			}},
		}},
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	items, err := st.ListChecklistForVerify(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || len(items[0].VerifyCommands) != 1 {
		t.Fatalf("verify snapshot: %+v", items)
	}

	reportDir := t.TempDir()
	workDir := t.TempDir()
	svc := NewService(Deps{
		Store:      st,
		Runner:     runnerfake.New(),
		ReportDir:  reportDir,
		WorkingDir: workDir,
	})

	cycleID := "cycle-verify-cmd"
	selfReport := map[string]reports.CriteriaEntry{
		items[0].ID: {ClaimedDone: true, Evidence: "done"},
	}
	snap := Snapshot{
		VerifyCommandTimeoutSeconds: domain.DefaultVerifyCommandTimeoutSeconds,
		Criteria:                    items,
	}

	evidence, err := svc.RunCriterionCommands(ctx, cycleID, 1, snap, selfReport, func(ctx context.Context, dir, command string) ([]byte, []byte, int, error) {
		if command != "echo hello" {
			t.Fatalf("command = %q", command)
		}
		return []byte("hello\n"), nil, 0, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 {
		t.Fatalf("evidence len = %d", len(evidence))
	}
	ev := evidence[0]
	for _, p := range []string{ev.StdoutPath, ev.StderrPath, ev.MetaPath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing artifact %s: %v", p, err)
		}
	}
	stdout, _ := os.ReadFile(ev.StdoutPath)
	if string(stdout) != "hello\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	metaBytes, _ := os.ReadFile(ev.MetaPath)
	var meta commandMetaFile
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.ExpectedOutcome != "prints hello" || meta.ExitCode != 0 {
		t.Fatalf("meta = %+v", meta)
	}

	section := FormatCommandEvidenceSection(evidence)
	if !strings.Contains(section, ev.StdoutPath) || !strings.Contains(section, "Expected outcome: prints hello") || !strings.Contains(section, "exit_code=0") {
		t.Fatalf("prompt section missing paths/details:\n%s", section)
	}

	base := filepath.Join(reportDir, cycleID, "checks", items[0].ID, "0")
	if _, err := os.Stat(base + ".stdout"); err != nil {
		t.Fatalf("expected base artifacts under %s: %v", base, err)
	}
}
