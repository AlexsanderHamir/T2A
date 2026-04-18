// Package cycles owns the task_cycles + task_cycle_phases write paths
// — the dual-write tier where every state change in the cycle/phase
// model also appends a mirrored audit row to task_events in the same
// SQL transaction. The dual-write is the documented invariant for
// /tasks/{id}/events to remain a complete witness of cycle activity
// (docs/EXECUTION-CYCLES.md, asserted by
// store_cycles_dualwrite_test.go).
//
// The public store facade re-exports StartCycleInput and
// CompletePhaseInput as type aliases plus seven *Store method
// delegations (StartCycle, TerminateCycle, GetCycle,
// ListCyclesForTask, StartPhase, CompletePhase, ListPhasesForCycle).
// All audit appends route through kernel.NextEventSeq +
// kernel.AppendEvent, never through the public events package, so
// the same hot helper guards every cycle/phase event the same way it
// guards CRUD and checklist events.
package cycles
