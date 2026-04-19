// Package domain defines task types ([Task], [TaskEvent], [TaskCycle], [TaskCyclePhase]),
// enums ([Status], [Priority], [EventType], [Actor], [Phase], [CycleStatus], [PhaseStatus]),
// sentinel errors ([ErrNotFound], [ErrInvalidInput], [ErrConflict]), and database driver Scan/Value methods
// for those enums. Execution-cycle state-machine helpers ([ValidPhaseTransition],
// [TerminalCycleStatus], [TerminalPhaseStatus]) live in cycle_state.go.
//
// Persistence is implemented in package store (github.com/AlexsanderHamir/T2A/pkgs/tasks/store).
// HTTP routes in package handler (github.com/AlexsanderHamir/T2A/pkgs/tasks/handler).
// Open database pool and AutoMigrate in package postgres (github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres).
//
// Tests for enum scanning live in sqltypes_test.go. See the parent module path
// github.com/AlexsanderHamir/T2A/pkgs/tasks for a full subsystem overview.
package domain
