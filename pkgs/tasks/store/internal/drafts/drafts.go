// Package drafts owns task_drafts persistence — the saved-form rows
// that back POST/GET/DELETE /task-drafts. Every payload write goes
// through kernel.NormalizeJSONObject so the documented invariant that
// GET /task-drafts/{id} returns payload as a JSON object is enforced
// in one place. The public store facade re-exports Summary and Detail
// as DraftSummary / DraftDetail, and the four CRUD methods.
//
// DeleteByIDInTx is exported so other store subpackages (in particular
// internal/tasks for Create-from-draft) can clean up the row inside
// their own transaction without taking a dependency on the public
// facade.
package drafts

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

// Summary is the listing-row shape returned by List and Save. Field
// tags are part of the HTTP contract (handler writes the value
// directly via writeJSON); do not rename without updating
// docs/API-HTTP.md and the web client.
type Summary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Detail is the GET /task-drafts/{id} body shape: Summary fields plus
// the normalized payload. Payload is always a JSON object — see
// kernel.NormalizeJSONObject for the on-write rules.
type Detail struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Payload   json.RawMessage `json:"payload"`
	UpdatedAt time.Time       `json:"updated_at"`
	CreatedAt time.Time       `json:"created_at"`
}

// Save upserts a draft row keyed by id. An empty id generates a new
// UUIDv4 so the handler can keep POST /task-drafts idempotent on the
// id-supplied path. name is trimmed and required; payload runs
// through kernel.NormalizeJSONObject.
func Save(ctx context.Context, db *gorm.DB, id, name string, payload json.RawMessage) (*Summary, error) {
	defer kernel.DeferLatency(kernel.OpSaveDraft)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.drafts.Save")
	id = strings.TrimSpace(id)
	if id == "" {
		id = uuid.NewString()
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: draft name required", domain.ErrInvalidInput)
	}
	normalized, err := kernel.NormalizeJSONObject(payload, "payload")
	if err != nil {
		return nil, err
	}
	payload = normalized
	now := time.Now().UTC()
	row := domain.TaskDraft{
		ID:          id,
		Name:        name,
		PayloadJSON: datatypes.JSON(payload),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.WithContext(ctx).Where("id = ?", id).FirstOrCreate(&row).Error; err != nil {
		return nil, fmt.Errorf("save draft: %w", err)
	}
	if err := db.WithContext(ctx).Model(&domain.TaskDraft{}).Where("id = ?", id).Updates(map[string]any{
		"name":         name,
		"payload_json": payload,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, fmt.Errorf("update draft: %w", err)
	}
	return &Summary{ID: id, Name: name, UpdatedAt: now, CreatedAt: row.CreatedAt}, nil
}

// List returns the most recently updated drafts up to limit. limit is
// clamped to [1, 100] with a default of 50 (mirrors the handler
// validation; defense-in-depth).
func List(ctx context.Context, db *gorm.DB, limit int) ([]Summary, error) {
	defer kernel.DeferLatency(kernel.OpListDrafts)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.drafts.List")
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	var rows []domain.TaskDraft
	if err := db.WithContext(ctx).Order("updated_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
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

// Get returns the full Detail for one draft id. id is trimmed; an
// empty trimmed id is rejected with domain.ErrInvalidInput. Missing
// rows surface as domain.ErrNotFound via mapNotFound.
func Get(ctx context.Context, db *gorm.DB, id string) (*Detail, error) {
	defer kernel.DeferLatency(kernel.OpGetDraft)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.drafts.Get")
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var row domain.TaskDraft
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

// Delete removes a draft by id. Trims id; empty id returns
// domain.ErrInvalidInput. Returns domain.ErrNotFound when the row
// does not exist (RowsAffected == 0) so the handler can map to 404.
func Delete(ctx context.Context, db *gorm.DB, id string) error {
	defer kernel.DeferLatency(kernel.OpDeleteDraft)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.drafts.Delete")
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	res := db.WithContext(ctx).Where("id = ?", id).Delete(&domain.TaskDraft{})
	if res.Error != nil {
		return fmt.Errorf("delete draft: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteByIDInTx removes the draft row keyed by id inside the open
// transaction tx. Empty id is a no-op (mirrors the previous
// deleteDraftByIDTx semantics, used by Create-from-draft so the
// linked draft is dropped atomically with the new task). Missing
// rows are not treated as an error (the create path may run before
// any draft was ever saved).
func DeleteByIDInTx(tx *gorm.DB, id string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.drafts.DeleteByIDInTx")
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if err := tx.Where("id = ?", id).Delete(&domain.TaskDraft{}).Error; err != nil {
		return fmt.Errorf("delete draft by id: %w", err)
	}
	return nil
}

// mapNotFound translates gorm.ErrRecordNotFound into the public
// sentinel domain.ErrNotFound so handlers can use errors.Is without
// importing gorm.
func mapNotFound(err error) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.drafts.mapNotFound")
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return fmt.Errorf("db: %w", err)
}
