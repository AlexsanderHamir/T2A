// Package commits persists worker-indexed git commits for execution
// cycles into task_cycle_commits (ADR-0014, ADR-0016).
package commits

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const logCmd = "taskapi"

// Entry is one commit row to upsert for a cycle.
type Entry struct {
	PhaseSeq      int64
	Seq           int64
	Repo          string
	Worktree      string
	Branch        string
	SHA           string
	CommittedAt   time.Time
	Message       string
	Status        domain.CommitStatus
	GateReason    string
	SourceCycleID string
}

// UpsertCycleCommits inserts or updates commit rows for one cycle batch.
// Idempotent on (cycle_id, sha). Empty entries is a no-op.
func UpsertCycleCommits(ctx context.Context, db *gorm.DB, taskID, cycleID string, entries []Entry) error {
	defer kernel.DeferLatency(kernel.OpUpsertCycleCommits)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.commits.UpsertCycleCommits",
		"cycle_id", cycleID, "entry_count", len(entries))
	taskID = strings.TrimSpace(taskID)
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	if taskID == "" {
		return fmt.Errorf("%w: task_id", domain.ErrInvalidInput)
	}
	if len(entries) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]domain.TaskCycleCommit, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		sha := strings.TrimSpace(e.SHA)
		if sha == "" {
			return fmt.Errorf("%w: sha", domain.ErrInvalidInput)
		}
		if e.PhaseSeq <= 0 || e.Seq <= 0 {
			return fmt.Errorf("%w: phase_seq and seq must be positive", domain.ErrInvalidInput)
		}
		if _, dup := seen[sha]; dup {
			return fmt.Errorf("%w: duplicate sha %s", domain.ErrInvalidInput, sha)
		}
		seen[sha] = struct{}{}
		status := e.Status
		if status == "" {
			status = domain.CommitEligible
		}
		if !domain.ValidCommitStatus(status) {
			return fmt.Errorf("%w: invalid commit status %q", domain.ErrInvalidInput, status)
		}
		rows = append(rows, domain.TaskCycleCommit{
			ID:            uuid.NewString(),
			TaskID:        taskID,
			CycleID:       cycleID,
			PhaseSeq:      e.PhaseSeq,
			Seq:           e.Seq,
			Repo:          strings.TrimSpace(e.Repo),
			Worktree:      strings.TrimSpace(e.Worktree),
			Branch:        strings.TrimSpace(e.Branch),
			SHA:           sha,
			CommittedAt:   e.CommittedAt.UTC(),
			Message:       e.Message,
			Status:        status,
			GateReason:    strings.TrimSpace(e.GateReason),
			SourceCycleID: strings.TrimSpace(e.SourceCycleID),
			RecordedAt:    now,
		})
	}
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "cycle_id"}, {Name: "sha"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"phase_seq", "seq", "repo", "worktree", "branch",
			"committed_at", "message", "status", "gate_reason", "source_cycle_id", "recorded_at",
		}),
	}).Omit("Cycle", "Task").Create(&rows).Error
	if err != nil {
		return fmt.Errorf("upsert cycle commits: %w", err)
	}
	return nil
}

// ListCommitsForCycle returns commits for cycleID ordered by seq ASC.
func ListCommitsForCycle(ctx context.Context, db *gorm.DB, cycleID string) ([]domain.TaskCycleCommit, error) {
	defer kernel.DeferLatency(kernel.OpListCommitsForCycle)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.commits.ListCommitsForCycle",
		"cycle_id", cycleID)
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	var rows []domain.TaskCycleCommit
	if err := db.WithContext(ctx).
		Where("cycle_id = ?", cycleID).
		Order("seq ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list cycle commits: %w", err)
	}
	return rows, nil
}

// ListEligibleCommitsForCycle returns commits with status eligible for verify.
func ListEligibleCommitsForCycle(ctx context.Context, db *gorm.DB, cycleID string) ([]domain.TaskCycleCommit, error) {
	defer kernel.DeferLatency(kernel.OpListEligibleCommitsForCycle)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.commits.ListEligibleCommitsForCycle",
		"cycle_id", cycleID)
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	var rows []domain.TaskCycleCommit
	if err := db.WithContext(ctx).
		Where("cycle_id = ? AND status = ?", cycleID, domain.CommitEligible).
		Order("seq ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list eligible cycle commits: %w", err)
	}
	return rows, nil
}

// ListCommitsForTask returns distinct commits indexed for taskID across every
// execution attempt. When the same SHA appears on multiple cycles, the row
// with the highest CommitStatusRank wins.
func ListCommitsForTask(ctx context.Context, db *gorm.DB, taskID string) ([]domain.TaskCycleCommit, error) {
	defer kernel.DeferLatency(kernel.OpListCommitsForTask)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.commits.ListCommitsForTask",
		"task_id", taskID)
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: task_id", domain.ErrInvalidInput)
	}
	var rows []domain.TaskCycleCommit
	if err := db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("committed_at ASC, seq ASC, recorded_at ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list task commits: %w", err)
	}
	return dedupeCommitsBySHA(rows), nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func dedupeCommitsBySHA(rows []domain.TaskCycleCommit) []domain.TaskCycleCommit {
	if len(rows) == 0 {
		return nil
	}
	best := make(map[string]domain.TaskCycleCommit, len(rows))
	order := make([]string, 0, len(rows))
	for i := range rows {
		sha := strings.TrimSpace(rows[i].SHA)
		if sha == "" {
			continue
		}
		prev, ok := best[sha]
		if !ok || domain.CommitStatusRank(rows[i].Status) > domain.CommitStatusRank(prev.Status) {
			if !ok {
				order = append(order, sha)
			}
			best[sha] = rows[i]
		}
	}
	out := make([]domain.TaskCycleCommit, 0, len(order))
	for _, sha := range order {
		out = append(out, best[sha])
	}
	return out
}

// MarkCycleCommitsSuperseded sets status superseded for SHAs in cycleID not in keepSHAs.
func MarkCycleCommitsSuperseded(ctx context.Context, db *gorm.DB, cycleID string, keepSHAs map[string]struct{}) error {
	defer kernel.DeferLatency(kernel.OpMarkCycleCommitsSuperseded)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.commits.MarkCycleCommitsSuperseded",
		"cycle_id", cycleID, "keep_count", len(keepSHAs))
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	var rows []domain.TaskCycleCommit
	if err := db.WithContext(ctx).Where("cycle_id = ?", cycleID).Find(&rows).Error; err != nil {
		return fmt.Errorf("list commits for supersede: %w", err)
	}
	for _, row := range rows {
		if _, ok := keepSHAs[row.SHA]; ok {
			continue
		}
		if row.Status == domain.CommitSuperseded {
			continue
		}
		if err := db.WithContext(ctx).Model(&domain.TaskCycleCommit{}).
			Where("id = ?", row.ID).
			Updates(map[string]any{
				"status":      domain.CommitSuperseded,
				"recorded_at": time.Now().UTC(),
			}).Error; err != nil {
			return fmt.Errorf("supersede commit %s: %w", row.SHA, err)
		}
	}
	return nil
}
