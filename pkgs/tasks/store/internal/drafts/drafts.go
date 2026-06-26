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
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/internal/namedpayload"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"gorm.io/gorm"
)

// Summary is the listing-row shape returned by List and Save. Field
// tags are part of the HTTP contract (handler writes the value
// directly via writeJSON); do not rename without updating
// docs/api.md and the web client.
type Summary = namedpayload.Summary

// Detail is the GET /task-drafts/{id} body shape: Summary fields plus
// the normalized payload. Payload is always a JSON object — see
// kernel.NormalizeJSONObject for the on-write rules.
type Detail = namedpayload.Detail

// Save upserts a draft row keyed by id.
//
//funclogmeasure:skip category=hot-path reason="Thin delegate to namedpayload; operation trace is emitted there."
func Save(ctx context.Context, db *gorm.DB, id, name string, payload json.RawMessage) (*Summary, error) {
	return namedpayload.SaveDraft(ctx, db, id, name, payload)
}

// List returns the most recently updated drafts up to limit.
//
//funclogmeasure:skip category=hot-path reason="Thin delegate to namedpayload; operation trace is emitted there."
func List(ctx context.Context, db *gorm.DB, limit int) ([]Summary, error) {
	return namedpayload.ListDrafts(ctx, db, limit)
}

// Get returns the full Detail for one draft id.
//
//funclogmeasure:skip category=hot-path reason="Thin delegate to namedpayload; operation trace is emitted there."
func Get(ctx context.Context, db *gorm.DB, id string) (*Detail, error) {
	return namedpayload.GetDraft(ctx, db, id)
}

// Delete removes a draft by id.
//
//funclogmeasure:skip category=hot-path reason="Thin delegate to namedpayload; operation trace is emitted there."
func Delete(ctx context.Context, db *gorm.DB, id string) error {
	return namedpayload.DeleteDraft(ctx, db, id)
}

// DeleteByIDInTx removes the draft row keyed by id inside the open
// transaction tx. Empty id is a no-op.
func DeleteByIDInTx(tx *gorm.DB, id string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.drafts.DeleteByIDInTx")
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if err := tx.Where("id = ?", id).Delete(&model.TaskDraft{}).Error; err != nil {
		return fmt.Errorf("delete draft by id: %w", err)
	}
	return nil
}
