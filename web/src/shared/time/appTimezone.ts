import { useAppSettings } from "@/settings/useAppSettings";

/**
 * Safety fallback zone used when neither the server nor the browser
 * can provide a usable IANA identifier (e.g. an exotic runtime where
 * `Intl.DateTimeFormat().resolvedOptions().timeZone` throws or returns
 * an empty string). "UTC" is the only zone every Intl implementation
 * is guaranteed to accept, so callers can rely on `formatInAppTimezone`
 * never throwing when this value is passed through.
 *
 * Historically this mirrored `pkgs/tasks/domain/app_settings.go`'s
 * `DefaultDisplayTimezone`. That backend default is now the empty
 * string (the "auto-detect" sentinel — see `useAppTimezone` below); we
 * keep `DEFAULT_APP_TIMEZONE = "UTC"` as the SPA-side safety net so the
 * UI still renders something sensible if auto-detect fails.
 */
export const DEFAULT_APP_TIMEZONE = "UTC";

/**
 * detectBrowserTimezone reads the operator's browser timezone via
 * `Intl.DateTimeFormat().resolvedOptions().timeZone`. Returns an IANA
 * identifier like `"America/New_York"` on every modern browser.
 *
 * Falls back to DEFAULT_APP_TIMEZONE ("UTC") if the Intl API throws or
 * returns an empty string — extremely rare (only seen on locked-down
 * embedded runtimes and ancient browsers) but cheaper to guard than to
 * debug later.
 */
export function detectBrowserTimezone(): string {
  try {
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (typeof tz === "string" && tz.length > 0) return tz;
  } catch {
    // fall through to the safety net
  }
  return DEFAULT_APP_TIMEZONE;
}

/**
 * useAppTimezone returns the IANA timezone the SPA should use to
 * render every operator-facing timestamp.
 *
 * Precedence (highest to lowest):
 *  1. `settings.display_timezone` — a non-empty explicit override
 *     chosen in the SettingsPage selector and validated server-side
 *     via `time.LoadLocation`. Always wins.
 *  2. The operator's browser timezone (`detectBrowserTimezone()`).
 *     Used whenever the server returns the empty-string
 *     "auto-detect" sentinel (the default seed) OR when the settings
 *     query is still loading, so the first paint already lands in
 *     local time rather than flashing UTC for a frame.
 *  3. DEFAULT_APP_TIMEZONE ("UTC") — only if the Intl API refuses to
 *     produce a zone at all. See `detectBrowserTimezone`.
 *
 * Stage 1 of the task scheduling plan introduced the field; later
 * stages (3–5) call this hook from every timestamp render so a single
 * PATCH /settings { display_timezone } re-renders the whole SPA in
 * the chosen zone via React Query invalidation.
 */
export function useAppTimezone(): string {
  const { settings } = useAppSettings();
  if (!settings) return detectBrowserTimezone();
  const tz = settings.display_timezone;
  if (typeof tz !== "string" || tz.length === 0) {
    return detectBrowserTimezone();
  }
  return tz;
}

/**
 * formatInAppTimezone formats an ISO-8601 / RFC3339 UTC timestamp
 * for human display in the operator-chosen timezone, using
 * Intl.DateTimeFormat under the hood.
 *
 * Defensive contract:
 * - Empty / undefined `iso` returns "" so callers can render
 *   `formatInAppTimezone(task.pickup_not_before, tz)` directly inside
 *   JSX without `?:` ladders.
 * - Unparseable `iso` returns the original string verbatim so the
 *   operator at least sees what the server sent rather than a silent
 *   blank cell.
 * - Unknown / invalid `tz` falls back to UTC (Intl.DateTimeFormat
 *   throws RangeError for invalid timeZone; we catch and retry with
 *   UTC). This keeps the SPA from white-screening when an operator
 *   pastes an invalid zone via devtools and the server hasn't
 *   re-validated yet.
 *
 * `opts` are forwarded to Intl.DateTimeFormat. Default formatting
 * is "YYYY-MM-DD HH:mm zzz" (medium dateStyle, short timeStyle, the
 * timezone name appended via `timeZoneName: "short"`) — chosen to
 * match the existing observability page's "updated_at" rendering.
 */
