// Package namedpayload implements shared CRUD for named JSON-payload
// entities (task drafts and task templates).
package namedpayload

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
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Summary is the listing-row shape for drafts and templates.
type Summary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Detail is the GET-by-id body shape for drafts and templates.
type Detail struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Payload   json.RawMessage `json:"payload"`
	UpdatedAt time.Time       `json:"updated_at"`
	CreatedAt time.Time       `json:"created_at"`
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func clampLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func saveRow(
	ctx context.Context,
	db *gorm.DB,
	id, name string,
	payload json.RawMessage,
	nameRequiredMsg string,
	opSave string,
	logOp string,
	saveErr string,
	updateErr string,
	newRow func(string, string, datatypes.JSON, time.Time) model.TaskDraft,
) (*Summary, error) {
	defer kernel.DeferLatency(opSave)()
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", logOp)
	id = kernel.ResolveID(id)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidInput, nameRequiredMsg)
	}
	normalized, err := kernel.NormalizeJSONObject(payload, "payload")
	if err != nil {
		return nil, err
	}
	payload = normalized
	now := time.Now().UTC()
	row := newRow(id, name, datatypes.JSON(payload), now)
	if err := db.WithContext(ctx).Where("id = ?", id).FirstOrCreate(&row).Error; err != nil {
		return nil, fmt.Errorf("%s: %w", saveErr, err)
	}
	if err := db.WithContext(ctx).Model(&model.TaskDraft{}).Where("id = ?", id).Updates(map[string]any{
		"name":         name,
		"payload_json": payload,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, fmt.Errorf("%s: %w", updateErr, err)
	}
	return &Summary{ID: id, Name: name, UpdatedAt: now, CreatedAt: row.CreatedAt}, nil
}

func saveTemplateRow(
	ctx context.Context,
	db *gorm.DB,
	id, name string,
	payload json.RawMessage,
	nameRequiredMsg string,
	opSave string,
	logOp string,
	saveErr string,
	updateErr string,
	newRow func(string, string, datatypes.JSON, time.Time) model.TaskTemplate,
) (*Summary, error) {
	defer kernel.DeferLatency(opSave)()
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", logOp)
	id = kernel.ResolveID(id)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidInput, nameRequiredMsg)
	}
	normalized, err := kernel.NormalizeJSONObject(payload, "payload")
	if err != nil {
		return nil, err
	}
	payload = normalized
	now := time.Now().UTC()
	row := newRow(id, name, datatypes.JSON(payload), now)
	if err := db.WithContext(ctx).Where("id = ?", id).FirstOrCreate(&row).Error; err != nil {
		return nil, fmt.Errorf("%s: %w", saveErr, err)
	}
	if err := db.WithContext(ctx).Model(&model.TaskTemplate{}).Where("id = ?", id).Updates(map[string]any{
		"name":         name,
		"payload_json": payload,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, fmt.Errorf("%s: %w", updateErr, err)
	}
	return &Summary{ID: id, Name: name, UpdatedAt: now, CreatedAt: row.CreatedAt}, nil
}

func SaveDraft(ctx context.Context, db *gorm.DB, id, name string, payload json.RawMessage) (*Summary, error) {
	return saveRow(ctx, db, id, name, payload,
		"draft name required",
		kernel.OpSaveDraft,
		"tasks.store.drafts.Save",
		"save draft", "update draft",
		func(id, name string, p datatypes.JSON, now time.Time) model.TaskDraft {
			return model.TaskDraft{ID: id, Name: name, PayloadJSON: p, CreatedAt: now, UpdatedAt: now}
		},
	)
}

func SaveTemplate(ctx context.Context, db *gorm.DB, id, name string, payload json.RawMessage) (*Summary, error) {
	return saveTemplateRow(ctx, db, id, name, payload,
		"template name required",
		kernel.OpSaveTemplate,
		"tasks.store.templates.Save",
		"save template", "update template",
		func(id, name string, p datatypes.JSON, now time.Time) model.TaskTemplate {
			return model.TaskTemplate{ID: id, Name: name, PayloadJSON: p, CreatedAt: now, UpdatedAt: now}
		},
	)
}

