# `pkgs/tasks/store`

GORM-backed persistence for tasks, audit events, checklists, drafts, draft evaluations, cycles/phases, the ready-task queue, dev-mirror, and DB health probes. **Product docs:** [docs/PERSISTENCE.md](../../docs/PERSISTENCE.md), [docs/EXTENSIBILITY.md](../../docs/EXTENSIBILITY.md). **Ready-queue hooks:** [docs/AGENT-QUEUE.md](../../docs/AGENT-QUEUE.md). API contracts: [docs/API-HTTP.md](../../docs/API-HTTP.md).

Package overview and conventions: `go doc -all .` (starts in [doc.go](./doc.go)).

## Architecture

The public package is a **facade**: every `*Store` method in a `facade_*.go` file delegates to a per-domain package under [`internal/`](./internal). Public types (`CreateTaskInput`, `TaskNode`, `DraftTaskEvaluation`, …) are Go type aliases over the internal definitions, so external callers stay unchanged across reshuffles. Cross-domain transactions are composed by calling the exported `…InTx` helpers from sibling internal packages inside one `*gorm.DB.Transaction(...)`.

Ready-task notifications (`(*Store).notifyReadyTask`) are intentionally only fired by the facade; subpackages return the updated task plus the previous status so the facade can decide whether to notify exactly once. This keeps `internal/notify` out of the per-domain dependency graphs.

## Where code lives

| Concern | Facade file | Internal package | Notes |
|---|---|---|---|
| Wiring | `store.go`, `store_ready_notify.go` | `internal/notify` | `Store`, `NewStore`, `SetReadyTaskNotifier`, `notifyReadyTask`. |
| Tasks — CRUD | `facade_tasks.go` | `internal/tasks` | `Get`, `Create`, `Update`, `Delete`. `CreateTaskInput` / `UpdateTaskInput` / `ParentFieldPatch` aliased here. |
| Tasks — lists & trees | `facade_tree.go` | `internal/tasks` | `List` / `ListFlat`, `ListRootForest{,After}`, `GetTaskTree`. `TaskNode` and `MaxTaskTreeDepth` aliased. |
| Stats | `facade_stats.go` | `internal/stats` | `GlobalTaskStats`. |
| Checklist | `facade_checklist.go` | `internal/checklist` | List / add / update / delete / set-done. Exports `ValidateCanMarkDoneInTx` and `DeleteOwnedItemsInTx` for sibling subpackages. |
| Cycles & phases | `facade_cycles.go` | `internal/cycles` | `StartCycle`, `TerminateCycle`, `GetCycle`, `ListCyclesForTask`, `StartPhase`, `CompletePhase`, `ListPhasesForCycle`. Dual-writes mirror events into `task_events`. |
| Task events | `facade_events.go` | `internal/events` | `AppendTaskEvent`, list / count, keyset page + `ApprovalPending`, `GetTaskEvent`, `AppendTaskEventResponseMessage`, `ThreadEntriesForDisplay`. |
| Draft evaluations | `facade_eval.go` | `internal/eval` | `EvaluateDraftTask`, `ListDraftEvaluations`. Exports `AttachDraftEvaluationsInTx` for `Create`-from-draft. |
| Task drafts | `facade_drafts.go` | `internal/drafts` | `SaveDraft`, `ListDrafts`, `GetDraft`, `DeleteDraft`. Exports `DeleteByIDInTx` for `Create`-from-draft. |
| Agent ready queue | `facade_ready.go` | `internal/ready` | `ListReadyTaskQueueCandidates`, `ListReadyTasksUserCreated`, `DefaultReadyTimeout`. |
| Health | `facade_health.go` | `internal/health` | `Ping`, `Ready`. |
| Dev simulation | `facade_devmirror.go` | `internal/devmirror` | `ApplyDevTaskRowMirror`, `ListDevsimTasks`. |
| Shared kernel | — | `internal/kernel` | `Op*` Prometheus labels, `DeferLatency`, `AppendEvent`, `NextEventSeq`, `EventPairJSON`, `Valid*` validators, `ValidateActor`, `LoadTask`, `NormalizeJSONObject`. The only place `promauto` registers metrics. |

Tests live alongside the public facade (`store_*_test.go`, `store_test.go`) and exercise the API end-to-end against the SQLite test harness. Strictly internal helpers (e.g. `isDuplicatePrimaryKey`) keep their white-box test in the corresponding `internal/<domain>/` package.

When adding a **new** store method, extend the table above in the same PR and place the implementation in the matching `internal/<domain>/` package, with a thin `(*Store)` delegation in `facade_<domain>.go`.
