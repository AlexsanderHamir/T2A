# `pkgs/tasks/store`

GORM-backed persistence for tasks, audit events, checklists, drafts, cycles/phases, the ready-task queue, dev-mirror, and DB health probes. **Architecture and dual-write invariant:** [docs/architecture.md](../../docs/architecture.md), [docs/data-model.md](../../docs/data-model.md). **Behavioral deep-dives:** [docs/domain/persistence.md](../../docs/domain/persistence.md), [docs/domain/task-events.md](../../docs/domain/task-events.md). **How to extend:** [docs/contributing.md](../../docs/contributing.md). API contracts: [docs/api.md](../../docs/api.md).

Package overview and conventions: `go doc -all .` (starts in [doc.go](./doc.go)).

## Architecture

The public package is a **facade**: every `*Store` method in a `facade_*.go` file delegates to a per-domain package under [`internal/`](./internal). Public types (`CreateTaskInput`, `TaskNode`, …) are Go type aliases over the internal definitions, so external callers stay unchanged across reshuffles. Cross-domain transactions are composed by calling the exported `…InTx` helpers from sibling internal packages inside one `*gorm.DB.Transaction(...)`.

Ready-task notifications (`(*Store).notifyReadyTask`) are intentionally only fired by the facade; subpackages return the updated task plus the previous status so the facade can decide whether to notify exactly once. This keeps `internal/notify` out of the per-domain dependency graphs.

## Where code lives

| Concern | Facade file | Tests | Internal package | Notes |
|---|---|---|---|---|
| Wiring | `store.go` | (in `facade_tasks_test.go`) | `internal/notify` | `Store`, `NewStore`, `ReadyTaskNotifier`, `SetReadyTaskNotifier`, `notifyReadyTask`. |
| Projects & project context | `facade_projects.go` | `facade_projects_test.go` | `internal/projects` | Project CRUD, context item CRUD, and immutable task context snapshots used by agent runs. |
| Tasks — CRUD, lists & trees | `facade_tasks.go` | `facade_tasks_test.go` | `internal/tasks` | `Get`, `Create`, `Update`, `Delete`, `List` / `ListFlat`, `ListRootForest{,After}`, `GetTaskTree`. Readiness delegates to [`pkgs/tasks/scheduling/`](../scheduling/). `CreateTaskInput`, `UpdateTaskInput`, `ParentFieldPatch`, `TaskNode`, `MaxTaskTreeDepth` aliased here. Tests also cover the ready-task notifier wiring and the operation-duration histogram. |
| Stats | `facade_stats.go` | — | `internal/stats` | `GlobalTaskStats`. |
| Checklist | `facade_checklist.go` | `facade_checklist_test.go` | `internal/checklist` | List / add / update / delete / set-done. Exports `ValidateCanMarkDoneInTx` and `DeleteOwnedItemsInTx` for sibling subpackages. |
| Cycles & phases | `facade_cycles.go` | `facade_cycles_test.go` | `internal/cycles` | `StartCycle`, `TerminateCycle`, `GetCycle`, `ListCyclesForTask`, `StartPhase`, `CompletePhase`, `ListPhasesForCycle`, `LastSessionID` (ADR-0031 Cursor resume). Dual-writes mirror events into `task_events`; tests pin the audit contract, `meta_json`/`details_json` normalization, and rollback on mirror failure. |
| Cycle commits | `facade_commits.go` | `facade_commits_test.go` | `internal/commits` | `UpsertCycleCommits`, `ListCommitsForCycle`, `ListCommitsForTask`. Worker-indexed SHAs after execute ingest — [docs/domain/cycle-commits.md](../../docs/domain/cycle-commits.md). |
| Task events | `facade_events.go` | `facade_events_test.go` | `internal/events` | `AppendTaskEvent`, list / count, keyset page + `ApprovalPending`, `GetTaskEvent`, `AppendTaskEventResponseMessage`, `ThreadEntriesForDisplay`. |
| Task drafts | `facade_drafts.go` | `facade_drafts_test.go` | `internal/drafts` | `SaveDraft`, `ListDrafts`, `GetDraft`, `DeleteDraft`. Exports `DeleteByIDInTx` for `Create`-from-draft. Tests also pin the `payload_json` normalization invariant. |
| Agent ready queue | `facade_ready.go` | `facade_ready_test.go`, `scheduling_parity_test.go` | `internal/ready` | `ListReadyTaskQueueCandidates`, `ListReadyTasksUserCreated`, `DefaultReadyTimeout`. SQL dequeuable predicates mirror [`scheduling/`](../scheduling/). |
| Health | `facade_health.go` | `facade_health_test.go` | `internal/health` | `Ping`, `Ready`. |
| Dev simulation | `facade_devmirror.go` | `facade_devmirror_test.go` | `internal/devmirror` | `ApplyDevTaskRowMirror`, `ListDevsimTasks`. |
| Shared kernel | — | — | `internal/kernel` | `Op*` Prometheus labels, `DeferLatency`, `AppendEvent`, `NextEventSeq`, `EventPairJSON`, `Valid*` validators, `ValidateActor`, `LoadTask`, `NormalizeJSONObject`. The only place `promauto` registers metrics. |

Each `facade_<domain>_test.go` exercises the public API end-to-end against the SQLite test harness, mirroring its production sibling 1:1. Strictly internal helpers (e.g. `isDuplicatePrimaryKey`) keep their white-box test in the corresponding `internal/<domain>/` package.

When adding a **new** store method, extend the table above in the same PR and place the implementation in the matching `internal/<domain>/` package, with a thin `(*Store)` delegation in `facade_<domain>.go` and tests in `facade_<domain>_test.go`.
