# Harness improvements backlog (ROI-ranked)

**Snapshot:** June 2026

Ranked investment backlog for T2A’s **outer harness** (`pkgs/agents/harness`). Read [HARNESS_LANDSCAPE.md](HARNESS_LANDSCAPE.md) first for industry context; read [docs/domain/harness.md](docs/domain/harness.md) for operational behavior. Implement via ADR + vertical slice per [docs/contributing.md](docs/contributing.md).

> **How to use this doc** — Pick from the top when planning harness work. Re-score when product priorities shift. Each item includes effort, evidence, and a measurable success signal.
>
> **Tracking** — Each improvement title has a todo checkbox (`- [ ]` / `- [x]`). Check an item when it is **shipped** (merged + documented). Add the ADR or PR link in that item’s **Status** row.

---

## ROI rubric

Each candidate is scored 1–5 on four axes, then weighted:

```text
ROI_score = 0.35 × reliability + 0.25 × operator_leverage + 0.25 × (5 - effort_band) + 0.15 × strategic_fit
```

| Axis | 5 | 1 |
| --- | --- | --- |
| **Reliability** | Prevents wrong completions, silent corruption, or audit drift | Cosmetic / rare edge |
| **Operator leverage** | Cuts debug time, wasted agent runs, or manual retries | Internal refactor only |
| **Effort** (mapped to score: S=4, M=3, L=2, XL=1) | S — harness-only, ≤ ~3 files | XL — architectural pivot |
| **Strategic fit** | Extends outer harness without replacing Cursor inner loop | Contradicts V1 delegation |

**Tiers**

| Tier | ROI_score (typical) | Meaning |
| --- | --- | --- |
| **P0 — Highest** | ≥ 3.8 | Do next; strong reliability or cost win, S–M effort |
| **P1 — High** | 3.2 – 3.7 | Meaningful leverage; schedule within a quarter |
| **P2 — Medium** | 2.5 – 3.1 | Worth doing when touched; or enabler for later work |
| **P3 — Low / defer** | < 2.5 | Narrow audience, L+ effort, or V1 intentional deferral |

---

## Investigation summary (June 2026)

Passes run against [HARNESS_LANDSCAPE.md](HARNESS_LANDSCAPE.md), [docs/domain/harness.md](docs/domain/harness.md), and harness code.

| Hypothesis | Finding |
| --- | --- |
| T0 lost on restart | **Mostly mitigated** — `ReconstructCheckpoint` rebuilds `previouslyPassed` and `verifyAttempt` from `task_cycle_verify_reports` ([`checkpoint.go`](pkgs/agents/harness/internal/resume/checkpoint.go)). Cross-cycle operator resume intentionally resets `verifyAttempt` to 0 ([ADR-0015](docs/adr/ADR-0015-dual-retry-modes.md)). Residual risk: `verifyAttempt` mirrors DB `attempt_seq`, not an explicit retry counter — document and test edge cases. |
| Full execute on every verify retry | **Confirmed** — [`cycle_loop.go`](pkgs/agents/harness/cycle_loop.go) `retryLoop` always calls `runCycleLoopExecute` (full `runner.Run`). Verify already skips **locked** criteria within one pass ([`checks.go`](pkgs/agents/harness/internal/verify/checks.go)); cross-cycle **verify-only** exists in [`retry_run.go`](pkgs/agents/harness/retry_run.go) but not in-cycle. |
| End-to-end trace without Cursor logs | **Gap** — Cycle metrics + throttled SSE progress ([`metrics.go`](pkgs/agents/harness/metrics.go), [`sse.go`](internal/taskapi/agentworker/sse.go)); no `trace_id` spanning harness phases and runner progress. |
| Notifier back-pressure | **Risk** — SSE adapters call `pub.Publish` synchronously on the harness goroutine ([`internal/taskapi/agentworker/sse.go`](internal/taskapi/agentworker/sse.go)). Hub slow path can block the run loop ([harness.md § Limitations](docs/domain/harness.md)). |
| Verdict mirror failures | **Silent** — `UpsertVerifyReports` / criteria mirror failures log `Warn` and verify continues ([`checks.go`](pkgs/agents/harness/internal/verify/checks.go), [harness.md](docs/domain/harness.md)). |
| Shared `repo_root` blast radius | **Single worker** — One in-process consumer ([architecture.md](docs/architecture.md)); concurrent harness runs on same repo are not a V1 target, but host `repo_root` is not isolated per cycle. |

