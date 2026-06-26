package model

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

func FromDomainTaskDraft(d domain.TaskDraft) TaskDraft {
	return TaskDraft{
		ID:          d.ID,
		Name:        d.Name,
		PayloadJSON: datatypesFromRaw(d.PayloadJSON),
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func FromDomainTaskDraftPtr(d *domain.TaskDraft) *TaskDraft {
	if d == nil {
		return nil
	}
	m := FromDomainTaskDraft(*d)
	return &m
}

func ToDomainTaskDraft(m TaskDraft) domain.TaskDraft {
	return domain.TaskDraft{
		ID:          m.ID,
		Name:        m.Name,
		PayloadJSON: rawFromDatatypes(m.PayloadJSON),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func ToDomainTaskDrafts(rows []TaskDraft) []domain.TaskDraft {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.TaskDraft, len(rows))
	for i := range rows {
		out[i] = ToDomainTaskDraft(rows[i])
	}
	return out
}

func FromDomainTaskTemplate(d domain.TaskTemplate) TaskTemplate {
	return TaskTemplate{
		ID:          d.ID,
		Name:        d.Name,
		PayloadJSON: datatypesFromRaw(d.PayloadJSON),
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func FromDomainTaskTemplatePtr(d *domain.TaskTemplate) *TaskTemplate {
	if d == nil {
		return nil
	}
	m := FromDomainTaskTemplate(*d)
	return &m
}

func ToDomainTaskTemplate(m TaskTemplate) domain.TaskTemplate {
	return domain.TaskTemplate{
		ID:          m.ID,
		Name:        m.Name,
		PayloadJSON: rawFromDatatypes(m.PayloadJSON),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func ToDomainTaskTemplates(rows []TaskTemplate) []domain.TaskTemplate {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.TaskTemplate, len(rows))
	for i := range rows {
		out[i] = ToDomainTaskTemplate(rows[i])
	}
	return out
}
