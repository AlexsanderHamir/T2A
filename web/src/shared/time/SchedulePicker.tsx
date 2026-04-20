import { useId, useMemo } from "react";
import {
  formatInAppTimezone,
  isoToZonedDatetimeLocal,
  zonedDatetimeLocalToIso,
} from "./appTimezone";

type Props = {
  /**
   * Current schedule as an RFC3339 UTC ISO string, or `null` to mean
   * "no schedule — the agent picks this task up immediately when the
   * worker is free". The picker emits the same shape upward via
   * `onChange` so parents can serialize it directly into the
   * `pickup_not_before` field on the create / patch request body.
   */
  value: string | null;
  onChange: (next: string | null) => void;
  /**
   * IANA timezone identifier (e.g. "America/New_York") used for the
   * displayed wall-clock literal in the `<input>` AND the human
   * caption underneath. Wire format stays UTC; this is a render-only
   * concern.
   *
   * Decoupled from the `useAppTimezone()` hook so the picker is
   * trivially testable with any zone (the hook reads from
   * `useAppSettings`, which would force a `QueryClientProvider` setup
   * in every test) and reusable in stages 4 & 5 where the same
   * picker may render inside detail / list contexts that already
   * have the zone in scope.
   */
  appTimezone: string;
  disabled?: boolean;
  /**
   * Override for `Date.now()` when computing quick-pick anchors.
   * Tests pass a fixed clock so quick-pick output is deterministic;
   * production callers leave it undefined and we read from
   * `Date.now`.
   */
  nowMs?: number;
  /**
   * Optional id prefix for the contained input + caption — useful
   * when multiple pickers live on the same page (Stage 5 may render
   * the picker inline per row in a bulk-reschedule popover).
   */
  idPrefix?: string;
};

/**
 * SchedulePicker — operator-facing UI for the `pickup_not_before`
 * field. Composes a native `<input type="datetime-local">` with five
 * quick-pick chips so the common cases require zero typing:
 *
 *  - **In 1 hour**: now + 60 minutes (timezone-agnostic).
 *  - **Tonight 9 PM**: today 21:00 in `appTimezone`. If 21:00 already
 *    passed today, falls forward to tomorrow 21:00 (so the chip is
 *    never a no-op deferral into the past).
 *  - **Tomorrow 9 AM**: tomorrow 09:00 in `appTimezone`.
 *  - **Next Monday 9 AM**: the *next* Monday strictly in the future
 *    at 09:00 in `appTimezone`. If today is Monday before 09:00 we
 *    still go to next week — operators typing "Next Monday" almost
 *    never mean "later today even though today is Monday".
 *  - **Clear**: emits `null`.
 *
 * The native input shows naive local time (no zone suffix). The
 * caption beneath ("Agent will pick up at 2026-04-22 09:00 EDT") is
 * the source of truth for the operator: it always shows the
 * formatted instant in `appTimezone`, mirroring what the server
 * actually scheduled. This is the standard fix for the "I picked
 * 9 AM but it stored 9 AM UTC" trap that bites every native
 * datetime-local UI: we never let the browser guess a zone.
 *
 * Wire contract: emits an RFC3339 UTC ISO string for any concrete
 * pick, or `null` for "no schedule". Parents serialize this
 * directly into `pickup_not_before` on the create / patch body — the
 * server treats `null` (and the symmetrical empty-string on PATCH)
 * as "clear the schedule".
 */
