package cycles

import (
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// StartCycleInput captures everything needed to begin a new execution
// attempt for a task. The store decides AttemptSeq; callers cannot
// supply it. Re-aliased by the public store facade as
// store.StartCycleInput so handler code stays unchanged.
type StartCycleInput struct {
	TaskID        string
	TriggeredBy   domain.Actor
	ParentCycleID *string
	// Meta is small free-form runner metadata such as
	// {"runner":"cursor-cli","prompt_hash":"..."}. nil and empty are
	// normalized to the zero JSON object "{}" via
	// kernel.NormalizeJSONObject so the on-disk shape never carries
	// SQL NULL or a non-object value.
	Meta []byte
}

// CompletePhaseInput captures the terminal transition for a phase row,
// keyed by (cycleID, phaseSeq) so the URL-level identifier from
// /cycles/{cycleId}/phases/{phaseSeq} is also the natural store key.
// Re-aliased by the public store facade as store.CompletePhaseInput.
type CompletePhaseInput struct {
	CycleID  string
	PhaseSeq int64
	Status   domain.PhaseStatus
	// Summary is a short human-readable note (nil to leave the column null).
	Summary *string
	// Details is structured per-phase output (verify checks, persist
	// artifact ids, …). nil/empty become the zero JSON object "{}"
	// via kernel.NormalizeJSONObject.
	Details []byte
	// By identifies who recorded the terminal transition; mirrored as
	// the Actor on the audit row in task_events.
	By domain.Actor
}

// AppendStreamEventInput captures one durable normalized progress event for
// a cycle attempt. The store assigns StreamSeq per cycle.
type AppendStreamEventInput struct {
	TaskID   string
	CycleID  string
	PhaseSeq int64
	At       time.Time
	Source   string
	Kind     string
	Subtype  string
	Message  string
	Tool     string
	Payload  []byte
}
