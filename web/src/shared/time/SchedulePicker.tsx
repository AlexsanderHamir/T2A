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
 * field. Composes a native `<input type="datetime-local">` wrapped in
 * a Stripe-style composed field (calendar icon leading + inline clear
 * trailing) with four quick-pick chips so the common cases require
 * zero typing:
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
 *
 * A fifth "clear" affordance lives inside the field as a trailing
 * icon button (no testid drift: still `schedule-picker-clear`) that
 * only animates in when a value is set, so the field feels like a
 * single cohesive control rather than a row of chips plus a separate
 * destructive button.
 *
 * The native input shows naive local time (no zone suffix). The
 * caption beneath ("Agent will pick up at 2026-04-22 09:00 EDT") is
 * the source of truth for the operator: it always shows the
 * formatted instant in `appTimezone`, mirroring what the server
 * actually scheduled. This is the standard fix for the "I picked
 * 9 AM but it stored 9 AM UTC" trap that bites every native
 * datetime-local UI: we never let the browser guess a zone.
 *
 * Quick-pick chips also report an "active" state (highlighted in
 * brand color) when the current value matches what the chip *would*
 * produce now — within a 2-minute tolerance for the relative
 * "In 1 hour" chip, exact-match for the wall-clock chips. Operators
 * get immediate visual feedback ("yes, you just picked Tonight")
 * without us having to track per-click state that wouldn't survive a
 * modal remount.
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

  // Active-chip detection: re-run each quick-pick against `now` and
  // compare to `value`. Wall-clock picks ("Tonight 9 PM", "Tomorrow
  // 9 AM", "Next Monday 9 AM") produce stable target ISOs within the
  // same calendar day, so they hit an exact-millisecond match after
  // a click. The relative "In 1 hour" target drifts by the wall-clock
  // render delta, so we allow a 2-minute tolerance — wide enough to
  // cover any plausible re-render gap, narrow enough to never
  // accidentally match an unrelated wall-clock pick sitting one hour
  // away (wall-clock picks are always ≥ 8h from "in 1 hour" in a
  // normal workflow).
  const activeKind = useMemo<Exclude<QuickPickKind, "clear"> | null>(() => {
    if (!value) return null;
    const valueMs = Date.parse(value);
    if (Number.isNaN(valueMs)) return null;
    const kinds: Exclude<QuickPickKind, "clear">[] = [
      "in_1h",
      "tonight_9pm",
      "tomorrow_9am",
      "next_monday_9am",
    ];
    for (const k of kinds) {
      const iso = computeQuickPickIso(k, now, appTimezone);
      if (!iso) continue;
      const targetMs = Date.parse(iso);
      if (Number.isNaN(targetMs)) continue;
      if (Math.abs(targetMs - valueMs) <= 120_000) return k;
    }
    return null;
  }, [value, now, appTimezone]);

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

  const hasValue = value !== null;

  return (
    <fieldset
      className="schedule-picker"
      disabled={disabled}
      aria-labelledby={legendId}
    >
      <legend id={legendId} className="schedule-picker-legend">
        Schedule for
      </legend>
      <div className="schedule-picker-well" data-scheduled={hasValue ? "true" : "false"}>
        <div className="schedule-picker-field">
          <span className="schedule-picker-field-icon" aria-hidden="true">
            <CalendarGlyph />
          </span>
          <input
            id={inputId}
            type="datetime-local"
            className="schedule-picker-input"
            value={inputValue}
            onChange={(e) => handleInputChange(e.target.value)}
            aria-describedby={captionId}
            data-testid="schedule-picker-input"
          />
          <button
            type="button"
            className="schedule-picker-clear-icon"
            onClick={() => handleQuickPick("clear")}
            data-testid="schedule-picker-clear"
            aria-label="Clear schedule"
            aria-disabled={!hasValue}
            tabIndex={hasValue ? 0 : -1}
          >
            <ClearGlyph />
          </button>
        </div>
        <div className="schedule-picker-quick">
          <span className="schedule-picker-quick-label">Quick picks</span>
          <div
            className="schedule-picker-chips"
            role="group"
            aria-label="Schedule quick picks"
          >
            <QuickChip
              testId="schedule-picker-in-1h"
              active={activeKind === "in_1h"}
              onClick={() => handleQuickPick("in_1h")}
            >
              In 1 hour
            </QuickChip>
            <QuickChip
              testId="schedule-picker-tonight"
              active={activeKind === "tonight_9pm"}
              onClick={() => handleQuickPick("tonight_9pm")}
            >
              Tonight 9 PM
            </QuickChip>
            <QuickChip
              testId="schedule-picker-tomorrow"
              active={activeKind === "tomorrow_9am"}
              onClick={() => handleQuickPick("tomorrow_9am")}
            >
              Tomorrow 9 AM
            </QuickChip>
            <QuickChip
              testId="schedule-picker-next-monday"
              active={activeKind === "next_monday_9am"}
              onClick={() => handleQuickPick("next_monday_9am")}
            >
              Next Monday 9 AM
            </QuickChip>
          </div>
        </div>
        <p
          id={captionId}
          className="schedule-picker-caption"
          data-scheduled={hasValue ? "true" : "false"}
        >
          <span className="schedule-picker-status-dot" aria-hidden="true" />
          <span>{caption}</span>
        </p>
      </div>
    </fieldset>
  );
}

type QuickChipProps = {
  testId: string;
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
};

function QuickChip({ testId, active, onClick, children }: QuickChipProps) {
  return (
    <button
      type="button"
      className="schedule-picker-chip"
      data-testid={testId}
      data-active={active ? "true" : "false"}
      aria-pressed={active}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

function CalendarGlyph() {
  // 16x16 calendar. Stroke-only, `currentColor` so the icon follows
  // the field's text color and respects dark-mode inversion without a
  // separate icon asset.
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.4"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect x="2.25" y="3.5" width="11.5" height="10.25" rx="2" />
      <path d="M2.25 6.5h11.5" />
      <path d="M5.5 2v3" />
      <path d="M10.5 2v3" />
    </svg>
  );
}

function ClearGlyph() {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 12 12"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
    >
      <path d="M3 3l6 6" />
      <path d="M9 3l-6 6" />
    </svg>
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
