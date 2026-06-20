// Package templates owns task_templates persistence for POST/GET/PATCH/DELETE
// /task-templates. Payload writes use kernel.NormalizeJSONObject like drafts.
package templates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const logCmd = "taskapi"

type Summary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Detail struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Payload   json.RawMessage `json:"payload"`
	UpdatedAt time.Time       `json:"updated_at"`
	CreatedAt time.Time       `json:"created_at"`
}

func Save(ctx context.Context, db *gorm.DB, id, name string, payload json.RawMessage) (*Summary, error) {
	defer kernel.DeferLatency(kernel.OpSaveTemplate)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.templates.Save")
	id = strings.TrimSpace(id)
	if id == "" {
		id = uuid.NewString()
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: template name required", domain.ErrInvalidInput)
	}
	normalized, err := kernel.NormalizeJSONObject(payload, "payload")
	if err != nil {
		return nil, err
	}
	payload = normalized
	now := time.Now().UTC()
	row := domain.TaskTemplate{
		ID:          id,
		Name:        name,
		PayloadJSON: datatypes.JSON(payload),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.WithContext(ctx).Where("id = ?", id).FirstOrCreate(&row).Error; err != nil {
		return nil, fmt.Errorf("save template: %w", err)
	}
	if err := db.WithContext(ctx).Model(&domain.TaskTemplate{}).Where("id = ?", id).Updates(map[string]any{
		"name":         name,
		"payload_json": payload,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, fmt.Errorf("update template: %w", err)
	}
	return &Summary{ID: id, Name: name, UpdatedAt: now, CreatedAt: row.CreatedAt}, nil
}

func List(ctx context.Context, db *gorm.DB, limit int, q string) ([]Summary, error) {
	defer kernel.DeferLatency(kernel.OpListTemplates)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.templates.List")
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	query := db.WithContext(ctx).Model(&domain.TaskTemplate{}).Order("updated_at DESC").Limit(limit)
	q = strings.TrimSpace(q)
	if q != "" {
		like := "%" + escapeLike(strings.ToLower(q)) + "%"
		query = query.Where("LOWER(name) LIKE ?", like)
	}
	var rows []domain.TaskTemplate
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	out := make([]Summary, 0, len(rows))
	for _, r := range rows {
		out = append(out, Summary{
			ID:        r.ID,
			Name:      r.Name,
			UpdatedAt: r.UpdatedAt,
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func Get(ctx context.Context, db *gorm.DB, id string) (*Detail, error) {
	defer kernel.DeferLatency(kernel.OpGetTemplate)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.templates.Get")
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var row domain.TaskTemplate
	if err := db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &Detail{
		ID:        row.ID,
		Name:      row.Name,
		Payload:   json.RawMessage(row.PayloadJSON),
		UpdatedAt: row.UpdatedAt,
		CreatedAt: row.CreatedAt,
	}, nil
}

func Patch(ctx context.Context, db *gorm.DB, id string, name *string, payload json.RawMessage) (*Detail, error) {
	defer kernel.DeferLatency(kernel.OpPatchTemplate)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.templates.Patch")
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	if name == nil && payload == nil {
		return nil, fmt.Errorf("%w: no fields to update", domain.ErrInvalidInput)
	}
	var row domain.TaskTemplate
	if err := db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, mapNotFound(err)
	}
	updates := map[string]any{"updated_at": time.Now().UTC()}
	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return nil, fmt.Errorf("%w: template name required", domain.ErrInvalidInput)
		}
		updates["name"] = trimmed
	}
	if payload != nil {
		normalized, err := kernel.NormalizeJSONObject(payload, "payload")
		if err != nil {
			return nil, err
		}
		updates["payload_json"] = normalized
	}
	if err := db.WithContext(ctx).Model(&domain.TaskTemplate{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("patch template: %w", err)
	}
	return Get(ctx, db, id)
}

func Delete(ctx context.Context, db *gorm.DB, id string) error {
	defer kernel.DeferLatency(kernel.OpDeleteTemplate)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.templates.Delete")
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	res := db.WithContext(ctx).Where("id = ?", id).Delete(&domain.TaskTemplate{})
	if res.Error != nil {
		return fmt.Errorf("delete template: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func mapNotFound(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return fmt.Errorf("db: %w", err)
}