---

## Ranked improvements

**Progress:** 2 / 16 actionable items shipped (P0–P2). P3 items are intentional deferrals — check only when explicitly pursued.

### P0 — Highest ROI

#### - [x] 1. Verify-only retry within the same cycle

| | |
| --- | --- |
| **Status** | Shipped — [ADR-0028](docs/adr/ADR-0028-in-cycle-verify-only-retry.md), commit `c01e3fe` |
| **ROI_score** | ~4.1 |
| **Problem** | Any verify failure triggers a full execute phase (`runner.Run`), even when execute succeeded, commits ingested, and `criteria-report.json` is still valid. Wastes tokens and time ([HARNESS_LANDSCAPE.md](HARNESS_LANDSCAPE.md) PEV gap; validated in [`cycle_loop.go`](pkgs/agents/harness/cycle_loop.go)). |
| **Proposed change** | Extend orchestration: on retryable verify failure, if execute artifacts + commit ingest are still valid, set `skipFirstExecute` (same branch as `resumeEntryAfterExecuteSuccess`) and only re-run verify. Preserve locked `previouslyPassed`. |
| **Effort** | M — orchestration FSM + `cycle_loop` + tests ([ADR-0018](docs/adr/ADR-0018-harness-orchestration-fsm.md)) |
| **ETCSLV** | E, V |
| **Evidence** | Landscape matrix E/V; [`runCycleLoop`](pkgs/agents/harness/cycle_loop.go); verify-only precedent in [`retry_run.go`](pkgs/agents/harness/retry_run.go) |
| **Dependencies / risks** | Must not skip execute when criteria self-report stale or git state changed; integrity rules unchanged |
| **Success signal** | ↓ execute phases per cycle on verify-retry failures; `ObserveVerifyRetries` unchanged but median `RecordRun` duration drops for multi-criterion tasks |

#### - [x] 2. Harness trace correlation (cycle → phase → runner)

| | |
| --- | --- |
| **Status** | Shipped — [ADR-0030](docs/adr/ADR-0030-attempt-phase-correlation.md), feature commit `e8d5e83`. UI phase filter + `?phase=N` in cycle 3 (`d7e4900`). |
| **ROI_score** | ~3.9 |
| **Problem** | Operators correlate cycles via task id + cycle id in logs, but cannot stitch harness phases to Cursor stream events without manual grep ([HARNESS_LANDSCAPE.md](HARNESS_LANDSCAPE.md) observability row). |
| **Proposed change** | Propagate a stable `run_correlation_id` (or OTel trace) from `StartCycle` through phase rows, slog fields, SSE payloads, and runner progress events. |
| **Effort** | M — harness + `agentworker` adapters + optional `logctx` |
| **ETCSLV** | L (instrumentation) |
| **Evidence** | [`metrics.go`](pkgs/agents/harness/metrics.go); landscape observability gap |
| **Dependencies / risks** | Label cardinality if per-tool spans exported to Prometheus — prefer trace backend for fine grain |
| **Success signal** | One query links phase ledger row → harness logs → progress SSE for a stuck run |

