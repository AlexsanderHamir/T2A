# ADR-0006: Phase-Boundary Cycle Resume

**Date:** 2026-06-11
**Status:** Accepted (execute commit **marker** policy superseded by [ADR-0014](ADR-0014-cycle-commit-tracking.md); phase-boundary resume model unchanged)
**Deciders:** Engineering

## Context

ADR-0003 deferred automatic git commits during execute. After a `taskapi` process restart, any in-flight agent cycle was treated as orphaned: startup sweep aborted running cycles and failed tasks. That discarded partial progress and forced operators to manually retry from `failed → ready`, starting a new cycle.

The harness already persists enough state to reconstruct progress without a new checkpoint table: phase ledger rows, verify/criteria report tables, context snapshots, and (optionally) git commits tagged with a cycle marker. The runner remains stateless — each `runner.Run` is fresh; resume means rehydrating a logical checkpoint into the composed prompt and continuing the **same open** `task_cycles` row.

## Decision

Replace fail-all orphan sweep with **phase-boundary resume**:

1. **Startup finalization** (`FinalizeInterruptedPhases`): running phase rows → `CompletePhase(failed, "process_restart")`. Cycles and tasks stay `running`.
2. **Reconcile** (`ReconcileRunningTasksNotQueued`): enqueue running tasks whose cycle is still open so the worker can call `Harness.Resume`.
3. **Worker admission**: `status=running` + open cycle → `Harness.Resume`; `status=ready` → existing `Harness.Run`.
4. **Checkpoint loader** (`reconstructCheckpoint`): derive locked criteria, verify attempt, feedback, and resume branch (execute vs verify) from existing tables.
5. **Store transitions**: allow `execute→execute` and `verify→verify` only when the highest-seq phase failed with summary `process_restart`.
6. **Execute commit policy** (default **on**): `app_settings.agent_commit_execute_work` instructs the agent to commit with `t2a:cycle=<cycle_id>` before finishing execute, enabling git-log fallback when the working tree is clean after restart.

Mid-CLI session resume, worker leases, and a dedicated checkpoint table are out of scope for V1.

## Consequences

### Positive

- Process restart no longer fails in-flight tasks; the UI shows the same open cycle continuing.
- Progress is durable via DB reports and (when enabled) tagged commits.
- Runner contract unchanged — checkpoint is encoded in prompt composition only.

### Negative / Trade-offs

- Resume quality depends on prior execute commits when the working tree is clean and commit policy is off.
- Same-phase restart transitions are gated on `process_restart` summary to prevent REST abuse.
- Single-process worker assumption unchanged; multi-replica workers would still race.

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Keep fail-all sweep | Discards partial work; poor operator experience after benign restarts |
| New checkpoint table | Existing ledger + reports sufficient for V1 |
| Resume mid-CLI session | Runner adapters are stateless; Cursor session_id is post-hoc audit only |
| Always auto-commit without prompt contract | ADR-0003 concerns; opt-out + explicit marker keeps behavior deterministic |

## Supersedes / Related

- Supersedes the fail-all behavior described under "Process-restart orphan sweep" in prior `docs/architecture.md` (cycles aborted, tasks failed).
- Builds on ADR-0005 (`pkgs/agents/harness`) as the resume entry point.
- Execute commit **marker** policy (`t2a:cycle=`, `agent_commit_execute_work`) superseded by [ADR-0014](ADR-0014-cycle-commit-tracking.md); phase-boundary resume mechanics unchanged.
