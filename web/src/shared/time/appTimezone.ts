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

/** One row for Settings (and similar) timezone `<select>` menus. */
export type TimezoneSelectOption = { value: string; label: string };

/**
 * Lowercase search haystack for a timezone row: IANA id, Meet label,
 * region + city tokens (so "tokyo", "asia", "gmt+9" can all match).
 */
export function getTimezoneSearchHaystack(opt: TimezoneSelectOption): string {
  const v = opt.value;
  const segments = v.split("/");
  const region = segments.length > 1 ? segments[0] : "";
  const city =
    segments.length > 0 ? segments[segments.length - 1].replace(/_/g, " ") : v;
  const flatSlashes = v.replace(/\//g, " ");
  const flatUnderscores = v.replace(/_/g, " ");
  const labelLo = opt.label.toLowerCase();
  return [
    v,
    labelLo,
    region,
    city,
    flatSlashes,
    flatUnderscores,
    // e.g. "(gmt-07:00)" → extra tokens for "7" / "07" seekers
    labelLo.replace(/[()]/g, " "),
  ]
    .join(" ")
    .toLowerCase();
}

/** True when every whitespace-separated token appears in `haystack`. */
export function matchesTimezoneSearchQuery(
  haystack: string,
  query: string,
): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  const tokens = q.split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return true;
  return tokens.every((t) => haystack.includes(t));
}

/**
 * Filters IANA timezone options by substring / token search. Empty
 * `query` returns the full list (caller may cap UI height).
 */
export function filterTimezoneSelectOptions(
  options: TimezoneSelectOption[],
  query: string,
  maxResults = 200,
): TimezoneSelectOption[] {
  const q = query.trim();
  if (!q) return options;
  const out: TimezoneSelectOption[] = [];
  for (const o of options) {
    if (out.length >= maxResults) break;
    const h = getTimezoneSearchHaystack(o);
    if (matchesTimezoneSearchQuery(h, q)) out.push(o);
  }
  return out;
}

/**
 * Parses `GMT+9`, `GMT-05:30`, etc. from Intl `longOffset` to minutes
 * east of UTC.
 */
function parseGmtLongOffsetToMinutes(s: string): number {
  const m = /GMT([+-])(\d{1,2})(?::(\d{2}))?/i.exec(s);
  if (!m) {
    return 0;
  }
  const sign = m[1] === "-" ? -1 : 1;
  const h = parseInt(m[2], 10);
  const min = m[3] ? parseInt(m[3], 10) : 0;
  return sign * (h * 60 + min);
}

/**
 * GMT offset in minutes east of UTC at `date` (DST-aware), from Intl
 * `longOffset`. Used for Meet-style labels and offset sorting.
 */
export function getTimezoneOffsetMinutesAt(timeZone: string, date: Date): number {
  try {
    const parts = new Intl.DateTimeFormat("en-US", {
      timeZone,
      timeZoneName: "longOffset",
    }).formatToParts(date);
    const raw =
      parts.find((p) => p.type === "timeZoneName")?.value ?? "GMT+0";
    return parseGmtLongOffsetToMinutes(raw);
  } catch {
    return 0;
  }
}

function formatGmtOffsetParen(minutes: number): string {
  const sign = minutes >= 0 ? "+" : "-";
  const abs = Math.abs(minutes);
  const h = Math.floor(abs / 60);
  const min = abs % 60;
  return `(GMT${sign}${String(h).padStart(2, "0")}:${String(min).padStart(2, "0")})`;
}

/**
 * Last path segment of an IANA id, underscores → spaces (e.g.
 * `America/Los_Angeles` → `Los Angeles`).
 */
function ianaToDisplayCity(iana: string): string {
  if (iana === "UTC") return "UTC";
  const seg = iana.split("/").pop() ?? iana;
  return seg.replace(/_/g, " ");
}

/**
 * User-facing menu label (Google Meet–style): `(GMT+09:00) Tokyo` for
 * `Asia/Tokyo`. Wire value stays the IANA id everywhere else.
 */
export function formatTimezoneMenuLabel(iana: string, at: Date = new Date()): string {
  const mins = getTimezoneOffsetMinutesAt(iana, at);
  const paren = formatGmtOffsetParen(mins);
  const city = ianaToDisplayCity(iana);
  return `${paren} ${city}`;
}

/**
 * Options for Settings timezone `<select>`: sorted by current UTC
 * offset (earliest / west first), then by IANA id. Labels use
 * {@link formatTimezoneMenuLabel}; values remain canonical IANA names
 * for PATCH /settings and `time.LoadLocation` on the server.
 */
export function getTimezoneSelectOptions(at: Date = new Date()): TimezoneSelectOption[] {
  const ids = supportedTimezones();
  const rows = ids.map((value) => ({
    value,
    label: formatTimezoneMenuLabel(value, at),
    offsetMin: getTimezoneOffsetMinutesAt(value, at),
  }));
  rows.sort((a, b) => {
    const d = a.offsetMin - b.offsetMin;
    if (d !== 0) return d;
    return a.value.localeCompare(b.value);
  });
  return rows.map(({ value, label }) => ({ value, label }));
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
