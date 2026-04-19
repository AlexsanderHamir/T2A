package domain

import (
	"database/sql/driver"
	"fmt"
	"log/slog"
)

// scanStringEnum is the per-row chokepoint every typed Scan delegate funnels
// through. Carrying the slog.Debug here (rather than on each per-type
// wrapper) emits one trace line per row instead of two — see Session 26 in
// .agent/backend-improvement-agent.log for the audit. The static-string
// arg list is cheap; the prior `if slog.Default().Enabled(...)` guard
// added a function-call read on every row without saving any allocation.
func scanStringEnum[T ~string](dst *T, value any) error {
	slog.Debug("trace", "operation", "domain.scanStringEnum")
	if value == nil {
		var zero T
		*dst = zero
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*dst = T(string(v))
	case string:
		*dst = T(v)
	default:
		return fmt.Errorf("tasks: scan %T into enum", value)
	}
	return nil
}

// valueStringEnum is the write-side mirror of scanStringEnum; same single-
// trace-per-row rationale.
func valueStringEnum[T ~string](s T) (driver.Value, error) {
	slog.Debug("trace", "operation", "domain.valueStringEnum")
	return string(s), nil
}

// The per-type Scan/Value methods below are trivial delegates required by
// database/sql to bind the typed enums to driver.Value. They are skip-listed
// in cmd/funclogmeasure/analyze.go because (a) they are pure delegates with
// no decision logic worth tracing and (b) the underlying scanStringEnum /
// valueStringEnum already emit one slog.Debug per row, so per-type logging
// here would double the trace volume on every row read or write.

func (s *Status) Scan(value any) error          { return scanStringEnum(s, value) }
func (s Status) Value() (driver.Value, error)   { return valueStringEnum(s) }
func (p *Priority) Scan(value any) error        { return scanStringEnum(p, value) }
func (p Priority) Value() (driver.Value, error) { return valueStringEnum(p) }
func (t *TaskType) Scan(value any) error        { return scanStringEnum(t, value) }
func (t TaskType) Value() (driver.Value, error) { return valueStringEnum(t) }
func (e *EventType) Scan(value any) error       { return scanStringEnum(e, value) }
func (e EventType) Value() (driver.Value, error) {
	return valueStringEnum(e)
}
func (a *Actor) Scan(value any) error              { return scanStringEnum(a, value) }
func (a Actor) Value() (driver.Value, error)       { return valueStringEnum(a) }
func (p *Phase) Scan(value any) error              { return scanStringEnum(p, value) }
func (p Phase) Value() (driver.Value, error)       { return valueStringEnum(p) }
func (s *CycleStatus) Scan(value any) error        { return scanStringEnum(s, value) }
func (s CycleStatus) Value() (driver.Value, error) { return valueStringEnum(s) }
func (s *PhaseStatus) Scan(value any) error        { return scanStringEnum(s, value) }
func (s PhaseStatus) Value() (driver.Value, error) { return valueStringEnum(s) }
