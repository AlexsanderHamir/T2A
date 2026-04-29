/**
 * Relative-offset preset catalog for the SchedulePicker quick-pick popover.
 *
 * Each bucket exposes a unit (`minute`, `hour`, `day`, `week`, `month`) and a
 * curated list of offsets the operator can apply with one click. The picker
 * always anchors on `Date.now()` (or the test-injected clock) at the moment
 * the chip is clicked — there is no wall-clock target component, just
 * "now + amount".
 *
 * Bounds are intentional and chosen so adjacent buckets do not overlap
 * meaningfully:
 *
 *  - Minutes: 10..50 in 10-minute steps (anything ≥ 60 belongs in Hours).
 *  - Hours:   1..24 in 1-hour steps (anything ≥ 25 belongs in Days).
 *  - Days:    1..6 in 1-day steps (7 belongs in Weeks).
 *  - Weeks:   1..3 in 1-week steps (4 belongs in Months).
 *  - Months:  1..12 in 1-month steps.
 *
 * Month arithmetic is calendar-aware (preserves day-of-month and clamps to
 * the last day of the target month when the source date does not exist
 * there — e.g. Jan 31 + 1 month = Feb 28/29). Every other unit is plain
 * millisecond addition, which is correct because operators reading a chip
 * that says "+3d" mean exactly 3 × 24h, not "the same wall-clock time three
 * calendar days from now". Server validation accepts any RFC3339 instant.
 */

export type QuickOffsetUnit = "minute" | "hour" | "day" | "week" | "month";

export type QuickOffsetBucket = {
  unit: QuickOffsetUnit;
  /** Section header rendered inside the popover (uppercase styling applied via CSS). */
  label: string;
  /** Singular noun used in `aria-label` strings ("3 hours from now"). */
  ariaUnitSingular: string;
  /** Plural noun used in `aria-label` strings. */
  ariaUnitPlural: string;
  amounts: number[];
  /** Compact chip face — just the number; the section label conveys the unit. */
  formatChip: (amount: number) => string;
};

export const QUICK_OFFSET_BUCKETS: QuickOffsetBucket[] = [
  {
    unit: "minute",
    label: "Minutes",
    ariaUnitSingular: "minute",
    ariaUnitPlural: "minutes",
    amounts: [10, 20, 30, 40, 50],
    formatChip: (n) => `${n}`,
  },
  {
    unit: "hour",
    label: "Hours",
    ariaUnitSingular: "hour",
    ariaUnitPlural: "hours",
    amounts: Array.from({ length: 24 }, (_, i) => i + 1),
    formatChip: (n) => `${n}`,
  },
  {
    unit: "day",
    label: "Days",
    ariaUnitSingular: "day",
    ariaUnitPlural: "days",
    amounts: [1, 2, 3, 4, 5, 6],
    formatChip: (n) => `${n}`,
  },
  {
    unit: "week",
    label: "Weeks",
    ariaUnitSingular: "week",
    ariaUnitPlural: "weeks",
    amounts: [1, 2, 3],
    formatChip: (n) => `${n}`,
  },
  {
    unit: "month",
    label: "Months",
    ariaUnitSingular: "month",
    ariaUnitPlural: "months",
    amounts: Array.from({ length: 12 }, (_, i) => i + 1),
    formatChip: (n) => `${n}`,
  },
];

const MS_PER_MINUTE = 60 * 1000;
const MS_PER_HOUR = 60 * MS_PER_MINUTE;
const MS_PER_DAY = 24 * MS_PER_HOUR;
const MS_PER_WEEK = 7 * MS_PER_DAY;

/**
 * Return the RFC3339 UTC ISO string for `nowMs + amount × unit`. For minute /
 * hour / day / week the offset is a flat millisecond add; for month the
 * computation is calendar-based via UTC `Date` setters and clamps to the
 * last day of the target month when the source day does not exist there
 * (Jan 31 + 1 month = Feb 28 or Feb 29 depending on year).
 *
 * Returns the input now ISO when `amount` is non-positive — chips never emit
 * a "no-op" or "in the past" pick. Callers should treat any negative-amount
 * result as a programmer error, not a user-facing case.
 */
export function computeOffsetIso(
  unit: QuickOffsetUnit,
  amount: number,
  nowMs: number,
): string {
  if (!Number.isFinite(amount) || amount <= 0) {
    return new Date(nowMs).toISOString();
  }

  if (unit === "month") {
    const d = new Date(nowMs);
    const sourceMonth = d.getUTCMonth();
    const targetMonth = sourceMonth + amount;
    const expectedMonth = ((targetMonth % 12) + 12) % 12;
    const candidate = new Date(d.getTime());
    candidate.setUTCMonth(targetMonth);
    if (candidate.getUTCMonth() !== expectedMonth) {
      // setUTCMonth rolled over (e.g. Jan 31 → Feb 31 → Mar 3). Pull back
      // to the last day of the intended month.
      candidate.setUTCDate(0);
    }
    return candidate.toISOString();
  }

  const ms =
    unit === "minute"
      ? amount * MS_PER_MINUTE
      : unit === "hour"
        ? amount * MS_PER_HOUR
        : unit === "day"
          ? amount * MS_PER_DAY
          : amount * MS_PER_WEEK;
  return new Date(nowMs + ms).toISOString();
}

/**
 * Render a screen-reader-friendly label for a chip. The visible chip face is
 * just the number, so the accessible name has to spell out the unit and
 * include a hint that the offset is anchored on "now". Matches the format
 * VoiceOver / NVDA users expect ("Schedule for 3 hours from now").
 */
export function quickOffsetChipAriaLabel(
  bucket: QuickOffsetBucket,
  amount: number,
): string {
  const noun = amount === 1 ? bucket.ariaUnitSingular : bucket.ariaUnitPlural;
  return `Schedule for ${amount} ${noun} from now`;
}
