package store

import "github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"

// TaskNode is a task row plus nested children for API tree responses.
type TaskNode struct {
	domain.Task
	Children []TaskNode `json:"children,omitempty" gorm:"-"`
}