export function SchedulePicker({
  value,
  onChange,
  appTimezone,
  disabled = false,
  nowMs,
  idPrefix,
}: Props) {
  const baseId = useId();
  const prefix = idPrefix ?? baseId;
  const inputId = `${prefix}-schedule-input`;
  const captionId = `${prefix}-schedule-caption`;
  const legendId = `${prefix}-schedule-legend`;

  const inputValue = useMemo(
    () => isoToZonedDatetimeLocal(value, appTimezone),
    [value, appTimezone],
  );

  const caption = useMemo(() => {
    if (!value) {
      return "Picks up immediately when the worker is free.";
    }
    const formatted = formatInAppTimezone(value, appTimezone);
    return `Agent will pick up at ${formatted}.`;
  }, [value, appTimezone]);

  const now = nowMs ?? Date.now();

  const handleInputChange = (raw: string) => {
    if (!raw) {
      onChange(null);
      return;
    }
    const iso = zonedDatetimeLocalToIso(raw, appTimezone);
    if (!iso) return;
    onChange(iso);
  };

  const handleQuickPick = (kind: QuickPickKind) => {
    if (kind === "clear") {
      onChange(null);
      return;
    }
    const iso = computeQuickPickIso(kind, now, appTimezone);
    if (!iso) return;
    onChange(iso);
  };

  return (
    <fieldset
      className="schedule-picker"
      disabled={disabled}
      aria-labelledby={legendId}
    >
      <legend id={legendId} className="schedule-picker-legend">
        Schedule for
      </legend>
      <div className="schedule-picker-row">
        <input
          id={inputId}
          type="datetime-local"
          className="schedule-picker-input"
          value={inputValue}
          onChange={(e) => handleInputChange(e.target.value)}
          aria-describedby={captionId}
          data-testid="schedule-picker-input"
        />
      </div>
      <div className="schedule-picker-chips" role="group" aria-label="Quick picks">
        <button
          type="button"
          className="schedule-picker-chip"
          onClick={() => handleQuickPick("in_1h")}
          data-testid="schedule-picker-in-1h"
        >
          In 1 hour
        </button>
        <button
          type="button"
          className="schedule-picker-chip"
          onClick={() => handleQuickPick("tonight_9pm")}
          data-testid="schedule-picker-tonight"
        >
          Tonight 9 PM
        </button>
        <button
          type="button"
          className="schedule-picker-chip"
          onClick={() => handleQuickPick("tomorrow_9am")}
          data-testid="schedule-picker-tomorrow"
        >
          Tomorrow 9 AM
        </button>
        <button
          type="button"
          className="schedule-picker-chip"
          onClick={() => handleQuickPick("next_monday_9am")}
          data-testid="schedule-picker-next-monday"
        >
          Next Monday 9 AM
        </button>
        <button
          type="button"
          className="schedule-picker-chip schedule-picker-chip--clear"
          onClick={() => handleQuickPick("clear")}
          data-testid="schedule-picker-clear"
          aria-disabled={value === null}
        >
          Clear
        </button>
      </div>
      <p id={captionId} className="schedule-picker-caption muted">
        {caption}
      </p>
    </fieldset>
  );
}

type QuickPickKind =
  | "in_1h"
  | "tonight_9pm"
  | "tomorrow_9am"
  | "next_monday_9am"
  | "clear";

/**
 * computeQuickPickIso converts a quick-pick chip selection into an
 * RFC3339 UTC ISO string anchored on `nowMs`.
 *
 * `in_1h` is timezone-agnostic — straight nowMs + 1h, since "in 1
 * hour" is a relative offset, not a wall-clock target.
 *
 * The wall-clock chips (`tonight_9pm`, `tomorrow_9am`,
 * `next_monday_9am`) need the operator's calendar context to
 * compute the right *date* component, so they:
 *
 *   1. Discover today's calendar date in `tz` via
 *      `isoToZonedDatetimeLocal(now, tz)` (which uses
 *      `Intl.DateTimeFormat` parts under the hood).
 *   2. Compose `YYYY-MM-DDTHH:mm` with the desired wall-clock target
 *      (21:00 / 09:00) in that calendar.
 *   3. Round-trip back to UTC via `zonedDatetimeLocalToIso(local, tz)`.
 *      That helper handles DST transitions correctly because the
 *      offset is computed at the candidate instant rather than at
 *      `now`.
 *
 * "Tonight 9 PM" falls forward to tomorrow 21:00 if 21:00 already
 * passed in `tz`, so the chip is never a no-op deferral into the
 * past (an operator clicking "Tonight" at 22:00 means "the next
 * 21:00", not "an hour ago"). "Next Monday" is always strictly in
 * the future — even on Monday before 09:00 we go to next week,
 * because typing "Next Monday" on a Monday almost never means
 * "today".
 */