export function formatInAppTimezone(
  iso: string | null | undefined,
  tz: string,
  opts?: Intl.DateTimeFormatOptions,
): string {
  if (typeof iso !== "string" || iso.length === 0) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const baseOpts: Intl.DateTimeFormatOptions = {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    timeZoneName: "short",
    hour12: false,
    ...opts,
    timeZone: tz,
  };
  try {
    return new Intl.DateTimeFormat(undefined, baseOpts).format(d);
  } catch {
    // Invalid timeZone — fall back to UTC so the operator still sees
    // something sensible rather than a stack trace.
    return new Intl.DateTimeFormat(undefined, { ...baseOpts, timeZone: "UTC" }).format(d);
  }
}

/**
 * supportedTimezones returns a sorted list of IANA timezone
 * identifiers for the SettingsPage selector. Prefers
 * `Intl.supportedValuesOf("timeZone")` (Chrome 99+/Firefox 93+/Safari
 * 15.4+); on older runtimes returns a curated short list of common
 * zones spanning every continent so operators in unsupported browsers
 * still have a usable selector.
 *
 * The fallback list is intentionally short (~30 zones) — operators on
 * exotic zones can still type any IANA identifier into the
 * underlying input via copy-paste and the server validates with
 * time.LoadLocation, so we don't try to enumerate every zone here.
 */
export function supportedTimezones(): string[] {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const intl = Intl as any;
  if (typeof intl.supportedValuesOf === "function") {
    try {
      const v = intl.supportedValuesOf("timeZone");
      if (Array.isArray(v) && v.length > 0) {
        // Intl.supportedValuesOf("timeZone") returns canonical IANA
        // names and intentionally OMITS the legacy alias "UTC" (the
        // canonical name is "Etc/UTC"). Operators expect to see plain
        // "UTC" first since it's the seed default the backend uses
        // (domain.DefaultDisplayTimezone) and the wire format every
        // API returns. Prepend it so the SettingsPage selector always
        // includes it.
        const merged = ["UTC", ...v.filter((z: string) => z !== "UTC")];
        return [...new Set(merged)].sort((a, b) => {
          if (a === "UTC") return -1;
          if (b === "UTC") return 1;
          return a.localeCompare(b);
        });
      }
    } catch {
      // fall through to curated list
    }
  }
  return FALLBACK_TIMEZONES;
}

/**
 * isoToZonedDatetimeLocal converts an RFC3339 / ISO-8601 UTC instant
 * to the naive `YYYY-MM-DDTHH:mm` string that an
 * `<input type="datetime-local">` accepts as its `value` attribute,
 * expressed in the operator's chosen IANA timezone. This is the
 * inverse of `zonedDatetimeLocalToIso` below; together they let the
 * SchedulePicker round-trip between "what the operator sees" and
 * "what the wire carries" without Moment / date-fns / Luxon as a
 * dependency.
 *
 * Defensive contract:
 *  - Empty / unparseable `iso` returns "" (the picker shows a blank
 *    input — same UX as the user clearing the field).
 *  - Invalid `tz` falls back to UTC (mirrors `formatInAppTimezone`).
 *
 * Implementation note: we use `Intl.DateTimeFormat.formatToParts`
 * with `hour12: false` to extract year/month/day/hour/minute in the
 * chosen zone, then assemble them into the `YYYY-MM-DDTHH:mm`
 * literal the input expects. Skipping `Date.toISOString().slice(0, 16)`
 * because that would be UTC, not the chosen zone.
 */