func ListDrafts(ctx context.Context, db *gorm.DB, limit int) ([]Summary, error) {
	defer kernel.DeferLatency(kernel.OpListDrafts)()
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.drafts.List")
	limit = clampLimit(limit)
	var rows []model.TaskDraft
	if err := db.WithContext(ctx).Order("updated_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}
	return summariesFromDraftRows(rows), nil
}

func ListTemplates(ctx context.Context, db *gorm.DB, limit int, q string) ([]Summary, error) {
	defer kernel.DeferLatency(kernel.OpListTemplates)()
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.templates.List")
	limit = clampLimit(limit)
	query := db.WithContext(ctx).Model(&model.TaskTemplate{}).Order("updated_at DESC").Limit(limit)
	q = strings.TrimSpace(q)
	if q != "" {
		like := "%" + escapeLike(strings.ToLower(q)) + "%"
		query = query.Where("LOWER(name) LIKE ?", like)
	}
	var rows []model.TaskTemplate
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	return summariesFromTemplateRows(rows), nil
}

func GetDraft(ctx context.Context, db *gorm.DB, id string) (*Detail, error) {
	return getDraftByID(ctx, db, id)
}

func GetTemplate(ctx context.Context, db *gorm.DB, id string) (*Detail, error) {
	return getTemplateByID(ctx, db, id)
}

func getDraftByID(ctx context.Context, db *gorm.DB, id string) (*Detail, error) {
	defer kernel.DeferLatency(kernel.OpGetDraft)()
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.drafts.Get")
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var row model.TaskDraft
	if err := db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, kernel.MapNotFound(err)
	}
	return detailFromDraft(row), nil
}

func getTemplateByID(ctx context.Context, db *gorm.DB, id string) (*Detail, error) {
	defer kernel.DeferLatency(kernel.OpGetTemplate)()
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.templates.Get")
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var row model.TaskTemplate
	if err := db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, kernel.MapNotFound(err)
	}
	return detailFromTemplate(row), nil
}

func DeleteDraft(ctx context.Context, db *gorm.DB, id string) error {
	return deleteByID(ctx, db, id, kernel.OpDeleteDraft, "tasks.store.drafts.Delete", "delete draft", &model.TaskDraft{})
}

func DeleteTemplate(ctx context.Context, db *gorm.DB, id string) error {
	return deleteByID(ctx, db, id, kernel.OpDeleteTemplate, "tasks.store.templates.Delete", "delete template", &model.TaskTemplate{})
}

func deleteByID(ctx context.Context, db *gorm.DB, id string, op string, logOp, deleteErr string, row any) error {
	defer kernel.DeferLatency(op)()
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", logOp)
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	res := db.WithContext(ctx).Where("id = ?", id).Delete(row)
	if res.Error != nil {
		return fmt.Errorf("%s: %w", deleteErr, res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by ListDrafts."
func summariesFromDraftRows(rows []model.TaskDraft) []Summary {
	out := make([]Summary, 0, len(rows))
	for _, r := range rows {
		out = append(out, Summary{ID: r.ID, Name: r.Name, UpdatedAt: r.UpdatedAt, CreatedAt: r.CreatedAt})
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by ListTemplates."
func summariesFromTemplateRows(rows []model.TaskTemplate) []Summary {
	out := make([]Summary, 0, len(rows))
	for _, r := range rows {
		out = append(out, Summary{ID: r.ID, Name: r.Name, UpdatedAt: r.UpdatedAt, CreatedAt: r.CreatedAt})
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func detailFromDraft(row model.TaskDraft) *Detail {
	return &Detail{
		ID: row.ID, Name: row.Name, Payload: json.RawMessage(row.PayloadJSON),
		UpdatedAt: row.UpdatedAt, CreatedAt: row.CreatedAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func detailFromTemplate(row model.TaskTemplate) *Detail {
	return &Detail{
		ID: row.ID, Name: row.Name, Payload: json.RawMessage(row.PayloadJSON),
		UpdatedAt: row.UpdatedAt, CreatedAt: row.CreatedAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by ListTemplates."
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
