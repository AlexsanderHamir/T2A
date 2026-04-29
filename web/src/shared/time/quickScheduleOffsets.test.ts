import { describe, expect, it } from "vitest";
import {
  QUICK_OFFSET_BUCKETS,
  computeOffsetIso,
  quickOffsetChipAriaLabel,
} from "./quickScheduleOffsets";

const NOON_2026_04_19 = new Date("2026-04-19T12:00:00Z").getTime();

describe("QUICK_OFFSET_BUCKETS", () => {
  it("declares minute / hour / day / week / month buckets in display order", () => {
    expect(QUICK_OFFSET_BUCKETS.map((b) => b.unit)).toEqual([
      "minute",
      "hour",
      "day",
      "week",
      "month",
    ]);
  });

  it("uses the documented amount ranges per bucket", () => {
    const byUnit = Object.fromEntries(
      QUICK_OFFSET_BUCKETS.map((b) => [b.unit, b.amounts]),
    );
    expect(byUnit.minute).toEqual([10, 20, 30, 40, 50]);
    expect(byUnit.hour).toEqual(Array.from({ length: 24 }, (_, i) => i + 1));
    expect(byUnit.day).toEqual([1, 2, 3, 4, 5, 6]);
    expect(byUnit.week).toEqual([1, 2, 3]);
    expect(byUnit.month).toEqual(Array.from({ length: 12 }, (_, i) => i + 1));
  });
});

describe("computeOffsetIso — millisecond units", () => {
  it("adds minutes precisely", () => {
    expect(computeOffsetIso("minute", 30, NOON_2026_04_19)).toBe(
      "2026-04-19T12:30:00.000Z",
    );
  });

  it("adds hours precisely (24h crosses midnight)", () => {
    expect(computeOffsetIso("hour", 24, NOON_2026_04_19)).toBe(
      "2026-04-20T12:00:00.000Z",
    );
  });

  it("adds days precisely", () => {
    expect(computeOffsetIso("day", 3, NOON_2026_04_19)).toBe(
      "2026-04-22T12:00:00.000Z",
    );
  });

  it("adds weeks precisely (1w = 7d, anchored on now)", () => {
    expect(computeOffsetIso("week", 2, NOON_2026_04_19)).toBe(
      "2026-05-03T12:00:00.000Z",
    );
  });
});

describe("computeOffsetIso — calendar months", () => {
  it("preserves day-of-month when the target month has enough days", () => {
    const apr15 = new Date("2026-04-15T09:00:00Z").getTime();
    expect(computeOffsetIso("month", 1, apr15)).toBe("2026-05-15T09:00:00.000Z");
  });

  it("clamps to the last day of the target month when the source day does not exist", () => {
    // Jan 31 + 1 month -> Feb 28 (2026 is not a leap year)
    const jan31 = new Date("2026-01-31T09:00:00Z").getTime();
    expect(computeOffsetIso("month", 1, jan31)).toBe("2026-02-28T09:00:00.000Z");
  });

  it("clamps to Feb 29 in a leap year", () => {
    // 2024 is a leap year. Jan 31 + 1 month -> Feb 29.
    const jan31Leap = new Date("2024-01-31T09:00:00Z").getTime();
    expect(computeOffsetIso("month", 1, jan31Leap)).toBe(
      "2024-02-29T09:00:00.000Z",
    );
  });

  it("crosses years (Dec + 12 months = next December)", () => {
    const dec1 = new Date("2026-12-01T00:00:00Z").getTime();
    expect(computeOffsetIso("month", 12, dec1)).toBe("2027-12-01T00:00:00.000Z");
  });

  it("returns now when amount is non-positive", () => {
    const stamp = new Date("2026-06-15T08:00:00Z").getTime();
    expect(computeOffsetIso("month", 0, stamp)).toBe("2026-06-15T08:00:00.000Z");
    expect(computeOffsetIso("hour", -1, stamp)).toBe("2026-06-15T08:00:00.000Z");
  });
});

describe("quickOffsetChipAriaLabel", () => {
  it("uses singular nouns for amount = 1", () => {
    const hourBucket = QUICK_OFFSET_BUCKETS.find((b) => b.unit === "hour")!;
    expect(quickOffsetChipAriaLabel(hourBucket, 1)).toBe(
      "Schedule for 1 hour from now",
    );
  });

  it("uses plural nouns for amount > 1", () => {
    const dayBucket = QUICK_OFFSET_BUCKETS.find((b) => b.unit === "day")!;
    expect(quickOffsetChipAriaLabel(dayBucket, 3)).toBe(
      "Schedule for 3 days from now",
    );
  });
});
