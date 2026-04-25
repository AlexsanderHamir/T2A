// Package store is the GORM-backed persistence facade for domain.Task,
// domain.TaskEvent, checklists, drafts, draft evaluations, cycles and
// phases, the ready-task queue, dev-mirror, and DB health probes.
//
// Architecture (post-split): the public surface area lives in
// `facade_*.go` files and consists of type aliases plus thin (*Store)
// methods that delegate into one of the per-domain subpackages under
// pkgs/tasks/store/internal/<domain>/. Subpackages own their own
// validation, transactional logic, and dual-write invariants; the
// facade only wires the *gorm.DB and fans out side-effects (the
// ready-task notifier) that are scoped to the *Store struct.
//
// Cross-domain transactions are composed by calling the exported
// InTx helpers on sibling internal packages (for example,
// internal/tasks.Create calls internal/eval.AttachDraftEvaluationsInTx,
// internal/drafts.DeleteByIDInTx, and internal/checklist.
// ValidateCanMarkDoneInTx inside the same gorm transaction). This
// keeps every multi-table write atomic without adding a circular
// dependency on the public store package.
//
// Ready-task notifications are intentionally a facade-only concern.
// Methods that may transition a task into StatusReady return the
// updated row (and, for Update, the previous status); the facade
// fires (*Store).notifyReadyTask exactly once when the transition is
// observed. Subpackages do not import internal/notify.
//
// Sentinel errors are domain.ErrNotFound, domain.ErrInvalidInput, and
// domain.ErrConflict; the store does not log on errors.
//
// Pagination conventions: limit ≤ 0 becomes a per-method default
// (50 or 100); limit > 200 (or > 100 for drafts) is clamped to the
// per-method maximum; negative offset becomes 0. ListTaskEvents
// returns rows in ascending seq order; ListTaskEventsPageCursor is a
// descending-seq keyset page with total and navigation flags.
//
// File layout: README.md in this directory maps each public concern
// to its facade file and internal subpackage. Add new behavior as
// named use-case methods (small input structs, one transaction,
// explicit audit events) in the appropriate facade_*.go file and
// keep the heavy logic inside internal/<domain>/. The handler should
// stay thin; see docs/EXTENSIBILITY.md.
//
// [DefaultReadyTimeout] is the recommended context deadline for
// (*Store).Ready from GET /health/ready.
package store
