package model

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

// FromDomainTask copies persisted columns from domain.Task to model.Task.
func FromDomainTask(d domain.Task) Task {
	return Task{
		ID:                    d.ID,
		Title:                 d.Title,
		InitialPrompt:         d.InitialPrompt,
		Status:                d.Status,
		Priority:              d.Priority,
		ProjectID:             d.ProjectID,
		ProjectContextItemIDs: append([]string(nil), d.ProjectContextItemIDs...),
		Tags:                  append([]string(nil), d.Tags...),
		Milestone:             d.Milestone,
		Gate:                  d.Gate,
		Runner:                d.Runner,
		CursorModel:           d.CursorModel,
		RunnerConfig:          datatypesFromRaw(d.RunnerConfig),
		PickupNotBefore:       d.PickupNotBefore,
		CriteriaSatisfiedAt:   d.CriteriaSatisfiedAt,
		PendingRetry:          d.PendingRetry,
		WorktreeBranchID:      d.WorktreeBranchID,
	}
}

// ToDomainTask copies persisted columns to domain.Task. DependsOn and CreatedAt
// remain zero until hydrate helpers run.
func ToDomainTask(m Task) domain.Task {
	runnerConfig := rawFromDatatypes(m.RunnerConfig)
	if len(runnerConfig) == 0 {
		runnerConfig = jsonRawObject()
	}
	return domain.Task{
		ID:                    m.ID,
		Title:                 m.Title,
		InitialPrompt:         m.InitialPrompt,
		Status:                m.Status,
		Priority:              m.Priority,
		ProjectID:             m.ProjectID,
		ProjectContextItemIDs: append([]string(nil), m.ProjectContextItemIDs...),
		Tags:                  append([]string(nil), m.Tags...),
		Milestone:             m.Milestone,
		Gate:                  m.Gate,
		Runner:                m.Runner,
		CursorModel:           m.CursorModel,
		RunnerConfig:          runnerConfig,
		PickupNotBefore:       m.PickupNotBefore,
		CriteriaSatisfiedAt:   m.CriteriaSatisfiedAt,
		PendingRetry:          m.PendingRetry,
		WorktreeBranchID:      m.WorktreeBranchID,
	}
}

// ToDomainTasks maps a slice of persistence tasks to domain.Task.
func ToDomainTasks(rows []Task) []domain.Task {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.Task, len(rows))
	for i := range rows {
		out[i] = ToDomainTask(rows[i])
	}
	return out
}

// FromDomainTaskPtr returns nil when d is nil.
func FromDomainTaskPtr(d *domain.Task) *Task {
	if d == nil {
		return nil
	}
	m := FromDomainTask(*d)
	return &m
}

// ToDomainTaskPtr returns nil when m is nil.
func ToDomainTaskPtr(m *Task) *domain.Task {
	if m == nil {
		return nil
	}
	d := ToDomainTask(*m)
	return &d
}
