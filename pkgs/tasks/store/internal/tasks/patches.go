package tasks

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

func applyTaskPatches(tx *gorm.DB, taskID string, cur *domain.Task, in UpdateInput, by domain.Actor, seq int64) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyTaskPatches")
	seqPtr := seq
	if err := applyTitlePatch(tx, taskID, cur, in.Title, by, &seqPtr); err != nil {
		return err
	}
	if err := applyInitialPromptPatch(tx, taskID, cur, in.InitialPrompt, by, &seqPtr); err != nil {
		return err
	}
	if err := applyPriorityPatch(tx, taskID, cur, in.Priority, by, &seqPtr); err != nil {
		return err
	}
	if err := applyProjectPatch(tx, cur, in.Project); err != nil {
		return err
	}
	if err := applyProjectContextSelectionPatch(tx, cur, in.ProjectContextItemIDs); err != nil {
		return err
	}
	if err := applyStatusPatch(tx, taskID, cur, in.Status, by, &seqPtr); err != nil {
		return err
	}
	if err := applyPickupNotBeforePatch(cur, in.PickupNotBefore); err != nil {
		return err
	}
	if err := applyCursorModelPatch(cur, in.CursorModel); err != nil {
		return err
	}
	if err := applyTagsPatch(cur, in.Tags); err != nil {
		return err
	}
	if err := applyMilestonePatch(cur, in.Milestone); err != nil {
		return err
	}
	if err := applyGatePatch(cur, in.Gate); err != nil {
		return err
	}
	if err := applyDependsOnPatch(tx, taskID, cur, in.DependsOn); err != nil {
		return err
	}
	return nil
}

func applyProjectContextSelectionPatch(tx *gorm.DB, cur *domain.Task, ids *[]string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyProjectContextSelectionPatch")
	if ids == nil {
		return nil
	}
	contextIDs, err := normalizeProjectContextItemIDs(*ids)
	if err != nil {
		return err
	}
	if len(contextIDs) > 0 {
		if cur.ProjectID == nil || strings.TrimSpace(*cur.ProjectID) == "" {
			return fmt.Errorf("%w: project_id required for project context selection", domain.ErrInvalidInput)
		}
		if err := validateProjectContextSelection(tx, *cur.ProjectID, contextIDs); err != nil {
			return err
		}
	}
	cur.ProjectContextItemIDs = contextIDs
	return nil
}

func applyProjectPatch(tx *gorm.DB, cur *domain.Task, project *ProjectFieldPatch) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyProjectPatch")
	if project == nil {
		return nil
	}
	if project.Clear {
		cur.ProjectID = nil
		cur.ProjectContextItemIDs = nil
		return nil
	}
	pid := strings.TrimSpace(project.ID)
	if pid == "" {
		return fmt.Errorf("%w: project_id", domain.ErrInvalidInput)
	}
	var n int64
	if err := tx.Model(&domain.Project{}).Where("id = ? AND status = ?", pid, domain.ProjectStatusActive).Count(&n).Error; err != nil {
		return fmt.Errorf("project lookup: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: project not found", domain.ErrInvalidInput)
	}
	cur.ProjectID = &pid
	cur.ProjectContextItemIDs = nil
	return nil
}

const maxTaskCursorModelLen = 256

func applyCursorModelPatch(cur *domain.Task, p *string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyCursorModelPatch")
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if len(v) > maxTaskCursorModelLen {
		return fmt.Errorf("%w: cursor_model too long (max 256)", domain.ErrInvalidInput)
	}
	cur.CursorModel = v
	return nil
}

func applyPickupNotBeforePatch(cur *domain.Task, p *PickupNotBeforePatch) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyPickupNotBeforePatch")
	if p == nil {
		return nil
	}
	if p.Clear {
		cur.PickupNotBefore = nil
		return nil
	}
	t := p.At.UTC()
	cur.PickupNotBefore = &t
	return nil
}

func applyTitlePatch(tx *gorm.DB, taskID string, cur *domain.Task, title *string, by domain.Actor, seq *int64) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyTitlePatch")
	if title == nil {
		return nil
	}
	v := strings.TrimSpace(*title)
	if v == "" {
		return fmt.Errorf("%w: title", domain.ErrInvalidInput)
	}
	if v == cur.Title {
		return nil
	}
	b, err := kernel.EventPairJSON(cur.Title, v)
	if err != nil {
		return err
	}
	if err := kernel.AppendEvent(tx, taskID, *seq, domain.EventMessageAdded, by, b); err != nil {
		return err
	}
	*seq++
	cur.Title = v
	return nil
}

func applyInitialPromptPatch(tx *gorm.DB, taskID string, cur *domain.Task, prompt *string, by domain.Actor, seq *int64) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyInitialPromptPatch")
	if prompt == nil {
		return nil
	}
	if *prompt == cur.InitialPrompt {
		return nil
	}
	b, err := kernel.EventPairJSON(cur.InitialPrompt, *prompt)
	if err != nil {
		return err
	}
	if err := kernel.AppendEvent(tx, taskID, *seq, domain.EventPromptAppended, by, b); err != nil {
		return err
	}
	*seq++
	cur.InitialPrompt = *prompt
	return nil
}

func applyPriorityPatch(tx *gorm.DB, taskID string, cur *domain.Task, pr *domain.Priority, by domain.Actor, seq *int64) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyPriorityPatch")
	if pr == nil {
		return nil
	}
	if !kernel.ValidPriority(*pr) {
		return fmt.Errorf("%w: priority", domain.ErrInvalidInput)
	}
	if *pr == cur.Priority {
		return nil
	}
	b, err := kernel.EventPairJSON(string(cur.Priority), string(*pr))
	if err != nil {
		return err
	}
	if err := kernel.AppendEvent(tx, taskID, *seq, domain.EventPriorityChanged, by, b); err != nil {
		return err
	}
	*seq++
	cur.Priority = *pr
	return nil
}

func applyStatusPatch(tx *gorm.DB, taskID string, cur *domain.Task, st *domain.Status, by domain.Actor, seq *int64) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyStatusPatch")
	if st == nil {
		return nil
	}
	if !kernel.ValidStatus(*st) {
		return fmt.Errorf("%w: status", domain.ErrInvalidInput)
	}
	if *st == cur.Status {
		return nil
	}
	if *st == domain.StatusDone {
		if err := checklist.ValidateCanMarkDoneInTx(tx, taskID); err != nil {
			return err
		}
	}
	b, err := kernel.EventPairJSON(string(cur.Status), string(*st))
	if err != nil {
		return err
	}
	if err := kernel.AppendEvent(tx, taskID, *seq, domain.EventStatusChanged, by, b); err != nil {
		return err
	}
	*seq++
	cur.Status = *st
	return nil
}
