// Package eval owns the per-draft scoring rubric and the
// task_draft_evaluations persistence (table column conventions in
// docs/DRAFTS.md).
//
// # Responsibilities
//
//   - EvaluateDraftTask runs the deterministic scoring rubric over a
//     draft snapshot and persists one task_draft_evaluations row per
//     call. The same input may yield slightly different rows because
//     the suggestion-pool sampling uses a wall-clock-seeded *rand.Rand
//     (the score itself is deterministic).
//   - ListDraftEvaluations returns the most-recent rows for a draft id,
//     newest first, capped at 200.
//   - AttachDraftEvaluationsInTx is the cross-domain hook the
//     tasks-CRUD subpackage calls inside its Create transaction so
//     evaluations recorded against a draft id get linked to the new
//     task row when the user finally creates it.
//
// The package is internal to pkgs/tasks/store; external callers go
// through the store facade methods (EvaluateDraftTask,
// ListDraftEvaluations).
package eval
