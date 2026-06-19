package handler

import "github.com/AlexsanderHamir/T2A/pkgs/tasks/realtime"

// Wire types re-exported from pkgs/tasks/realtime so existing handler
// callers keep stable names. New non-handler publishers should import
// realtime directly.
type (
	TaskChangeType          = realtime.ChangeType
	TaskChangeEvent         = realtime.Event
	AgentRunProgressPayload = realtime.RunProgressPayload
)

const (
	TaskCreated           = realtime.TaskCreated
	TaskUpdated           = realtime.TaskUpdated
	TaskDeleted           = realtime.TaskDeleted
	TaskGateChanged       = realtime.TaskGateChanged
	TaskDependencyChanged = realtime.TaskDependencyChanged
	TaskCycleChanged      = realtime.TaskCycleChanged
	AgentRunProgress      = realtime.AgentRunProgress
	ProjectCreated        = realtime.ProjectCreated
	ProjectUpdated        = realtime.ProjectUpdated
	ProjectDeleted        = realtime.ProjectDeleted
	ProjectContextChanged = realtime.ProjectContextChanged
	SettingsChanged       = realtime.SettingsChanged
	AgentRunCancelled     = realtime.AgentRunCancelled
	Resync                = realtime.Resync
)
