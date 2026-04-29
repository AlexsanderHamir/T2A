import { useId, useMemo, useRef, useState } from "react";
import {
  formatInAppTimezone,
  isoToZonedDatetimeLocal,
  zonedDatetimeLocalToIso,
} from "./appTimezone";
import { QuickScheduleOffsetPopover } from "./QuickScheduleOffsetPopover";
import {
  computeOffsetIso,
  type QuickOffsetUnit,
} from "./quickScheduleOffsets";

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
 * trailing) plus a `Quick pick` button that opens an anchored popover
 * full of relative offsets (10..50 min, 1..24 h, 1..6 d, 1..3 w,
 * 1..12 mo). The popover replaces the legacy four-chip row so common
 * deferrals are one click away without polluting the field with a
 * long inline list.
 *
 * The native input shows naive local time (no zone suffix). The
 * caption beneath ("Agent will pick up at 2026-04-22 09:00 EDT") is
 * the source of truth for the operator: it always shows the
 * formatted instant in `appTimezone`, mirroring what the server
 * actually scheduled. This is the standard fix for the "I picked
 * 9 AM but it stored 9 AM UTC" trap that bites every native
 * datetime-local UI: we never let the browser guess a zone.
 *
 * Quick-pick chips inside the popover anchor on `Date.now()` (or
 * `nowMs` in tests) at the moment of the click — they are pure
 * "now + X" offsets, not wall-clock targets, so DST and timezone
 * concerns drop out for everything except the rare month-boundary
 * arithmetic in `computeOffsetIso(unit="month", ...)`.
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

  const [popoverOpen, setPopoverOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);

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

  const handleInputChange = (raw: string) => {
    if (!raw) {
      onChange(null);
      return;
    }
    const iso = zonedDatetimeLocalToIso(raw, appTimezone);
    if (!iso) return;
    onChange(iso);
  };

  const handleClear = () => {
    onChange(null);
  };

  const handleQuickPick = (unit: QuickOffsetUnit, amount: number) => {
    // Re-read `Date.now()` at the moment of the click rather than reusing a
    // stale render-time `now`, so the offset is anchored on the click
    // instant. Tests inject a fixed `nowMs` and we honor that.
    const anchorMs = nowMs ?? Date.now();
    const iso = computeOffsetIso(unit, amount, anchorMs);
    onChange(iso);
    setPopoverOpen(false);
    // Focus returns to the trigger so the operator can reopen the popover
    // from the keyboard (or jump back into the form via Tab).
    triggerRef.current?.focus();
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
            onClick={handleClear}
            data-testid="schedule-picker-clear"
            aria-label="Clear schedule"
            aria-disabled={!hasValue}
            tabIndex={hasValue ? 0 : -1}
          >
            <ClearGlyph />
          </button>
        </div>
        <div className="schedule-picker-quick">
          <span className="schedule-picker-quick-label">Quick pick</span>
          <button
            ref={triggerRef}
            type="button"
            className="schedule-picker-quick-trigger"
            data-testid="schedule-picker-quick-trigger"
            data-active={popoverOpen ? "true" : "false"}
            aria-haspopup="dialog"
            aria-expanded={popoverOpen}
            onClick={() => setPopoverOpen((open) => !open)}
          >
            <ClockGlyph />
            <span>Schedule for later…</span>
            <span className="schedule-picker-quick-trigger-chevron" aria-hidden="true">
              ▾
            </span>
          </button>
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
      {popoverOpen ? (
        <QuickScheduleOffsetPopover
          anchor={triggerRef.current}
          onPick={handleQuickPick}
          onClose={() => setPopoverOpen(false)}
        />
      ) : null}
    </fieldset>
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

function ClockGlyph() {
  // 14x14 clock to anchor the "Schedule for later…" trigger. Same
  // stroke-only treatment as CalendarGlyph so both icons read as
  // siblings inside the schedule well.
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 14 14"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.4"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="7" cy="7" r="5.25" />
      <path d="M7 4v3.25l2 1.4" />
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
