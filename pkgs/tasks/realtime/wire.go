package realtime

// ChangeType names SSE payload types for task and project lifecycle.
type ChangeType string

const (
	TaskCreated           ChangeType = "task_created"
	TaskUpdated           ChangeType = "task_updated"
	TaskDeleted           ChangeType = "task_deleted"
	TaskGateChanged       ChangeType = "task_gate_changed"
	TaskDependencyChanged ChangeType = "task_dependency_changed"
	TaskCycleChanged      ChangeType = "task_cycle_changed"
	AgentRunProgress      ChangeType = "agent_run_progress"
	ProjectCreated        ChangeType = "project_created"
	ProjectUpdated        ChangeType = "project_updated"
	ProjectDeleted        ChangeType = "project_deleted"
	ProjectContextChanged ChangeType = "project_context_changed"
	SettingsChanged       ChangeType = "settings_changed"
	AgentRunCancelled     ChangeType = "agent_run_cancelled"
	Resync                ChangeType = "resync"
)

// Event is one JSON line sent as an SSE data frame. See docs/api.md and
// docs/domain/sse-hub.md for wire contracts and enrichment rules.
type Event struct {
	Type     ChangeType          `json:"type"`
	ID       string              `json:"id"`
	CycleID  string              `json:"cycle_id,omitempty"`
	PhaseSeq int64               `json:"phase_seq,omitempty"`
	Progress *RunProgressPayload `json:"progress,omitempty"`
	Data     any                 `json:"data,omitempty"`
}

// RunProgressPayload is a normalized live runner update. Raw CLI JSON
// stays out of the browser event stream.
type RunProgressPayload struct {
	Kind    string `json:"kind"`
	Subtype string `json:"subtype,omitempty"`
	Message string `json:"message,omitempty"`
	Tool    string `json:"tool,omitempty"`
}
