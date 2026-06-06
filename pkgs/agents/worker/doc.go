// Package worker is the V1 in-process consumer of the ready-task queue.
//
// One Worker drives one task end-to-end through the execute -> verify
// substrate defined in docs/architecture.md. V1 records a single
// execute phase row per cycle that wraps the actual runner.Runner
// invocation; the verify phase is opened on demand by follow-up work
// (a corrective execute may follow a failed verify — see
// domain.ValidPhaseTransition).
//
// Lifecycle per task (full contract in docs/architecture.md):
//
//  1. Receive(ctx) blocks on the MemoryQueue.
//  2. Reload the task; if it is no longer StatusReady, log warn and ack.
//  3. Transition task ready -> running.
//  4. StartCycle with MetaJSON {"runner","runner_version","prompt_hash"}.
//  5. StartPhase(execute) -> runner.Run -> CompletePhase(succeeded|failed).
//  6. TerminateCycle(succeeded|failed) and Update task -> done|failed.
//  7. AckAfterRecv as the very last step so a redelivery during the run
//     is ignored by the queue's pending-set guard.
//
// Edge cases the package handles directly:
//
//   - Panic in runner.Run is caught by defer recover; if a cycle was
//     started, the recovery path best-effort completes the running phase
//     as PhaseStatusFailed, terminates the cycle as CycleStatusFailed
//     with reason "panic", and walks the task to StatusFailed using a
//     non-cancelled background context with ShutdownAbortTimeout
//     deadline. The Run loop continues to the next task.
//   - Shutdown mid-runner: when the parent context cancels while
//     runner.Run is in flight, the worker uses the same background
//     context + deadline pattern to complete the execute phase, write
//     TerminateCycle(..., aborted, "shutdown"), and walk the task to
//     StatusFailed so the operator sees an honest audit row even though
//     the request context is dead.
//   - SSE fan-out: the optional CycleChangeNotifier is invoked after
//     every successful StartCycle / StartPhase / CompletePhase /
//     TerminateCycle so the SPA's existing task_cycle_changed cache
//     slot lights up immediately. cmd/taskapi wires SSEHub through a
//     tiny adapter; tests pass nil and the calls become no-ops.
//
// Out of scope for V1 (deferred to V2-V4 of
// docs/architecture.md): retries, concurrency, leases,
// dead-letter, runner selection, per-phase decomposition. The startup
// orphan-cycle sweep that complements this worker is wired in
// cmd/taskapi (see docs/architecture.md "Process restart and the
// orphan sweep").
package worker
