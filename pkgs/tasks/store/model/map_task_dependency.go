package model

import (
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// FromDomainTaskDependency copies a domain row to its persistence model.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func FromDomainTaskDependency(d domain.TaskDependency) TaskDependency {
	return TaskDependency{
		TaskID:          d.TaskID,
		DependsOnTaskID: d.DependsOnTaskID,
		Satisfies:       d.Satisfies,
		CreatedAt:       d.CreatedAt,
	}
}

// ToDomainTaskDependency copies a persistence row to domain.TaskDependency.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainTaskDependency(m TaskDependency) domain.TaskDependency {
	return domain.TaskDependency{
		TaskID:          m.TaskID,
		DependsOnTaskID: m.DependsOnTaskID,
		Satisfies:       m.Satisfies,
		CreatedAt:       m.CreatedAt,
	}
}

// ToDomainTaskDependencies maps persistence rows to domain.TaskDependency.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ToDomainTaskDependencies(rows []TaskDependency) []domain.TaskDependency {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.TaskDependency, len(rows))
	for i := range rows {
		out[i] = ToDomainTaskDependency(rows[i])
	}
	return out
}

// NewTaskDependencyRow builds a persistence row for insert.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func NewTaskDependencyRow(taskID, dependsOnTaskID string, satisfies domain.DependencySatisfies, at time.Time) TaskDependency {
	return TaskDependency{
		TaskID:          taskID,
		DependsOnTaskID: dependsOnTaskID,
		Satisfies:       satisfies,
		CreatedAt:       at,
	}
}
