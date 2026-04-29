import { describe, expect, it } from "vitest";
import { formatRelativeTime } from "./relativeTime";

const NOW = new Date("2026-04-29T12:00:00Z");

function isoMinus(ms: number): string {
  return new Date(NOW.getTime() - ms).toISOString();
}

describe("formatRelativeTime", () => {
  it("returns empty string for empty / nullish input", () => {
    expect(formatRelativeTime("")).toBe("");
    expect(formatRelativeTime(null)).toBe("");
    expect(formatRelativeTime(undefined)).toBe("");
  });

  it("returns empty string for unparseable input", () => {
    expect(formatRelativeTime("not-a-date", NOW)).toBe("");
  });

  it("collapses anything under 45s to 'just now'", () => {
    expect(formatRelativeTime(isoMinus(0), NOW)).toBe("just now");
    expect(formatRelativeTime(isoMinus(10_000), NOW)).toBe("just now");
    expect(formatRelativeTime(isoMinus(44_000), NOW)).toBe("just now");
  });

  it("future timestamps (clock skew) also collapse to 'just now'", () => {
    const future = new Date(NOW.getTime() + 60_000).toISOString();
    expect(formatRelativeTime(future, NOW)).toBe("just now");
  });

  it("formats minutes for the [45s, 1h) range", () => {
    expect(formatRelativeTime(isoMinus(60_000), NOW)).toBe("1 min ago");
    expect(formatRelativeTime(isoMinus(5 * 60_000), NOW)).toBe("5 min ago");
    expect(formatRelativeTime(isoMinus(59 * 60_000), NOW)).toBe("59 min ago");
  });

  it("formats hours for the [1h, 24h) range", () => {
    expect(formatRelativeTime(isoMinus(60 * 60_000), NOW)).toBe("1 h ago");
    expect(formatRelativeTime(isoMinus(3 * 60 * 60_000), NOW)).toBe("3 h ago");
    expect(formatRelativeTime(isoMinus(23 * 60 * 60_000), NOW)).toBe("23 h ago");
  });

  it("formats days for the [1d, 7d) range", () => {
    expect(formatRelativeTime(isoMinus(24 * 60 * 60_000), NOW)).toBe("1 d ago");
    expect(formatRelativeTime(isoMinus(6 * 24 * 60 * 60_000), NOW)).toBe(
      "6 d ago",
    );
  });

  it("formats weeks for the [7d, 30d) range", () => {
    expect(formatRelativeTime(isoMinus(7 * 24 * 60 * 60_000), NOW)).toBe(
      "1 w ago",
    );
    expect(formatRelativeTime(isoMinus(29 * 24 * 60 * 60_000), NOW)).toBe(
      "4 w ago",
    );
  });

  it("formats months for the [30d, 365d) range", () => {
    expect(formatRelativeTime(isoMinus(30 * 24 * 60 * 60_000), NOW)).toBe(
      "1 mo ago",
    );
    expect(formatRelativeTime(isoMinus(364 * 24 * 60 * 60_000), NOW)).toBe(
      "12 mo ago",
    );
  });

  it("formats years for >= 365d", () => {
    expect(formatRelativeTime(isoMinus(365 * 24 * 60 * 60_000), NOW)).toBe(
      "1 y ago",
    );
    expect(formatRelativeTime(isoMinus(3 * 365 * 24 * 60 * 60_000), NOW)).toBe(
      "3 y ago",
    );
  });
});
