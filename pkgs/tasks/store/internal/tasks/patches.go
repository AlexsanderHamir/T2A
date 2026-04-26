package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

func wouldCreateParentCycle(tx *gorm.DB, taskID, newParent string) (bool, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.wouldCreateParentCycle")
	cur := strings.TrimSpace(newParent)
	seen := make(map[string]bool)
	for cur != "" {
		if cur == taskID {
			return true, nil
		}
		if seen[cur] {
			return true, fmt.Errorf("%w: parent cycle", domain.ErrInvalidInput)
		}
		seen[cur] = true
		var t domain.Task
		if err := tx.Where("id = ?", cur).First(&t).Error; err != nil {
			// errors.Is (not ==) so a wrapped sentinel still
			// becomes domain.ErrNotFound. The rest of the package
			// (crud.go, cycles.go, drafts.go, etc.) already uses
			// errors.Is — keeping these two old `==` checks meant
			// patches.go was a single spot where an upstream
			// wrapper would silently turn 404 into 500.
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, domain.ErrNotFound
			}
			return false, fmt.Errorf("load parent chain: %w", err)
		}
		if t.ParentID == nil || *t.ParentID == "" {
			break
		}
		cur = *t.ParentID
	}
	return false, nil
}

func applyTaskPatches(tx *gorm.DB, taskID string, cur *domain.Task, in UpdateInput, by domain.Actor, seq int64) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyTaskPatches")
	seqPtr := seq
	if err := applyTitlePatch(tx, taskID, cur, in.Title, by, &seqPtr); err != nil {
		return err
	}
	if err := applyInitialPromptPatch(tx, taskID, cur, in.InitialPrompt, by, &seqPtr); err != nil {
		return err
	}
	if err := applyParentPatch(tx, taskID, cur, in.Parent, by, &seqPtr); err != nil {
		return err
	}
	if err := applyChecklistInheritPatch(tx, taskID, cur, in.ChecklistInherit, by, &seqPtr); err != nil {
		return err
	}
	if err := applyPriorityPatch(tx, taskID, cur, in.Priority, by, &seqPtr); err != nil {
		return err
	}
	if err := applyTaskTypePatch(cur, in.TaskType); err != nil {
		return err
	}
	if err := applyProjectPatch(tx, cur, in.Project); err != nil {
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
	if cur.ChecklistInherit && (cur.ParentID == nil || *cur.ParentID == "") {
		return fmt.Errorf("%w: checklist_inherit requires parent_id", domain.ErrInvalidInput)
	}
	return nil
}

func applyProjectPatch(tx *gorm.DB, cur *domain.Task, project *ProjectFieldPatch) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyProjectPatch")
	if project == nil {
		return nil
	}
	if project.Clear {
		cur.ProjectID = nil
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
	return nil
}

// applyPickupNotBeforePatch mutates cur.PickupNotBefore in place. The
// scheduling change is intentionally NOT recorded as a task event:
// pickup_not_before is operator-facing scheduling metadata, not part
// of the task's audit narrative. The wire-level slog line on the
// HTTP handler (handler_task_crud.go: patch) is the audit trail
// (commit body documents this rationale; see docs/SCHEDULING.md
// Implementation decisions). The handler is responsible for
// rejecting empty/invalid values on the way in; this layer trusts
// that the time has already been validated and is UTC.
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

func applyParentPatch(tx *gorm.DB, taskID string, cur *domain.Task, parent *ParentFieldPatch, by domain.Actor, seq *int64) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyParentPatch")
	if parent == nil {
		return nil
	}
	var prevStr string
	if cur.ParentID != nil {
		prevStr = *cur.ParentID
	}
	var nextStr string
	var nextPtr *string
	if parent.Clear {
		nextPtr = nil
	} else {
		pid := strings.TrimSpace(parent.ID)
		if pid == "" {
			return fmt.Errorf("%w: parent_id", domain.ErrInvalidInput)
		}
		if pid == taskID {
			return fmt.Errorf("%w: task cannot be its own parent", domain.ErrInvalidInput)
		}
		var n int64
		if err := tx.Model(&domain.Task{}).Where("id = ?", pid).Count(&n).Error; err != nil {
			return fmt.Errorf("parent lookup: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("%w: parent not found", domain.ErrInvalidInput)
		}
		cycle, err := wouldCreateParentCycle(tx, taskID, pid)
		if err != nil {
			return err
		}
		if cycle {
			return fmt.Errorf("%w: parent would create a cycle", domain.ErrInvalidInput)
		}
		nextPtr = &pid
		nextStr = pid
	}
	if prevStr != nextStr {
		// Audit invariant (docs/API-HTTP.md line 205): subtask_added /
		// subtask_removed are emitted on the **parent** task, with payload
		// `{child_task_id, title}` — mirroring the Create / Delete flows in
		// crud.go so consumers that subscribe to a parent's events to track
		// its children list see PATCH-reparents the same way they see new
		// subtasks and cascaded deletes. We deliberately do NOT append a
		// parent-related event to the child's own audit log: that would
		// diverge from Create (no parent-event on the new child) and Delete
		// (the gone task has no audit at all).
		title := strings.TrimSpace(cur.Title)
		if prevStr != "" {
			if err := appendParentChildEvent(tx, prevStr, taskID, title, domain.EventSubtaskRemoved, by); err != nil {
				return err
			}
		}
		if nextStr != "" {
			if err := appendParentChildEvent(tx, nextStr, taskID, title, domain.EventSubtaskAdded, by); err != nil {
				return err
			}
		}
	}
	cur.ParentID = nextPtr
	return nil
}

// appendParentChildEvent writes a single subtask_added / subtask_removed audit
// row on `parentID` with the documented `{child_task_id, title}` payload. It
// allocates its own per-parent event seq via kernel.NextEventSeq because the
// caller's `seq` cursor belongs to the **child** task being patched, not to
// the parent receiving the audit row.
func appendParentChildEvent(tx *gorm.DB, parentID, childID, childTitle string, t domain.EventType, by domain.Actor) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.appendParentChildEvent")
	pseq, err := kernel.NextEventSeq(tx, parentID)
	if err != nil {
		return err
	}
	b, err := json.Marshal(map[string]string{
		"child_task_id": childID,
		"title":         childTitle,
	})
	if err != nil {
		return err
	}
	return kernel.AppendEvent(tx, parentID, pseq, t, by, b)
}

func applyChecklistInheritPatch(tx *gorm.DB, taskID string, cur *domain.Task, inherit *bool, by domain.Actor, seq *int64) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyChecklistInheritPatch")
	if inherit == nil {
		return nil
	}
	was := cur.ChecklistInherit
	want := *inherit
	if want && !was {
		if err := checklist.DeleteOwnedItemsInTx(tx, taskID); err != nil {
			return err
		}
	}
	if want != was {
		b, err := json.Marshal(map[string]bool{"from": was, "to": want})
		if err != nil {
			return err
		}
		if err := kernel.AppendEvent(tx, taskID, *seq, domain.EventChecklistInheritChanged, by, b); err != nil {
			return err
		}
		*seq++
	}
	cur.ChecklistInherit = want
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

func applyTaskTypePatch(cur *domain.Task, tt *domain.TaskType) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyTaskTypePatch")
	if tt == nil {
		return nil
	}
	if !kernel.ValidTaskType(*tt) {
		return fmt.Errorf("%w: task_type", domain.ErrInvalidInput)
	}
	cur.TaskType = *tt
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
