# ADR-0027: Stream-idle evidence recovery

**Date:** 2026-06-19
**Status:** Accepted
**Deciders:** Harness / agent-worker maintainers

## Context

Cursor execute and verify runs block until `cursor-agent` exits. In production we observed runs where the agent finished tool work (commit + criteria report) but the CLI never emitted a terminal `result` event. With `max_run_duration_seconds = 0`, the harness waited indefinitely and verify never started.

## Decision

1. **Liveness:** Monitor stdout line silence in `adapterkit.DefaultStreamExecWithIdle`. Idle tiers are derived from one setting (`stream_idle_stuck_seconds`, default 60):
   - suspicious at `stuck / 2` (30s)
   - kill warning at `stuck - 5s` (55s)
   - stuck kill at `stuck` (60s)
2. **Grace period:** Idle detection starts only after the first stdout line (cold-start silence does not fire tiers).
3. **Signal:** Idle kill cancels the run context with `adapterkit.ErrStreamIdle`, mapped to `runner.ErrStale` (distinct from wall-clock `runner.ErrTimeout` and operator cancel).
4. **Recovery:** On `ErrStale`, run existing evidence gates instead of inventing new success criteria:
   - **Execute:** `ingestExecuteCommits` + `DecideExecutePostRun` with `EvidenceRecovery: true`
   - **Verify LLM:** `ParseVerifyReport` fallback when the verify cursor run returns stale
5. **Observability:** Reuse `agent_run_progress` with `kind: run_state` subtypes (`idle_suspicious`, `idle_kill_pending`, `idle_recovered`).

## Consequences

### Positive

- Hung runs auto-recover when git/report evidence passes existing gates
- Worker slot freed without operator intervention
- Clear operator warnings before kill

### Negative / Trade-offs

- Long silent shell tools (>60s without stdout) may false-positive; mitigated by configurable `stream_idle_stuck_seconds` (`0` disables)
- Recovery does not apply to wall-clock timeout (agent may still be working)

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Treat stale as `runner_timeout` | Wrong audit semantics; timeout means wall-clock cap |
| Recovery without killing process | Worker stays blocked; simpler to kill then evaluate |
| Tool-call-aware idle heuristics | Over-engineered for v1 |
| Separate settings per idle tier | One knob derives all tiers |
