// Package automations owns persistence for the global prompt automation library.
package automations

import (
	"context"
	"errors"
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

// CreateInput is the store input for creating an automation.
type CreateInput struct {
	ID          string
	Title       string
	Description string
}

// UpdateInput is a partial patch for one automation.
type UpdateInput struct {
	Title       *string
	Description *string
}

// Create inserts a new active automation.
func Create(ctx context.Context, db *gorm.DB, input CreateInput) (domain.Automation, error) {
	defer kernel.DeferLatency(kernel.OpCreateAutomation)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.automations.Create")
	title, description, err := domain.ValidateAutomationFields(input.Title, input.Description)
	if err != nil {
		return domain.Automation{}, err
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = uuid.NewString()
	}
	if err := assertTitleAvailable(ctx, db, title, ""); err != nil {
		return domain.Automation{}, err
	}
	now := time.Now().UTC()
	row := domain.Automation{
		ID:          id,
		Title:       title,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Automation{}, mapWriteError(err)
	}
	return row, nil
}

// List returns automations ordered by title.
func List(ctx context.Context, db *gorm.DB, includeArchived bool, limit int) ([]domain.Automation, error) {
	defer kernel.DeferLatency(kernel.OpListAutomations)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.automations.List")
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	q := db.WithContext(ctx).Order("title ASC").Limit(limit)
	if !includeArchived {
		q = q.Where("archived_at IS NULL")
	}
	var rows []domain.Automation
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list automations: %w", err)
	}
	return rows, nil
}

// GetByID returns one automation by id, including archived rows.
func GetByID(ctx context.Context, db *gorm.DB, id string) (domain.Automation, error) {
	defer kernel.DeferLatency(kernel.OpGetAutomation)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.automations.GetByID")
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Automation{}, fmt.Errorf("%w: automation id required", domain.ErrInvalidInput)
	}
	var row domain.Automation
	if err := db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.Automation{}, mapNotFound(err)
	}
	return row, nil
}

// Update applies a partial patch and returns the updated row.
func Update(ctx context.Context, db *gorm.DB, id string, input UpdateInput) (domain.Automation, error) {
	defer kernel.DeferLatency(kernel.OpUpdateAutomation)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.automations.Update")
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Automation{}, fmt.Errorf("%w: automation id required", domain.ErrInvalidInput)
	}
	if input.Title == nil && input.Description == nil {
		return domain.Automation{}, fmt.Errorf("%w: no fields to update", domain.ErrInvalidInput)
	}
	var out domain.Automation
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row domain.Automation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error; err != nil {
			return mapNotFound(err)
		}
		if row.ArchivedAt != nil {
			return fmt.Errorf("%w: automation is archived", domain.ErrInvalidInput)
		}
		if input.Title != nil {
			title, _, err := domain.ValidateAutomationFields(*input.Title, row.Description)
			if err != nil {
				return err
			}
			if err := assertTitleAvailable(ctx, tx, title, id); err != nil {
				return err
			}
			row.Title = title
		}
		if input.Description != nil {
			_, description, err := domain.ValidateAutomationFields(row.Title, *input.Description)
			if err != nil {
				return err
			}
			row.Description = description
		}
		row.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&row).Error; err != nil {
			return mapWriteError(err)
		}
		out = row
		return nil
	})
	if err != nil {
		return domain.Automation{}, err
	}
	return out, nil
}

// Archive soft-deletes an automation.
func Archive(ctx context.Context, db *gorm.DB, id string) error {
	defer kernel.DeferLatency(kernel.OpArchiveAutomation)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.automations.Archive")
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: automation id required", domain.ErrInvalidInput)
	}
	now := time.Now().UTC()
	res := db.WithContext(ctx).Model(&domain.Automation{}).
		Where("id = ? AND archived_at IS NULL", id).
		Updates(map[string]any{"archived_at": now, "updated_at": now})
	if res.Error != nil {
		return mapWriteError(res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ValidateSelectionIDs ensures every id refers to a non-archived automation.
func ValidateSelectionIDs(ctx context.Context, db *gorm.DB, selections []domain.AutomationSelection) error {
	if len(selections) == 0 {
		return nil
	}
	ids := make([]string, len(selections))
	for i, sel := range selections {
		ids[i] = sel.AutomationID
	}
	var count int64
	if err := db.WithContext(ctx).Model(&domain.Automation{}).
		Where("id IN ? AND archived_at IS NULL", ids).
		Count(&count).Error; err != nil {
		return fmt.Errorf("automation lookup: %w", err)
	}
	if int(count) != len(ids) {
		return fmt.Errorf("%w: unknown or archived automation in selection", domain.ErrInvalidInput)
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ResolveForTask loads library rows for task selections. Missing or archived
// ids are skipped — callers log warnings for audit.
func ResolveForTask(ctx context.Context, db *gorm.DB, selections []domain.AutomationSelection) ([]domain.ResolvedAutomation, error) {
	if len(selections) == 0 {
		return nil, nil
	}
	ids := make([]string, len(selections))
	for i, sel := range selections {
		ids[i] = sel.AutomationID
	}
	var rows []domain.Automation
	if err := db.WithContext(ctx).
		Where("id IN ? AND archived_at IS NULL", ids).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("resolve automations: %w", err)
	}
	rowByID := make(map[string]domain.Automation, len(rows))
	for _, row := range rows {
		rowByID[row.ID] = row
	}
	out := make([]domain.ResolvedAutomation, 0, len(selections))
	for _, sel := range selections {
		row, ok := rowByID[sel.AutomationID]
		if !ok {
			continue
		}
		out = append(out, domain.ResolvedAutomation{
			AutomationID: row.ID,
			Title:        row.Title,
			Description:  row.Description,
			State:        sel.State,
		})
	}
	return out, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func assertTitleAvailable(ctx context.Context, db *gorm.DB, title, excludeID string) error {
	title = strings.TrimSpace(title)
	q := db.WithContext(ctx).Model(&domain.Automation{}).
		Where("LOWER(title) = LOWER(?) AND archived_at IS NULL", title)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return fmt.Errorf("automation title lookup: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w: automation title already exists", domain.ErrInvalidInput)
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapNotFound(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") {
		return fmt.Errorf("%w: automation conflict", domain.ErrConflict)
	}
	return err
}
