package store

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"github.com/google/uuid"
)

func buildCreateTaskFromInput(in CreateTaskInput, by domain.Actor) (t *domain.Task, title string, parentID *string, st domain.Status, err error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.buildCreateTaskFromInput")
	if err := kernel.ValidateActor(by); err != nil {
		return nil, "", nil, "", err
	}
	title = strings.TrimSpace(in.Title)
	if title == "" {
		return nil, "", nil, "", fmt.Errorf("%w: title required", domain.ErrInvalidInput)
	}
	st = in.Status
	if st == "" {
		st = domain.StatusReady
	}
	if !kernel.ValidStatus(st) {
		return nil, "", nil, "", fmt.Errorf("%w: status", domain.ErrInvalidInput)
	}
	pr := in.Priority
	if pr == "" {
		return nil, "", nil, "", fmt.Errorf("%w: priority required", domain.ErrInvalidInput)
	}
	if !kernel.ValidPriority(pr) {
		return nil, "", nil, "", fmt.Errorf("%w: priority", domain.ErrInvalidInput)
	}
	tt := in.TaskType
	if tt == "" {
		tt = domain.TaskTypeGeneral
	}
	if !kernel.ValidTaskType(tt) {
		return nil, "", nil, "", fmt.Errorf("%w: task_type", domain.ErrInvalidInput)
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = uuid.NewString()
	}

	parentID = in.ParentID
	if parentID != nil {
		p := strings.TrimSpace(*parentID)
		if p == "" {
			parentID = nil
		} else {
			parentID = &p
		}
	}
	if in.ChecklistInherit && (parentID == nil || *parentID == "") {
		return nil, "", nil, "", fmt.Errorf("%w: checklist_inherit requires parent_id", domain.ErrInvalidInput)
	}

	t = &domain.Task{
		ID:               id,
		Title:            title,
		InitialPrompt:    in.InitialPrompt,
		Status:           st,
		Priority:         pr,
		TaskType:         tt,
		ParentID:         parentID,
		ChecklistInherit: in.ChecklistInherit,
	}
	return t, title, parentID, st, nil
}
