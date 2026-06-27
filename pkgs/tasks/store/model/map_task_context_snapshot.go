package model

import (
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
