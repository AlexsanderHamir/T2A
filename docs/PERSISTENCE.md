# Persistence and audit (`store`)

GORM-backed tasks and append-only **`task_events`**. HTTP JSON shapes: [API-HTTP.md](./API-HTTP.md). Env and migrate timeouts: [RUNTIME-ENV.md](./RUNTIME-ENV.md). System limitations (multi-replica SSE, etc.): [DESIGN.md](./DESIGN.md#limitations).

Tasks: CRUD via GORM; ordering and list limits match the store package doc.

REST shape vs audit: the JSON task resource has no `created_at` / `updated_at` fields. Timestamps live only on `task_events` (`At` in UTC when the event is written). Operators needing “when did this task last change?” should query audit rows (or add a future API field).

Concurrency: `Update` runs in a transaction and loads the task row with a row lock (`SELECT … FOR UPDATE` via GORM). Concurrent patches to the same task serialize; there is no ETag / version on the task row—last successful transaction wins.

Audit: append-only `task_events` for typed changes. Event type strings are `domain.EventType` values (e.g. `task_created`, `status_changed`, `prompt_appended`; title edits are stored as `message_added` in code). Used for history and debugging; events are not replayed into the SSE hub.

Schema: `postgres.Migrate` runs GORM `AutoMigrate` for `domain.Task`, `domain.TaskEvent`, checklist tables (`domain.TaskChecklistItem`, `domain.TaskChecklistCompletion`), and draft evaluations (`domain.TaskDraftEvaluation`). There are no checked-in versioned SQL migrations or down migrations.

## Related

- `pkgs/tasks/store/README.md` — which `store_*.go` file owns which concern.
- `.cursor/rules/BACKEND_AUTOMATION/persistence-gorm.mdc` — GORM models / SQLite test helpers.
- `pkgs/tasks/store/doc.go` — `go doc` next to code.