#### - [ ] 3. Surface or gate verdict DB mirror failures

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~3.8 |
| **Problem** | `UpsertVerifyReports` / criteria mirror failures are logged and verify continues ([`checks.go`](pkgs/agents/harness/internal/verify/checks.go)); UI and support tooling read DB mirrors ([harness.md § Limitations](docs/domain/harness.md)). |
| **Proposed change** | (a) Operator-visible: SSE `agent_run_progress` or cycle `details_json` flag `mirror_degraded`; and/or (b) fail verify retryably when mirror write fails after parse success. |
| **Effort** | S |
| **ETCSLV** | S, V |
| **Evidence** | harness limitation table; [`persist.go`](pkgs/agents/harness/internal/verify/persist.go) |
| **Dependencies / risks** | Failing verify on mirror error may increase retries — prefer visible degradation first |
| **Success signal** | Zero silent mirror failures in production logs without matching UI/SSE signal |

#### - [ ] 4. Non-blocking notifier contract + hub back-pressure guard

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~3.7 |
| **Problem** | Harness assumes notifiers do not block; SSE adapters publish synchronously on the worker goroutine ([`sse.go`](internal/taskapi/agentworker/sse.go)). |
| **Proposed change** | Document contract in `CycleChangeNotifier` / `ProgressNotifier`; enqueue to hub via non-blocking send or timeout; count dropped notifications in metrics. |
| **Effort** | S |
| **ETCSLV** | L |
| **Evidence** | harness.md limitation “Notifier blocking” |
| **Dependencies / risks** | SSE hub may need drop policy alignment ([sse-hub.md](docs/domain/sse-hub.md)) |
| **Success signal** | Harness run duration p99 unaffected by synthetic slow subscriber test |

#### - [ ] 5. Harden T0/T2 resume parity for verify retry budget

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~3.6 |
| **Problem** | `verifyAttempt` is derived from DB `attempt_seq` on resume ([`checkpoint.go`](pkgs/agents/harness/internal/resume/checkpoint.go)); cross-cycle resume resets to 0 by design. Edge mismatches could over- or under-count retries vs `verify_max_retries`. |
| **Proposed change** | Explicit `verify_retry_count` in phase `details_json` or cycle meta at each verify boundary; resume reads that field; table-driven tests for interrupt mid-retry. |
| **Effort** | S–M |
| **ETCSLV** | S |
| **Evidence** | ADR-0006; landscape T0 row; [`loadVerifyCheckpointData`](pkgs/agents/harness/internal/resume/checkpoint.go) |
| **Dependencies / risks** | Store write on verify complete — must stay inside existing phase transaction patterns |
| **Success signal** | Resume integration tests cover retry budget after `process_restart` with no drift |

---

### P1 — High ROI

#### - [ ] 6. Unified iteration / cost meter in Settings or task detail

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~3.4 |
| **Problem** | No single operator view of verify retries, run duration, and model labels per cycle ([HARNESS_LANDSCAPE.md](HARNESS_LANDSCAPE.md) budgets row). Data exists in metrics + phase ledger. |
| **Proposed change** | Expose per-cycle: execute phase count, verify attempts, wall time, runner/model — via existing APIs or SSE-enriched cycle payload. |
| **Effort** | L — web + handler read policy |
| **ETCSLV** | — (product surface) |
| **Evidence** | [`metrics.go`](pkgs/agents/harness/metrics.go); landscape budgets |
| **Success signal** | Operators see retry count before clicking “Resume from failure” |

#### - [ ] 7. Context budget and compaction for project context injection

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~3.3 |
| **Problem** | Project context snapshots inject full blocks via [`internal/prompt`](pkgs/agents/harness/internal/prompt/); no compaction ([HARNESS_LANDSCAPE.md](HARNESS_LANDSCAPE.md) C row). |
| **Proposed change** | `app_settings` max context chars; summarize or tier snapshots before execute/verify compose; warn in SPA when truncated. |
| **Effort** | M |
| **ETCSLV** | C |
| **Evidence** | [project-context.md](docs/domain/project-context.md); landscape C gap |
| **Success signal** | Large projects complete without runner context overflow failures |

