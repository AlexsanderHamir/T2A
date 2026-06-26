package model

import (
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func FromDomainTaskContextSnapshot(d domain.TaskContextSnapshot) TaskContextSnapshot {
	return TaskContextSnapshot{
		ID:              d.ID,
		TaskID:          d.TaskID,
		CycleID:         d.CycleID,
		ProjectID:       d.ProjectID,
		ContextJSON:     d.ContextJSON,
		RenderedContext: d.RenderedContext,
		TokenEstimate:   d.TokenEstimate,
		CreatedAt:       d.CreatedAt,
	}
}

func ToDomainTaskContextSnapshot(m TaskContextSnapshot) domain.TaskContextSnapshot {
	return domain.TaskContextSnapshot{
		ID:              m.ID,
		TaskID:          m.TaskID,
		CycleID:         m.CycleID,
		ProjectID:       m.ProjectID,
		ContextJSON:     m.ContextJSON,
		RenderedContext: m.RenderedContext,
		TokenEstimate:   m.TokenEstimate,
		CreatedAt:       m.CreatedAt,
	}
}
