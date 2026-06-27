package model

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskDraft(d domain.TaskDraft) TaskDraft {
	return TaskDraft{
		ID:          d.ID,
		Name:        d.Name,
		PayloadJSON: d.PayloadJSON,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskDraftPtr(d *domain.TaskDraft) *TaskDraft {
	if d == nil {
		return nil
	}
	m := FromDomainTaskDraft(*d)
	return &m
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainTaskDraft(m TaskDraft) domain.TaskDraft {
	return domain.TaskDraft{
		ID:          m.ID,
		Name:        m.Name,
		PayloadJSON: m.PayloadJSON,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskTemplate(d domain.TaskTemplate) TaskTemplate {
	return TaskTemplate{
		ID:          d.ID,
		Name:        d.Name,
		PayloadJSON: d.PayloadJSON,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskTemplatePtr(d *domain.TaskTemplate) *TaskTemplate {
	if d == nil {
		return nil
	}
	m := FromDomainTaskTemplate(*d)
	return &m
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainTaskTemplate(m TaskTemplate) domain.TaskTemplate {
	return domain.TaskTemplate{
		ID:          m.ID,
		Name:        m.Name,
		PayloadJSON: m.PayloadJSON,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
