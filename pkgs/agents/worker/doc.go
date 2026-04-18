// Package worker is the V1 in-process consumer of the ready-task queue.
//
// One Worker drives one task end-to-end through the
// diagnose -> execute -> verify -> persist substrate defined in moat.md.
// V1 records exactly two phase rows per cycle: a no-op diagnose row
// closed as PhaseStatusSkipped to satisfy the
// domain.ValidPhaseTransition "first phase must be diagnose" rule, and
// an execute row that wraps the actual runner.Runner invocation. See
// docs/AGENT-WORKER-PLAN.md (edge case #1) for the rationale.
//
// Lifecycle per task (matches docs/AGENT-WORKER-PLAN.md Stage 3):
//
//  1. Receive(ctx) blocks on the MemoryQueue.
//  2. Reload the task; if it is no longer StatusReady, log warn and ack.
//  3. Transition task ready -> running.
//  4. StartCycle with MetaJSON {"runner","runner_version","prompt_hash"}.
//  5. StartPhase(diagnose) -> CompletePhase(skipped, "single-phase V1; diagnose deferred").
//  6. StartPhase(execute) -> runner.Run -> CompletePhase(succeeded|failed).
//  7. TerminateCycle(succeeded|failed) and Update task -> done|failed.
//  8. AckAfterRecv as the very last step so a redelivery during the run
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
// Out of scope for V1 (deferred per docs/AGENT-WORKER-PLAN.md): retries,
// concurrency, leases, dead-letter, runner selection, per-phase
// decomposition. The startup orphan-cycle sweep that complements this
// worker lives in Stage 4 wiring inside cmd/taskapi.
package worker
