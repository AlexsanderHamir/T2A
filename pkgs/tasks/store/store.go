package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
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
	ID               string
	Title            string
	InitialPrompt    string
	Status           domain.Status
	Priority         domain.Priority
	ParentID         *string
	ChecklistInherit bool
}

// ParentFieldPatch updates parent_id when non-nil. Clear true means set parent to null.
type ParentFieldPatch struct {
	Clear bool
	ID    string
}

type UpdateTaskInput struct {
	Title            *string
	InitialPrompt    *string
	Status           *domain.Status
	Priority         *domain.Priority
	Parent           *ParentFieldPatch
	ChecklistInherit *bool
}

func (s *Store) Create(ctx context.Context, in CreateTaskInput, by domain.Actor) (*domain.Task, error) {
	if err := validateActor(by); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: title required", domain.ErrInvalidInput)
	}
	st := in.Status
	if st == "" {
		st = domain.StatusReady
	}
	if !validStatus(st) {
		return nil, fmt.Errorf("%w: status", domain.ErrInvalidInput)
	}
	pr := in.Priority
	if pr == "" {
		pr = domain.PriorityMedium
	}
	if !validPriority(pr) {
		return nil, fmt.Errorf("%w: priority", domain.ErrInvalidInput)
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = uuid.NewString()
	}

	parentID := in.ParentID
	if parentID != nil {
		p := strings.TrimSpace(*parentID)
		if p == "" {
			parentID = nil
		} else {
			parentID = &p
		}
	}
	if in.ChecklistInherit && (parentID == nil || *parentID == "") {
		return nil, fmt.Errorf("%w: checklist_inherit requires parent_id", domain.ErrInvalidInput)
	}

	t := &domain.Task{
		ID:               id,
		Title:            title,
		InitialPrompt:    in.InitialPrompt,
		Status:           st,
		Priority:         pr,
		ParentID:         parentID,
		ChecklistInherit: in.ChecklistInherit,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if parentID != nil {
			var n int64
			if err := tx.Model(&domain.Task{}).Where("id = ?", *parentID).Count(&n).Error; err != nil {
				return fmt.Errorf("parent lookup: %w", err)
			}
			if n == 0 {
				return fmt.Errorf("%w: parent not found", domain.ErrInvalidInput)
			}
		}
		if err := tx.Create(t).Error; err != nil {
			return fmt.Errorf("insert task: %w", err)
		}
		seq := int64(1)
		if err := appendEvent(tx, t.ID, seq, domain.EventTaskCreated, by, nil); err != nil {
			return err
		}
		seq++
		if parentID != nil {
			b, err := json.Marshal(map[string]string{"parent_id": *parentID})
			if err != nil {
				return err
			}
			if err := appendEvent(tx, t.ID, seq, domain.EventSubtaskAdded, by, b); err != nil {
				return err
			}
			seq++
		}
		if st == domain.StatusDone {
			if err := validateCanMarkDoneTx(tx, t.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	return t, nil
}

func (s *Store) Get(ctx context.Context, id string) (*domain.Task, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var t domain.Task
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get task: %w", err)
	}
	return &t, nil
}

// ListTaskEvents returns audit events for a task in ascending sequence order.
func (s *Store) ListTaskEvents(ctx context.Context, taskID string) ([]domain.TaskEvent, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var events []domain.TaskEvent
	err := s.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("seq ASC").
		Find(&events).Error
	if err != nil {
		return nil, fmt.Errorf("list task events: %w", err)
	}
	return events, nil
}

func (s *Store) Update(ctx context.Context, id string, in UpdateTaskInput, by domain.Actor) (*domain.Task, error) {
	if err := validateActor(by); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	if in.Title == nil && in.InitialPrompt == nil && in.Status == nil && in.Priority == nil && in.Parent == nil && in.ChecklistInherit == nil {
		return nil, fmt.Errorf("%w: no fields to update", domain.ErrInvalidInput)
	}

	var updated *domain.Task
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cur domain.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&cur).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
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
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("update task: %w", err)
	}
	return updated, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var childCount int64
	if err := s.db.WithContext(ctx).Model(&domain.Task{}).Where("parent_id = ?", id).Count(&childCount).Error; err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if childCount > 0 {
		return fmt.Errorf("%w: delete subtasks first", domain.ErrInvalidInput)
	}
	res := s.db.WithContext(ctx).Where("id = ?", id).Delete(&domain.Task{})
	if res.Error != nil {
		return fmt.Errorf("delete task: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func validStatus(s domain.Status) bool {
	switch s {
	case domain.StatusReady, domain.StatusRunning, domain.StatusBlocked, domain.StatusReview, domain.StatusDone, domain.StatusFailed:
		return true
	default:
		return false
	}
}

func validPriority(p domain.Priority) bool {
	switch p {
	case domain.PriorityLow, domain.PriorityMedium, domain.PriorityHigh, domain.PriorityCritical:
		return true
	default:
		return false
	}
}

func validateActor(a domain.Actor) error {
	switch a {
	case domain.ActorUser, domain.ActorAgent:
		return nil
	default:
		return fmt.Errorf("%w: actor", domain.ErrInvalidInput)
	}
}
