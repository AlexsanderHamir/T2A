import { errorMessage } from "@/lib/errorMessage";

type Props = {
  /**
   * The mutation / async error to surface, or `null` / `undefined` to
   * render nothing. Accepts both shapes that exist across the codebase:
   *   - `Error | null` — what react-query's `mutation.error` is typed as
   *     (`react-query` v5). Caller passes it through as-is.
   *   - `string | null` — what the flow hooks (`useTaskPatchFlow`,
   *     `useTaskDeleteFlow`) expose after pre-coercing the underlying
   *     `Error` via `errorMessage(...)`. Caller passes it through as-is.
   * Either way the banner runs the value through `errorMessage(error,
   * fallback)` so the wire shape is consistent: `Error` instances always
   * render their `.message`, strings render verbatim, and the `fallback`
   * only kicks in for non-Error / non-string values (defensive against
   * future flows that might throw plain objects).
   *
   * Passing `null` or `undefined` makes the component render nothing
   * (no empty `role="alert"` live-region churn at idle), so callers can
   * safely render `<MutationErrorBanner error={mutation.error} ... />`
   * unconditionally without an outer `error ? ... : null` gate.
   */
  error?: Error | string | null;
  /**
   * Kinder default the surrounding UI prefers over the raw `String(e)`
   * coercion for non-Error / non-string inputs (e.g. "Could not save
   * changes."). Forwarded straight through to `errorMessage(error,
   * fallback)`.
   *
   * Has no effect on `Error` instances (their `.message` always wins,
   * matching the `errorMessage` contract) or on plain strings (they
   * render verbatim). The fallback is a safety net for the unusual case
   * where the underlying mutation throws a non-Error value — the wire
   * shape used to be unstable across the app, and centralizing here
   * means a future caller doesn't have to remember to pass the
   * fallback through twice.
   */
  fallback?: string;
  /**
   * Extra class(es) appended to the canonical `err` selector so callers
   * can pin per-site visual hooks (e.g. `task-create-modal-err
   * task-create-modal-err--evaluate` to position the callout next to
   * the evaluate action). Must NOT include the leading `err` — the
   * banner adds it itself so the canonical app-base styling
   * (`.err > p { margin: 0 }`, danger background, inherited color)
   * always applies.
   */
  className?: string;
};

/**
 * Inline error callout for in-flow recoverable errors (mutation failures,
 * draft load failures, manual user-fix requests). Use over the page-level
 * `<ErrorBanner />` whenever the error happens inside a modal, popover,
 * or other overlay where the page-level banner would be hidden — see
 * `.agent/frontend-improvement-agent.log` Sessions 31-34 for the full
 * audit (the create / evaluate / subtask / checklist / patch / delete
 * surfaces all hit this same backdrop-hides-banner gap).
 *
 * Accessibility: emits `role="alert"` so screen-readers announce the
 * message immediately on render. Renders nothing when `error` is null /
 * undefined so we don't churn an empty live-region between submits.
 *
 * Why a single component instead of leaving the `<div className="err"
 * role="alert">` JSX inline at every call site:
 *   - Six call sites (ChecklistCriterionModal, SubtaskCreateModal,
 *     TaskCreateModal x2, DeleteConfirmDialog, TaskEditForm) had drifted
 *     into two different inner-shape patterns (`<p>{message}</p>` vs.
 *     bare text) — unifying here gives every alert the same DOM shape
 *     and lets the canonical `.err > p { margin: 0 }` rule own the
 *     spacing rather than each site re-inventing it.
 *   - Centralizing the `errorMessage(error, fallback)` call here means
 *     the shape contract (`Error | string | null`) is spelled out in
 *     one place; future callers don't have to remember to pre-coerce
 *     to a string nor to gate on `error ? ... : null`.
 *   - Future error-surface improvements (e.g. retry button slot,
 *     dismiss button, `aria-live="polite"` switch for less urgent
 *     errors) become a single edit instead of a six-place sweep.
 *
 * Reserved for in-flow inline use; the page-level layout still uses
 * `<ErrorBanner />` for the global rollup, and unrecoverable render
 * errors still use `<AppErrorBoundary />`.
 */
export function MutationErrorBanner({ error, fallback, className }: Props) {
  if (error === null || error === undefined) return null;
  // Strings are already user-presentable (the flow hooks pre-coerce via
  // `errorMessage(...)` before exposing `patchError` / `deleteError` /
  // `loadError` etc.), so render them verbatim without going through
  // `errorMessage` again — that helper's fallback path would clobber a
  // legitimate empty string with the fallback phrase, which is the wrong
  // semantics for a deliberately-already-coerced string. `Error`
  // instances and the rare unknown throwable still go through
  // `errorMessage(error, fallback)` so the contract (`Error.message`
  // wins, fallback for non-Error) matches every other site.
  const message =
    typeof error === "string" ? error : errorMessage(error, fallback);
  const cls = className ? `err ${className}` : "err";
  return (
    <div className={cls} role="alert">
      <p>{message}</p>
    </div>
  );
}
