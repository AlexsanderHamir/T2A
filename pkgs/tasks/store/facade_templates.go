package store

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/templates"
)

type TemplateSummary = templates.Summary

type TemplateDetail = templates.Detail

func (s *Store) SaveTemplate(ctx context.Context, id, name string, payload json.RawMessage) (*TemplateSummary, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.SaveTemplate")
	return templates.Save(ctx, s.db, id, name, payload)
}

func (s *Store) ListTemplates(ctx context.Context, limit int, q string) ([]TemplateSummary, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListTemplates")
	return templates.List(ctx, s.db, limit, q)
}

func (s *Store) GetTemplate(ctx context.Context, id string) (*TemplateDetail, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetTemplate")
	return templates.Get(ctx, s.db, id)
}

func (s *Store) PatchTemplate(ctx context.Context, id string, name *string, payload json.RawMessage) (*TemplateDetail, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.PatchTemplate")
	return templates.Patch(ctx, s.db, id, name, payload)
}

func (s *Store) DeleteTemplate(ctx context.Context, id string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.DeleteTemplate")
	return templates.Delete(ctx, s.db, id)
}
