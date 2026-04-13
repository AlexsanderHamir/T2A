package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type DraftSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

type DraftDetail struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Payload   json.RawMessage `json:"payload"`
	UpdatedAt time.Time       `json:"updated_at"`
	CreatedAt time.Time       `json:"created_at"`
}

func (s *Store) SaveDraft(ctx context.Context, id, name string, payload json.RawMessage) (*DraftSummary, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.SaveDraft")
	id = strings.TrimSpace(id)
	if id == "" {
		id = uuid.NewString()
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: draft name required", domain.ErrInvalidInput)
	}
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	now := time.Now().UTC()
	row := domain.TaskDraft{
		ID:          id,
		Name:        name,
		PayloadJSON: datatypes.JSON(payload),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.WithContext(ctx).Where("id = ?", id).FirstOrCreate(&row).Error; err != nil {
		return nil, fmt.Errorf("save draft: %w", err)
	}
	if err := s.db.WithContext(ctx).Model(&domain.TaskDraft{}).Where("id = ?", id).Updates(map[string]any{
		"name":         name,
		"payload_json": payload,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, fmt.Errorf("update draft: %w", err)
	}
	return &DraftSummary{ID: id, Name: name, UpdatedAt: now, CreatedAt: row.CreatedAt}, nil
}

func (s *Store) ListDrafts(ctx context.Context, limit int) ([]DraftSummary, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListDrafts")
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	var rows []domain.TaskDraft
	if err := s.db.WithContext(ctx).Order("updated_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}
	out := make([]DraftSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, DraftSummary{
			ID:        r.ID,
			Name:      r.Name,
			UpdatedAt: r.UpdatedAt,
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *Store) GetDraft(ctx context.Context, id string) (*DraftDetail, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetDraft")
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var row domain.TaskDraft
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, mapDBNotFound(err)
	}
	return &DraftDetail{
		ID:        row.ID,
		Name:      row.Name,
		Payload:   json.RawMessage(row.PayloadJSON),
		UpdatedAt: row.UpdatedAt,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (s *Store) DeleteDraft(ctx context.Context, id string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.DeleteDraft")
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	res := s.db.WithContext(ctx).Where("id = ?", id).Delete(&domain.TaskDraft{})
	if res.Error != nil {
		return fmt.Errorf("delete draft: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func deleteDraftByIDTx(tx *gorm.DB, id string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.deleteDraftByIDTx")
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if err := tx.Where("id = ?", id).Delete(&domain.TaskDraft{}).Error; err != nil {
		return fmt.Errorf("delete draft by id: %w", err)
	}
	return nil
}

func mapDBNotFound(err error) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.mapDBNotFound")
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return fmt.Errorf("db: %w", err)
}
