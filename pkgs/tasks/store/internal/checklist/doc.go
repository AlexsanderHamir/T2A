// Package checklist owns the per-task acceptance-criteria persistence
// (table task_checklist_items) and the per-subject completion ledger
// (table task_checklist_completions).
//
// # Responsibilities
//
//   - Definition CRUD: Add / Delete / UpdateText, all gated by
//     domain.Task.ChecklistInherit (an inherit-true task is a
//     read-through view of its definition source and may not own
//     definitions itself).
//   - Completion writes: SetDone, restricted to
//     domain.ActorAgent — the human user records criteria but does
//     not toggle the done flag (per docs/CHECKLIST.md).
//   - Read-side resolution: List / DefinitionSourceTaskID walk the
//     ParentID chain through ChecklistInherit-true ancestors so the
//     subject task always shows the inherited criteria.
//
// # Dual-write invariant
//
// Every mutation appends a matching task_events audit row in the same
// SQL transaction (kernel.NextEventSeq + kernel.AppendEvent). If the
// mirror append fails, the checklist write is rolled back. The mirror
// types are domain.EventChecklistItemAdded /
// EventChecklistItemRemoved / EventChecklistItemUpdated /
// EventChecklistItemToggled.
//
// # Cross-domain helpers
//
// ValidateCanMarkDoneInTx and DeleteOwnedItemsInTx are exported so
// the tasks-CRUD subpackage can compose its
// "task → status=done" and "task → delete" transactions without
// reaching back into private helpers; the contract is documented on
// the function comments.
//
// The package is internal to pkgs/tasks/store; external callers go
// through the store facade methods.
package checklist
