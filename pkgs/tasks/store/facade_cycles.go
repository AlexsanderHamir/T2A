package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/cycles"
)

// StartCycleInput is the public re-export of the cycles subpackage
// input struct. The alias keeps every existing call-site unchanged
// while the implementation lives in internal/cycles.
type StartCycleInput = cycles.StartCycleInput

// CompletePhaseInput is the public re-export of the phase completion
// input struct. The alias keeps every existing call-site unchanged
// while the implementation lives in internal/cycles.
type CompletePhaseInput = cycles.CompletePhaseInput

// StartCycle creates a new TaskCycle row with status=running for the
// given task. See cycles.Start for the full contract.
func (s *Store) StartCycle(ctx context.Context, in StartCycleInput) (*domain.TaskCycle, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.StartCycle")
	return cycles.Start(ctx, s.db, in)
}

// TerminateCycle moves a running cycle into a terminal state. See
// cycles.Terminate for the full contract.
func (s *Store) TerminateCycle(ctx context.Context, cycleID string, status domain.CycleStatus, reason string, by domain.Actor) (*domain.TaskCycle, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.TerminateCycle")
	return cycles.Terminate(ctx, s.db, cycleID, status, reason, by)
}

// GetCycle returns one cycle by id; ErrNotFound when missing.
func (s *Store) GetCycle(ctx context.Context, cycleID string) (*domain.TaskCycle, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetCycle")
	return cycles.Get(ctx, s.db, cycleID)
}

// ListCyclesForTask returns cycles for a task ordered by attempt_seq
// DESC (newest first); limit is clamped to [1, 200].
func (s *Store) ListCyclesForTask(ctx context.Context, taskID string, limit int) ([]domain.TaskCycle, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListCyclesForTask")
	return cycles.ListForTask(ctx, s.db, taskID, limit)
}

// StartPhase appends a new phase row to a running cycle. See
// cycles.StartPhase for the full state-machine and dual-write
// contract.
func (s *Store) StartPhase(ctx context.Context, cycleID string, phase domain.Phase, by domain.Actor) (*domain.TaskCyclePhase, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.StartPhase")
	return cycles.StartPhase(ctx, s.db, cycleID, phase, by)
}

// CompletePhase moves a running phase to a terminal status. See
// cycles.CompletePhase for the full contract.
func (s *Store) CompletePhase(ctx context.Context, in CompletePhaseInput) (*domain.TaskCyclePhase, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.CompletePhase")
	return cycles.CompletePhase(ctx, s.db, in)
}

// ListPhasesForCycle returns phases for cycleID in execution order
// (phase_seq ASC).
func (s *Store) ListPhasesForCycle(ctx context.Context, cycleID string) ([]domain.TaskCyclePhase, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListPhasesForCycle")
	return cycles.ListPhasesForCycle(ctx, s.db, cycleID)
}

// ListRunningCycles returns every cycle currently in CycleStatusRunning
// across all tasks (no per-task filter, no limit). Used by the agent
// worker's startup orphan sweep — the worker calls it once at boot to
// find cycles left dangling by a previous crash. Read-only.
func (s *Store) ListRunningCycles(ctx context.Context) ([]domain.TaskCycle, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListRunningCycles")
	return cycles.ListRunning(ctx, s.db)
}

// ListRunningCyclePhases returns every phase row currently in
// PhaseStatusRunning across all cycles (no filter, no limit). Used by
// the startup orphan sweep so phase rows whose parent cycle already
// terminated are not stranded mid-state. Read-only.
func (s *Store) ListRunningCyclePhases(ctx context.Context) ([]domain.TaskCyclePhase, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListRunningCyclePhases")
	return cycles.ListRunningPhases(ctx, s.db)
}