function computeQuickPickIso(
  kind: Exclude<QuickPickKind, "clear">,
  nowMs: number,
  tz: string,
): string {
  if (kind === "in_1h") {
    return new Date(nowMs + 60 * 60 * 1000).toISOString();
  }

  const nowIsoZ = new Date(nowMs).toISOString();
  const todayLocal = isoToZonedDatetimeLocal(nowIsoZ, tz);
  if (!todayLocal) return new Date(nowMs).toISOString();
  // todayLocal is "YYYY-MM-DDTHH:mm" in `tz`; we only want the date
  // half + the dow-of-today.
  const dateMatch = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(todayLocal);
  if (!dateMatch) return new Date(nowMs).toISOString();
  const [, yStr, moStr, dStr, hStr, miStr] = dateMatch;
  const year = Number(yStr);
  const month = Number(moStr);
  const day = Number(dStr);
  const hour = Number(hStr);
  const minute = Number(miStr);

  if (kind === "tonight_9pm") {
    // 21:00 in `tz` today. If we're already past 21:00 in `tz`, jump
    // to tomorrow's 21:00 (chip is never a no-op deferral).
    const target = composeDateLocal(year, month, day, 21, 0);
    if (hour > 21 || (hour === 21 && minute > 0)) {
      return zonedDatetimeLocalToIso(addDaysToLocal(target, 1), tz) || target;
    }
    return zonedDatetimeLocalToIso(target, tz);
  }

  if (kind === "tomorrow_9am") {
    const today = composeDateLocal(year, month, day, 9, 0);
    return zonedDatetimeLocalToIso(addDaysToLocal(today, 1), tz);
  }

  if (kind === "next_monday_9am") {
    // Determine day-of-week for `year-month-day` using a UTC
    // construction (the day-of-week of a calendar date is
    // zone-independent because we're asking about the calendar
    // grid in `tz`, not a UTC instant). Day-of-week from
    // `Date.getUTCDay()` is 0=Sun..6=Sat.
    const dow = new Date(Date.UTC(year, month - 1, day)).getUTCDay();
    // Days until *next* Monday (strictly in the future):
    //   if today is Monday (dow=1): 7 days.
    //   else: ((1 - dow + 7) % 7) || 7.
    const daysUntilMonday = dow === 1 ? 7 : ((1 - dow + 7) % 7);
    const today = composeDateLocal(year, month, day, 9, 0);
    return zonedDatetimeLocalToIso(addDaysToLocal(today, daysUntilMonday), tz);
  }

  return new Date(nowMs).toISOString();
}

function composeDateLocal(
  year: number,
  month: number,
  day: number,
  hour: number,
  minute: number,
): string {
  return (
    String(year).padStart(4, "0") +
    "-" +
    String(month).padStart(2, "0") +
    "-" +
    String(day).padStart(2, "0") +
    "T" +
    String(hour).padStart(2, "0") +
    ":" +
    String(minute).padStart(2, "0")
  );
}

/**
 * addDaysToLocal advances a `YYYY-MM-DDTHH:mm` literal by `delta`
 * calendar days, preserving the wall-clock time. Implemented via
 * `Date.UTC` arithmetic on the parsed date components so we don't
 * accidentally pull a zone offset in from the host environment —
 * the caller will round-trip the result through
 * `zonedDatetimeLocalToIso(local, tz)` to recover the correct UTC
 * instant, which handles any DST boundary the wall-clock crosses.
 */
function addDaysToLocal(local: string, delta: number): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(local);
  if (!m) return local;
  const [, yStr, moStr, dStr, hStr, miStr] = m;
  const ms = Date.UTC(
    Number(yStr),
    Number(moStr) - 1,
    Number(dStr),
    Number(hStr),
    Number(miStr),
  );
  const next = new Date(ms + delta * 24 * 60 * 60 * 1000);
  return composeDateLocal(
    next.getUTCFullYear(),
    next.getUTCMonth() + 1,
    next.getUTCDate(),
    next.getUTCHours(),
    next.getUTCMinutes(),
  );
}
