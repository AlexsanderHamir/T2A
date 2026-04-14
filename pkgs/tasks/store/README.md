# `pkgs/tasks/store`

GORM-backed persistence for tasks, audit events, checklists, and drafts. **Product docs:** [docs/PERSISTENCE.md](../../docs/PERSISTENCE.md), [docs/EXTENSIBILITY.md](../../docs/EXTENSIBILITY.md). **Ready-queue hooks:** [docs/AGENT-QUEUE.md](../../docs/AGENT-QUEUE.md). API contracts: [docs/API-HTTP.md](../../docs/API-HTTP.md).

Package overview and conventions: `go doc -all .` (starts in [doc.go](./doc.go)).

## Where code lives (`store_*.go`)

| Concern | Files | Notes |
|--------|--------|--------|
| **Types / wiring** | `store.go` | `Store`, `NewStore`, `CreateTaskInput`, `UpdateTaskInput`, `ParentFieldPatch`. |
| **Ready-task notifier** | `store_ready_notify.go` | `SetReadyTaskNotifier`, `notifyReadyTask` (called after commits that surface ready work). |
| **CRUD** | `store_crud_create.go`, `store_crud.go` | `Create`; `Get`, `Update`, `Delete`. |
| **Lists & trees** | `store_tree.go`, `task_node.go` | Root list (offset / keyset), flat list, full task tree; `TaskNode` JSON shape. |
| **Stats** | `store_stats.go` | Global task counters. |
| **Draft evaluations** | `store_evaluation.go`, `store_evaluation_score.go` | `EvaluateDraftTask`, list evaluations; scoring helpers (no `Store` methods in score file). |
| **Task drafts (saved forms)** | `store_draft.go` | Save / list / get / delete named drafts. |
| **Health** | `store_health.go` | `Ping`, `Ready` (readiness / DB probe). |
| **Task events — append** | `store_append_event.go` | `AppendTaskEvent` (synthetic + real audit rows). |
| **Task events — read** | `store_task_events_query.go`, `store_task_events_page.go`, `store_get_event.go` | Full list, counts, keyset page + `ApprovalPending`, single row by `seq`. |
| **Task events — user thread** | `store_event_user_response.go` | `AppendTaskEventResponseMessage` (thread append, locking on Postgres). |
| **Checklist — read** | `store_checklist.go` | Definition source resolution, list items for a subject task. |
| **Checklist — write** | `store_checklist_mutations.go`, `store_checklist_validate_tx.go` | Add / update text / set done / delete; transactional validation helpers. |
| **Validation helpers** | `store_validate.go` | Package-private status/priority/type/actor checks used by CRUD paths. |
| **Agent queue ordering** | `store_list_ready.go`, `store_list_ready_user.go` | `ListReadyTaskQueueCandidates` (reconcile FIFO); `ListReadyTasksUserCreated` (user-scoped list). |
| **Devsim** | `store_dev_mirror.go`, `store_devsim_list.go` | `ApplyDevTaskRowMirror`; `ListDevsimTasks` for synthetic SSE / lifecycle helpers. |

Tests mirror the same names (`store_*_test.go`, `store_test.go`, `is_duplicate_task_pk_test.go`).

When adding a **new** store method, extend the table above in the same PR so the map stays truthful.
