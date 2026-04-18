# `pkgs/tasks/store`

GORM-backed persistence for tasks, audit events, checklists, drafts, draft evaluations, cycles/phases, the ready-task queue, dev-mirror, and DB health probes. **Product docs:** [docs/PERSISTENCE.md](../../docs/PERSISTENCE.md), [docs/EXTENSIBILITY.md](../../docs/EXTENSIBILITY.md). **Ready-queue hooks:** [docs/AGENT-QUEUE.md](../../docs/AGENT-QUEUE.md). API contracts: [docs/API-HTTP.md](../../docs/API-HTTP.md).

Package overview and conventions: `go doc -all .` (starts in [doc.go](./doc.go)).

## Architecture

The public package is a **facade**: every `*Store` method in a `facade_*.go` file delegates to a per-domain package under [`internal/`](./internal). Public types (`CreateTaskInput`, `TaskNode`, `DraftTaskEvaluation`, …) are Go type aliases over the internal definitions, so external callers stay unchanged across reshuffles. Cross-domain transactions are composed by calling the exported `…InTx` helpers from sibling internal packages inside one `*gorm.DB.Transaction(...)`.

Ready-task notifications (`(*Store).notifyReadyTask`) are intentionally only fired by the facade; subpackages return the updated task plus the previous status so the facade can decide whether to notify exactly once. This keeps `internal/notify` out of the per-domain dependency graphs.

## Where code lives

| Concern | Facade file | Tests | Internal package | Notes |
|---|---|---|---|---|
| Wiring | `store.go` | (in `facade_tasks_test.go`) | `internal/notify` | `Store`, `NewStore`, `ReadyTaskNotifier`, `SetReadyTaskNotifier`, `notifyReadyTask`. |
| Tasks — CRUD, lists & trees | `facade_tasks.go` | `facade_tasks_test.go` | `internal/tasks` | `Get`, `Create`, `Update`, `Delete`, `List` / `ListFlat`, `ListRootForest{,After}`, `GetTaskTree`. `CreateTaskInput`, `UpdateTaskInput`, `ParentFieldPatch`, `TaskNode`, `MaxTaskTreeDepth` aliased here. Tests also cover the ready-task notifier wiring and the operation-duration histogram. |
| Stats | `facade_stats.go` | — | `internal/stats` | `GlobalTaskStats`. |
| Checklist | `facade_checklist.go` | `facade_checklist_test.go` | `internal/checklist` | List / add / update / delete / set-done. Exports `ValidateCanMarkDoneInTx` and `DeleteOwnedItemsInTx` for sibling subpackages. |
| Cycles & phases | `facade_cycles.go` | `facade_cycles_test.go` | `internal/cycles` | `StartCycle`, `TerminateCycle`, `GetCycle`, `ListCyclesForTask`, `StartPhase`, `CompletePhase`, `ListPhasesForCycle`. Dual-writes mirror events into `task_events`; tests pin the audit contract, `meta_json`/`details_json` normalization, and rollback on mirror failure. |
| Task events | `facade_events.go` | `facade_events_test.go` | `internal/events` | `AppendTaskEvent`, list / count, keyset page + `ApprovalPending`, `GetTaskEvent`, `AppendTaskEventResponseMessage`, `ThreadEntriesForDisplay`. |
| Draft evaluations | `facade_eval.go` | `facade_eval_test.go` | `internal/eval` | `EvaluateDraftTask`, `ListDraftEvaluations`. Exports `AttachDraftEvaluationsInTx` for `Create`-from-draft. |
| Task drafts | `facade_drafts.go` | `facade_drafts_test.go` | `internal/drafts` | `SaveDraft`, `ListDrafts`, `GetDraft`, `DeleteDraft`. Exports `DeleteByIDInTx` for `Create`-from-draft. Tests also pin the `payload_json` normalization invariant. |
| Agent ready queue | `facade_ready.go` | `facade_ready_test.go` | `internal/ready` | `ListReadyTaskQueueCandidates`, `ListReadyTasksUserCreated`, `DefaultReadyTimeout`. |
| Health | `facade_health.go` | `facade_health_test.go` | `internal/health` | `Ping`, `Ready`. |
| Dev simulation | `facade_devmirror.go` | `facade_devmirror_test.go` | `internal/devmirror` | `ApplyDevTaskRowMirror`, `ListDevsimTasks`. |
| Shared kernel | — | — | `internal/kernel` | `Op*` Prometheus labels, `DeferLatency`, `AppendEvent`, `NextEventSeq`, `EventPairJSON`, `Valid*` validators, `ValidateActor`, `LoadTask`, `NormalizeJSONObject`. The only place `promauto` registers metrics. |

Each `facade_<domain>_test.go` exercises the public API end-to-end against the SQLite test harness, mirroring its production sibling 1:1. Strictly internal helpers (e.g. `isDuplicatePrimaryKey`) keep their white-box test in the corresponding `internal/<domain>/` package.

When adding a **new** store method, extend the table above in the same PR and place the implementation in the matching `internal/<domain>/` package, with a thin `(*Store)` delegation in `facade_<domain>.go` and tests in `facade_<domain>_test.go`.