#### - [ ] 8. Optional per-cycle git worktree (light sandbox)

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~3.1 |
| **Problem** | All cycles mutate shared `repo_root` on host ([HARNESS_LANDSCAPE.md](HARNESS_LANDSCAPE.md) sandbox row). Single worker limits concurrency today, but operator + manual edits share tree. |
| **Proposed change** | Settings flag: create worktree per cycle, run harness in worktree path, merge or discard on terminate. |
| **Effort** | L — harness git svc + settings + cleanup |
| **ETCSLV** | T (isolation), S |
| **Evidence** | Landscape sandbox; [workspace-repo.md](docs/domain/workspace-repo.md) |
| **Dependencies / risks** | Git worktree cleanup on crash; Start over / fresh retry interaction |
| **Success signal** | Failed cycle leaves main worktree clean without operator git reset |

#### - [ ] 9. ETCSLV completeness scorecard (deployment audit)

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~3.0 |
| **Problem** | No explicit checklist of which ETCSLV components T2A implements vs delegates ([NanoHarness](https://github.com/HabitGraylight/NanoHarness) pattern). |
| **Proposed change** | Script or doc table auto-filled from config (runners, verify enabled, repo_root, metrics endpoint) — extends this backlog on each release. |
| **Effort** | S |
| **ETCSLV** | — (meta) |
| **Evidence** | HARNESS_LANDSCAPE ETCSLV section |
| **Success signal** | Onboarding answers “what harness do we have?” in one screen |

#### - [ ] 10. Second production runner (`claude-code`)

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~2.9 |
| **Problem** | Portability tied to Cursor V1 ([runner-adapters.md](docs/domain/runner-adapters.md)); scaffold only. |
| **Proposed change** | Complete `pkgs/agents/runner/claudecode` adapter using `adapterkit`; probe + Settings parity with cursor. |
| **Effort** | M |
| **ETCSLV** | E (delegated), portability |
| **Evidence** | Landscape portability row |
| **Success signal** | Production task completes on non-Cursor runner with same harness path |

---

### P2 — Medium ROI (near-term)

#### - [ ] 11. `Harness` interface / strategy registry

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~2.7 |
| **Problem** | Single concrete `Harness` ([ADR-0005](docs/adr/ADR-0005-extract-agent-harness.md)); tests and alt strategies wire the full type. |
| **Proposed change** | Narrow interface for worker (`Run`, `Resume`, `RunWithRetry`, `CancelCurrentRun`); keep concrete type as default impl. |
| **Effort** | M |
| **Success signal** | Worker tests inject fake harness without store |

#### - [ ] 12. Persist composed prompt reference in cycle meta

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~2.6 |
| **Problem** | Only `InitialPrompt` hashed in `meta_json` ([harness.md § Limitations](docs/domain/harness.md)); debugging resume/retry prompt drift is hard. |
| **Proposed change** | Store SHA-256 of final composed execute prompt per execute phase in `details_json` or snapshot table. |
| **Effort** | S |
| **Success signal** | Support compares prompt hash across retries without log scraping |

#### - [ ] 13. General harness hook framework (Pre/Post phase)

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~2.5 |
| **Problem** | Hooks partial in verify + `adapterkit`; no ETCSLV L layer at harness ([HARNESS_LANDSCAPE.md](HARNESS_LANDSCAPE.md)). |
| **Proposed change** | Registered hooks at `StartPhase` / `CompletePhase` boundaries for policy injection (lint, custom checks). |
| **Effort** | L |
| **Success signal** | Custom org policy without forking harness |

#### - [ ] 14. MCP bridge in runner (not harness-owned)

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~2.4 |
| **Problem** | No MCP at harness level; tools delegated to CLI ([landscape T row](HARNESS_LANDSCAPE.md)). |
| **Proposed change** | Optional MCP client inside runner adapter for selected tools; harness unchanged. |
| **Effort** | M–L |
| **Success signal** | Task uses MCP tool via configured runner |

#### - [ ] 15. Persisted plan artifact (PEV planning row)

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~2.3 |
| **Problem** | No durable plan object; task prompt + criteria only ([landscape planning row](HARNESS_LANDSCAPE.md)). |
| **Proposed change** | Optional `task_plans` or cycle meta plan JSON before first execute; verify checks plan alignment. |
| **Effort** | L |
| **Success signal** | UI shows plan vs outcome diff |

#### - [ ] 16. Automatic git rollback on verify failure mid-cycle

| | |
| --- | --- |
| **Status** | Not started |
| **ROI_score** | ~2.2 |
| **Problem** | Operator Start over resets git; in-cycle verify fail leaves dirty tree until manual action ([landscape rollback row](HARNESS_LANDSCAPE.md)). |
| **Proposed change** | Optional setting: `git reset --hard` to cycle base SHA when verify fails terminally (not on retry). |
| **Effort** | M |
| **Risks** | Destructive; must align with commit observe/admit and operator intent |
| **Success signal** | Failed tasks leave repo at known SHA without Start over |

---

### P3 — Low ROI / strategic defer (V1)

These are **intentionally not near-term** per [HARNESS_LANDSCAPE.md](HARNESS_LANDSCAPE.md) and [architecture.md](docs/architecture.md). Ranked lowest leverage for current product shape.

| Item | Status | Why defer |
| --- | --- | --- |
| - [ ] Own inner tool loop / ACI (replace Cursor) | Deferred (V1) | XL effort; contradicts V1 delegation; Cursor owns E+T inner loop |
| - [ ] Multi-agent coordinator + parallel implementors | Deferred (V1) | V1 single worker; queue and admission redesign required |
| - [ ] PEVR DAG replan | Deferred (V1) | XL orchestration; T2A task model is flat prompt + criteria, not sub-task graphs |
| - [ ] Mid-CLI session resume | Deferred (V1) | Blocked on runner statefulness ([ADR-0006](docs/adr/ADR-0006-phase-boundary-resume.md)) |
| - [ ] Multi-replica workers | Deferred (V1) | Documented not supported; races on cycles and queue |
| - [ ] Per-task Firecracker / container sandbox | Deferred (V1) | XL ops; worktree (item 8) is lighter ROI first |
| - [ ] Harness-level approval gates on tool calls | Deferred (V1) | Delegated to Cursor + OS; UI scope explosion |
| - [ ] Full OpenHarness-style skills/plugins/MCP stack at harness | Deferred (V1) | Duplicates runner/product layers |

---

## Quick wins (S band, one PR each)

- [ ] **Surface verdict mirror degradation** (item 3, visible-only path)
- [ ] **Notifier non-blocking guard** (item 4)
- [ ] **Composed prompt hash in phase details** (item 12)
- [ ] **ETCSLV scorecard script** (item 9)

---

## What we should not build (anti-goals)

- **Duplicate Cursor’s inner ReAct loop in Go** — invest in outer verify/durability instead ([HARNESS_LANDSCAPE.md](HARNESS_LANDSCAPE.md) layer split).
- **Multi-agent fleet without queue redesign** — breaks single-consumer ack ordering ([agent-queue.md](docs/domain/agent-queue.md)).
- **Partial completion ledger writes on failed cycles** — violates atomic completion ([ADR-0007](docs/adr/ADR-0007-parent-completion-via-criteria.md)).
- **Harness logic in worker admission** — keep boundary from [ADR-0005](docs/adr/ADR-0005-extract-agent-harness.md).

---

## See also

| Doc | Content |
| --- | --- |
| [HARNESS_LANDSCAPE.md](HARNESS_LANDSCAPE.md) | Industry patterns vs T2A |
| [docs/domain/harness.md](docs/domain/harness.md) | Operational harness behavior |
| [docs/domain/verify-agent.md](docs/domain/verify-agent.md) | Verify pipeline |
| [docs/domain/runner-adapters.md](docs/domain/runner-adapters.md) | Inner loop delegation |
| [pkgs/agents/harness/README.md](pkgs/agents/harness/README.md) | File map |
| [CODEBASE_TOUR.md](CODEBASE_TOUR.md) | Repo orientation |

### ADRs most relevant to this backlog

ADR-0005, ADR-0006, ADR-0015, ADR-0018, ADR-0021, ADR-0027
