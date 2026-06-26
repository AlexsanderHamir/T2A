// Package templates owns task_templates persistence for POST/GET/PATCH/DELETE
// /task-templates. Payload writes use kernel.NormalizeJSONObject like drafts.
package templates

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/internal/kernel"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/internal/namedpayload"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"gorm.io/gorm"
)

type Summary = namedpayload.Summary
type Detail = namedpayload.Detail

//funclogmeasure:skip category=hot-path reason="Thin delegate to namedpayload; operation trace is emitted there."
func Save(ctx context.Context, db *gorm.DB, id, name string, payload json.RawMessage) (*Summary, error) {
	return namedpayload.SaveTemplate(ctx, db, id, name, payload)
}

//funclogmeasure:skip category=hot-path reason="Thin delegate to namedpayload; operation trace is emitted there."
func List(ctx context.Context, db *gorm.DB, limit int, q string) ([]Summary, error) {
	return namedpayload.ListTemplates(ctx, db, limit, q)
}

//funclogmeasure:skip category=hot-path reason="Thin delegate to namedpayload; operation trace is emitted there."
func Get(ctx context.Context, db *gorm.DB, id string) (*Detail, error) {
	return namedpayload.GetTemplate(ctx, db, id)
}

func Patch(ctx context.Context, db *gorm.DB, id string, name *string, payload json.RawMessage) (*Detail, error) {
	defer kernel.DeferLatency(kernel.OpPatchTemplate)()
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.templates.Patch")
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	if name == nil && payload == nil {
		return nil, fmt.Errorf("%w: no fields to update", domain.ErrInvalidInput)
	}
	var row model.TaskTemplate
	if err := db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, kernel.MapNotFound(err)
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
	if err := db.WithContext(ctx).Model(&model.TaskTemplate{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("patch template: %w", err)
	}
	return Get(ctx, db, id)
}

//funclogmeasure:skip category=hot-path reason="Thin delegate to namedpayload; operation trace is emitted there."
func Delete(ctx context.Context, db *gorm.DB, id string) error {
	return namedpayload.DeleteTemplate(ctx, db, id)
}
