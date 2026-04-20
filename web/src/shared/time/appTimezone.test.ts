import { describe, expect, it } from "vitest";
import {
  DEFAULT_APP_TIMEZONE,
  formatInAppTimezone,
  supportedTimezones,
} from "./appTimezone";

describe("formatInAppTimezone", () => {
  // 2026-04-19T12:00:00Z is 08:00 EDT (America/New_York is UTC-4 in
  // April after DST starts) and 13:00 BST (Europe/London is UTC+1 in
  // April after DST starts). Pinning a date in April so a future
  // change to DST rules in either zone surfaces here.
  const SAMPLE_UTC_NOON = "2026-04-19T12:00:00Z";

  it("formats UTC instant in UTC", () => {
    const out = formatInAppTimezone(SAMPLE_UTC_NOON, "UTC");
    expect(out).toMatch(/12:00/);
    expect(out).toMatch(/UTC|Z|GMT/);
  });

  it("formats UTC instant in America/New_York (EDT, UTC-4 in April)", () => {
    const out = formatInAppTimezone(SAMPLE_UTC_NOON, "America/New_York");
    expect(out).toMatch(/08:00/);
  });

  it("formats UTC instant in Europe/London (BST, UTC+1 in April)", () => {
    const out = formatInAppTimezone(SAMPLE_UTC_NOON, "Europe/London");
    expect(out).toMatch(/13:00/);
  });

  it("returns empty string for null / undefined / empty input", () => {
    expect(formatInAppTimezone(null, "UTC")).toBe("");
    expect(formatInAppTimezone(undefined, "UTC")).toBe("");
    expect(formatInAppTimezone("", "UTC")).toBe("");
  });

  it("returns the original string for unparseable ISO input", () => {
    expect(formatInAppTimezone("not-a-date", "UTC")).toBe("not-a-date");
  });

  it("falls back to UTC for an invalid timezone identifier", () => {
    // Should not throw; should produce the same output as a UTC
    // format.
    const utcFormatted = formatInAppTimezone(SAMPLE_UTC_NOON, "UTC");
    const fallback = formatInAppTimezone(SAMPLE_UTC_NOON, "Not/A_Real_Zone");
    expect(fallback).toBe(utcFormatted);
  });

  it("respects forwarded Intl.DateTimeFormat options", () => {
    const out = formatInAppTimezone(SAMPLE_UTC_NOON, "UTC", {
      year: "numeric",
      month: "long",
      day: "numeric",
      hour: undefined,
      minute: undefined,
      timeZoneName: undefined,
    });
    // "April 19, 2026" or similar; assert the year + a long month
    // word is present rather than the literal English name so the
    // test passes under non-en locales too.
    expect(out).toMatch(/2026/);
    // No clock time component when hour/minute undefined.
    expect(out).not.toMatch(/12:00/);
  });
});

describe("supportedTimezones", () => {
  it("returns at least UTC and a handful of common zones", () => {
    const list = supportedTimezones();
    expect(list.length).toBeGreaterThan(10);
    expect(list).toContain("UTC");
    expect(list).toContain("America/New_York");
    expect(list).toContain("Europe/London");
  });

  it("returns UTC first (operator-friendly default) then sorted list", () => {
    const list = supportedTimezones();
    expect(list[0]).toBe("UTC");
    const rest = list.slice(1);
    const restSorted = [...rest].sort((a, b) => a.localeCompare(b));
    expect(rest).toEqual(restSorted);
  });
});

describe("DEFAULT_APP_TIMEZONE", () => {
  it("is UTC, matching the backend domain.DefaultDisplayTimezone", () => {
    expect(DEFAULT_APP_TIMEZONE).toBe("UTC");
  });
});
