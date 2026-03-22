package tasks

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

type CreateTaskInput struct {
	ID            string
	Title         string
	InitialPrompt string
	Status        Status
	Priority      Priority
}

type UpdateTaskInput struct {
	Title         *string
	InitialPrompt *string
	Status        *Status
	Priority      *Priority
}

func (s *Store) Create(ctx context.Context, in CreateTaskInput, by Actor) (*Task, error) {
	if err := validateActor(by); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: title required", ErrInvalidInput)
	}
	st := in.Status
	if st == "" {
		st = StatusReady
	}
	if !validStatus(st) {
		return nil, fmt.Errorf("%w: status", ErrInvalidInput)
	}
	pr := in.Priority
	if pr == "" {
		pr = PriorityMedium
	}
	if !validPriority(pr) {
		return nil, fmt.Errorf("%w: priority", ErrInvalidInput)
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = "task_" + uuid.NewString()
	}

	t := &Task{
		ID:            id,
		Title:         title,
		InitialPrompt: in.InitialPrompt,
		Status:        st,
		Priority:      pr,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(t).Error; err != nil {
			return fmt.Errorf("insert task: %w", err)
		}
		return appendEvent(tx, t.ID, 1, EventTaskCreated, by, nil)
	})
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	return t, nil
}

func (s *Store) Get(ctx context.Context, id string) (*Task, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", ErrInvalidInput)
	}
	var t Task
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get task: %w", err)
	}
	return &t, nil
}

func (s *Store) List(ctx context.Context, limit, offset int) ([]Task, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	var out []Task
	err := s.db.WithContext(ctx).
		Order("id ASC").
		Limit(limit).
		Offset(offset).
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return out, nil
}

func (s *Store) Update(ctx context.Context, id string, in UpdateTaskInput, by Actor) (*Task, error) {
	if err := validateActor(by); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", ErrInvalidInput)
	}
	if in.Title == nil && in.InitialPrompt == nil && in.Status == nil && in.Priority == nil {
		return nil, fmt.Errorf("%w: no fields to update", ErrInvalidInput)
	}

	var updated *Task
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cur Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&cur).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("load task: %w", err)
		}

		nextSeq, err := nextEventSeq(tx, id)
		if err != nil {
			return err
		}
		if err := applyTaskPatches(tx, id, &cur, in, by, nextSeq); err != nil {
			return err
		}

		if err := tx.Save(&cur).Error; err != nil {
			return fmt.Errorf("save task: %w", err)
		}
		updated = &cur
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update task: %w", err)
	}
	return updated, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: id", ErrInvalidInput)
	}
	res := s.db.WithContext(ctx).Where("id = ?", id).Delete(&Task{})
	if res.Error != nil {
		return fmt.Errorf("delete task: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func validStatus(s Status) bool {
	switch s {
	case StatusReady, StatusRunning, StatusBlocked, StatusReview, StatusDone, StatusFailed:
		return true
	default:
		return false
	}
}

func validPriority(p Priority) bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical:
		return true
	default:
		return false
	}
}

func validateActor(a Actor) error {
	switch a {
	case ActorUser, ActorAgent:
		return nil
	default:
		return fmt.Errorf("%w: actor", ErrInvalidInput)
	}
}
