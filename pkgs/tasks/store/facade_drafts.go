package store

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/drafts"
)

// DraftSummary is the listing-row shape returned by ListDrafts and SaveDraft.
// JSON tags are part of the HTTP contract for /task-drafts; see internal/drafts.
type DraftSummary = drafts.Summary

// DraftDetail is the GET /task-drafts/{id} body shape (Summary fields plus the
// normalized payload). See internal/drafts for normalization rules.
type DraftDetail = drafts.Detail

// SaveDraft upserts a draft row. Empty id assigns a new UUID; payload is
// normalized via kernel.NormalizeJSONObject (object-only, "{}" on null).
func (s *Store) SaveDraft(ctx context.Context, id, name string, payload json.RawMessage) (*DraftSummary, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.SaveDraft")
	return drafts.Save(ctx, s.db, id, name, payload)
}

// ListDrafts returns the most recently updated drafts up to limit (clamped 1..100, default 50).
func (s *Store) ListDrafts(ctx context.Context, limit int) ([]DraftSummary, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListDrafts")
	return drafts.List(ctx, s.db, limit)
}

// GetDraft returns one draft by id; missing rows surface as domain.ErrNotFound.
func (s *Store) GetDraft(ctx context.Context, id string) (*DraftDetail, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetDraft")
	return drafts.Get(ctx, s.db, id)
}

// DeleteDraft removes a draft by id; missing rows return domain.ErrNotFound.
func (s *Store) DeleteDraft(ctx context.Context, id string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.DeleteDraft")
	return drafts.Delete(ctx, s.db, id)
}
