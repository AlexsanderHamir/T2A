package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// isDuplicateTaskPrimaryKey detects unique/PK violations on task insert across GORM + SQLite + Postgres drivers.
func isDuplicateTaskPrimaryKey(err error) bool {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.isDuplicateTaskPrimaryKey")
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unique constraint failed") {
		return strings.Contains(msg, "tasks") && strings.Contains(msg, "id")
	}
	if strings.Contains(msg, "duplicate key value violates unique constraint") {
		return strings.Contains(msg, "tasks_pkey")
	}
	return false
}

func (s *Store) Create(ctx context.Context, in CreateTaskInput, by domain.Actor) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Create")
	t, title, parentID, st, err := buildCreateTaskFromInput(in, by)
	if err != nil {
		return nil, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return createTaskInTx(tx, t, in, by, title, parentID, st)
	})
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	if t.Status == domain.StatusReady {
		s.notifyReadyTask(ctx, *t)
	}
	return t, nil
}
