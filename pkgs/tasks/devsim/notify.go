package devsim

// ChangeKind classifies SSE notifications emitted during dev simulation.
type ChangeKind int

const (
	// ChangeUpdated maps to SSE task_updated.
	ChangeUpdated ChangeKind = iota
	// ChangeCreated maps to SSE task_created.
	ChangeCreated
	// ChangeDeleted maps to SSE task_deleted.
	ChangeDeleted
)