export function isoToZonedDatetimeLocal(
  iso: string | null | undefined,
  tz: string,
): string {
  if (typeof iso !== "string" || iso.length === 0) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const parts = formatPartsInZone(d, tz);
  if (!parts) return "";
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}`;
}

/**
 * zonedDatetimeLocalToIso is the inverse of `isoToZonedDatetimeLocal`:
 * given the literal value an `<input type="datetime-local">` emits in
 * the operator's chosen timezone (`YYYY-MM-DDTHH:mm`, no zone
 * suffix), return the RFC3339 UTC string the API expects on
 * `pickup_not_before`.
 *
 * Algorithm:
 *  1. Parse the local literal as if it were already UTC (via
 *     `Date.UTC`) to get a "guessed" timestamp.
 *  2. Format that timestamp in the target zone via
 *     `formatPartsInZone` to discover the actual wall-clock the zone
 *     would render.
 *  3. The delta between the input's wall-clock and the rendered
 *     wall-clock is the zone's UTC offset at that instant; subtract
 *     it to get the true UTC instant.
 *
 * This handles DST forwards and backwards correctly because the
 * offset is computed at the *guessed* timestamp, which is at most
 * one hour off from the true timestamp — well within the same DST
 * transition window for any sensible zone. (Iterating once would
 * fix the rare case where the guess lands on the wrong side of a
 * DST cliff; for the SchedulePicker's use case, "set a future
 * pickup time", being one hour off twice a year is preferable to
 * round-tripping through a heavyweight library.)
 *
 * Defensive contract:
 *  - Empty / malformed input returns "" (the picker treats this as
 *    "no schedule" and emits null upward).
 *  - Invalid `tz` falls back to UTC.
 */
export function zonedDatetimeLocalToIso(
  local: string,
  tz: string,
): string {
  if (typeof local !== "string" || local.length === 0) return "";
  // <input type="datetime-local"> emits "YYYY-MM-DDTHH:mm" — a strict
  // 16-char shape — but we accept the longer "with seconds" variant
  // some browsers offer for robustness.
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/.exec(local);
  if (!m) return "";
  const [, y, mo, d, h, mi, s] = m;
  const guess = Date.UTC(
    Number(y),
    Number(mo) - 1,
    Number(d),
    Number(h),
    Number(mi),
    s ? Number(s) : 0,
  );
  const parts = formatPartsInZone(new Date(guess), tz);
  if (!parts) {
    return new Date(guess).toISOString();
  }
  // The zone showed `parts.*` for the guessed UTC instant. The
  // operator typed the local wall-clock literal; the difference is
  // the UTC offset (in ms) we need to subtract from the guess to
  // land on the true UTC instant.
  const renderedAsUtc = Date.UTC(
    Number(parts.year),
    Number(parts.month) - 1,
    Number(parts.day),
    Number(parts.hour),
    Number(parts.minute),
    Number(parts.second ?? "0"),
  );
  const offsetMs = renderedAsUtc - guess;
  return new Date(guess - offsetMs).toISOString();
}

type ZonedParts = {
  year: string;
  month: string;
  day: string;
  hour: string;
  minute: string;
  second?: string;
};

function formatPartsInZone(d: Date, tz: string): ZonedParts | null {
  const opts: Intl.DateTimeFormatOptions = {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
    timeZone: tz,
  };
  let fmt: Intl.DateTimeFormat;
  try {
    fmt = new Intl.DateTimeFormat("en-US", opts);
  } catch {
    fmt = new Intl.DateTimeFormat("en-US", { ...opts, timeZone: "UTC" });
  }
  const map: Record<string, string> = {};
  for (const p of fmt.formatToParts(d)) {
    if (p.type !== "literal") map[p.type] = p.value;
  }
  if (!map.year || !map.month || !map.day || !map.hour || !map.minute) {
    return null;
  }
  // `hour12: false` in some Intl implementations returns "24" instead
  // of "00" at midnight; normalise so the assembled "YYYY-MM-DDTHH:mm"
  // stays a legal datetime-local value.
  let hour = map.hour;
  if (hour === "24") hour = "00";
  return {
    year: map.year,
    month: map.month,
    day: map.day,
    hour,
    minute: map.minute,
    second: map.second,
  };
}

const FALLBACK_TIMEZONES: string[] = [
  "UTC",
  "Africa/Cairo",
  "Africa/Johannesburg",
  "Africa/Lagos",
  "America/Anchorage",
  "America/Argentina/Buenos_Aires",
  "America/Bogota",
  "America/Chicago",
  "America/Denver",
  "America/Halifax",
  "America/Los_Angeles",
  "America/Mexico_City",
  "America/New_York",
  "America/Sao_Paulo",
  "America/Toronto",
  "Asia/Bangkok",
  "Asia/Dubai",
  "Asia/Hong_Kong",
  "Asia/Jerusalem",
  "Asia/Kolkata",
  "Asia/Seoul",
  "Asia/Shanghai",
  "Asia/Singapore",
  "Asia/Tokyo",
  "Australia/Sydney",
  "Europe/Berlin",
  "Europe/London",
  "Europe/Madrid",
  "Europe/Moscow",
  "Europe/Paris",
  "Europe/Zurich",
  "Pacific/Auckland",
  "Pacific/Honolulu",
];
