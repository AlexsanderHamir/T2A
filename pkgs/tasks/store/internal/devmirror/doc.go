// Package devmirror owns the dev-only "synthetic event -> task row
// mirror" path used by pkgs/tasks/devsim. ApplyTaskRowMirror updates
// a task row to reflect a fabricated audit event WITHOUT appending
// further audit rows, so the simulator can drive the UI through
// states without polluting the real audit log. ListDevsimTasks is
// the LIKE-pattern lookup the simulator uses to find its own rows.
//
// This subpackage is intentionally separate from internal/tasks
// because the dev-mirror path has different invariants (it bypasses
// the seq counter, may set status to terminal values without going
// through the usual transitions). Keeping the dev-only code in its
// own package makes that asymmetry obvious to readers and to the
// "no production import" guard expected by AGENTS.md.
//
// ApplyTaskRowMirror returns the reloaded task and the previous
// status so the caller (the public store facade) can decide whether
// to fire the ready-task notifier; the package itself does not hold
// a notifier reference.
package devmirror
