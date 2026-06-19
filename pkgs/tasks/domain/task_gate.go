package domain

import "time"

// GateKind identifies how a task gate is satisfied.
type GateKind string

const (
	GateKindManualApproval GateKind = "manual_approval"
)

// TaskGate pauses agent dequeue for a task until released. Nil gate on a task
// means no gate — the worker applies only status, pickup, and depends_on rules.
type TaskGate struct {
	Kind                      GateKind        `json:"kind"`
	Status                    GateStatus      `json:"status"`
	Hold                      bool            `json:"hold"`
	PendingReleaseDeadlineUTC *time.Time      `json:"pending_release_deadline,omitempty"`
	Criteria                  []GateCriterion `json:"criteria,omitempty"`
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// GateBlocksWorker reports whether the gate prevents the worker from dequeuing
// the task. Only released (or absent) gates allow pickup.
func (g *TaskGate) GateBlocksWorker() bool {
	if g == nil {
		return false
	}
	return g.Status != GateStatusReleased
}
