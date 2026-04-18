// Package tasks owns task row CRUD and the parent-child tree shape
// returned by the public store facade. It is the orchestration tier
// for the task lifecycle and composes cross-domain transactions
// (drafts cleanup, draft-evaluation linking, checklist guards) by
// calling the exported InTx helpers from sibling internal/* packages
// rather than duplicating their logic.
//
// Notification side effects are intentionally NOT performed here:
// every method that may move a task into StatusReady returns the
// resulting *domain.Task (and, for Update, the previous status) so
// the public store facade can decide whether to fire the ready-task
// notifier exactly once. Keeping the notifier wiring at the facade
// prevents this subpackage from depending on internal/notify (which
// only the *Store struct holds) and keeps the goroutine-fanout
// concern out of the transactional path.
//
// Public re-exports kept by the facade:
//   - CreateInput      <- store.CreateTaskInput
//   - UpdateInput      <- store.UpdateTaskInput
//   - ParentFieldPatch <- store.ParentFieldPatch
//   - Node             <- store.TaskNode
//   - MaxTreeDepth     <- store.MaxTaskTreeDepth
package tasks
