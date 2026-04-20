import { describe, expect, it } from "vitest";
import {
  DEFAULT_APP_TIMEZONE,
  detectBrowserTimezone,
  formatInAppTimezone,
  formatTimezoneMenuLabel,
  getTimezoneOffsetMinutesAt,
  getTimezoneSelectOptions,
  isoToZonedDatetimeLocal,
  supportedTimezones,
  zonedDatetimeLocalToIso,
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

describe("formatTimezoneMenuLabel / getTimezoneSelectOptions", () => {
  const summerUtc = new Date("2026-07-15T12:00:00Z");

  it("formats Asia/Tokyo with GMT+09 and city name", () => {
    const label = formatTimezoneMenuLabel("Asia/Tokyo", summerUtc);
    expect(label).toMatch(/\(GMT\+09:00\)/);
    expect(label).toMatch(/Tokyo/);
  });

  it("formats UTC as GMT+00:00", () => {
    expect(formatTimezoneMenuLabel("UTC", summerUtc)).toMatch(
      /\(GMT\+00:00\).*UTC/,
    );
  });

  it("sorts getTimezoneSelectOptions by offset (NY before Tokyo in summer)", () => {
    const opts = getTimezoneSelectOptions(summerUtc);
    const idxNy = opts.findIndex((o) => o.value === "America/New_York");
    const idxTokyo = opts.findIndex((o) => o.value === "Asia/Tokyo");
    expect(idxNy).toBeGreaterThanOrEqual(0);
    expect(idxTokyo).toBeGreaterThanOrEqual(0);
    expect(idxNy).toBeLessThan(idxTokyo);
  });

  it("getTimezoneOffsetMinutesAt matches Tokyo +9 in July", () => {
    expect(getTimezoneOffsetMinutesAt("Asia/Tokyo", summerUtc)).toBe(9 * 60);
  });
});

describe("isoToZonedDatetimeLocal / zonedDatetimeLocalToIso", () => {
  it("formats a UTC instant as the matching wall-clock literal in UTC", () => {
    expect(isoToZonedDatetimeLocal("2026-04-19T12:34:00Z", "UTC")).toBe(
      "2026-04-19T12:34",
    );
  });

  it("formats a UTC instant as the matching wall-clock literal in America/New_York (EDT, UTC-4 in April)", () => {
    expect(
      isoToZonedDatetimeLocal("2026-04-19T12:00:00Z", "America/New_York"),
    ).toBe("2026-04-19T08:00");
  });

  it("formats a UTC instant as the matching wall-clock literal in Asia/Tokyo (UTC+9, no DST)", () => {
    expect(
      isoToZonedDatetimeLocal("2026-04-19T00:00:00Z", "Asia/Tokyo"),
    ).toBe("2026-04-19T09:00");
  });

  it("returns empty string for blank / unparseable iso", () => {
    expect(isoToZonedDatetimeLocal("", "UTC")).toBe("");
    expect(isoToZonedDatetimeLocal(null, "UTC")).toBe("");
    expect(isoToZonedDatetimeLocal(undefined, "UTC")).toBe("");
    expect(isoToZonedDatetimeLocal("not-a-date", "UTC")).toBe("");
  });

  it("round-trips through zonedDatetimeLocalToIso for UTC", () => {
    const iso = "2026-04-19T12:34:00.000Z";
    const local = isoToZonedDatetimeLocal(iso, "UTC");
    expect(zonedDatetimeLocalToIso(local, "UTC")).toBe(iso);
  });

  it("round-trips through zonedDatetimeLocalToIso for America/New_York", () => {
    const iso = "2026-04-19T12:00:00.000Z";
    const local = isoToZonedDatetimeLocal(iso, "America/New_York");
    expect(local).toBe("2026-04-19T08:00");
    expect(zonedDatetimeLocalToIso(local, "America/New_York")).toBe(iso);
  });

  it("round-trips through zonedDatetimeLocalToIso for Asia/Tokyo", () => {
    const iso = "2026-04-19T00:00:00.000Z";
    const local = isoToZonedDatetimeLocal(iso, "Asia/Tokyo");
    expect(local).toBe("2026-04-19T09:00");
    expect(zonedDatetimeLocalToIso(local, "Asia/Tokyo")).toBe(iso);
  });

  it("zonedDatetimeLocalToIso returns empty for empty / malformed input", () => {
    expect(zonedDatetimeLocalToIso("", "UTC")).toBe("");
    expect(zonedDatetimeLocalToIso("not-a-datetime", "UTC")).toBe("");
  });

  it("zonedDatetimeLocalToIso accepts the optional :ss suffix some browsers emit", () => {
    expect(zonedDatetimeLocalToIso("2026-04-19T12:34:56", "UTC")).toBe(
      "2026-04-19T12:34:56.000Z",
    );
  });
});

describe("DEFAULT_APP_TIMEZONE", () => {
  it("is UTC — the client-side safety fallback when browser auto-detect fails", () => {
    // The backend's domain.DefaultDisplayTimezone is now "" (the
    // auto-detect sentinel). DEFAULT_APP_TIMEZONE keeps its "UTC"
    // value as the final fallback zone formatInAppTimezone uses when
    // Intl.DateTimeFormat refuses to produce a zone at all. See the
    // doc comment on the constant for the full precedence chain.
    expect(DEFAULT_APP_TIMEZONE).toBe("UTC");
  });
});

describe("detectBrowserTimezone", () => {
  it("returns a non-empty IANA zone identifier from Intl.DateTimeFormat", () => {
    const tz = detectBrowserTimezone();
    expect(typeof tz).toBe("string");
    expect(tz.length).toBeGreaterThan(0);
    // Every zone Intl returns is either "UTC" or contains a "/"
    // (continent/city, "Etc/...", etc.). Asserting the shape instead
    // of a literal keeps the test independent of the CI runner's
    // actual timezone.
    expect(tz === "UTC" || tz.includes("/")).toBe(true);
  });
});
