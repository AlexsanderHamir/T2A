import { useAppSettings } from "@/settings/useAppSettings";

/**
 * Default IANA timezone used by the SPA when settings haven't loaded
 * yet OR the server explicitly returned "UTC". Kept in a constant so
 * tests and callers can compare without hardcoding the literal in N
 * places.
 *
 * MUST stay in sync with pkgs/tasks/domain/app_settings.go's
 * `DefaultDisplayTimezone`.
 */
export const DEFAULT_APP_TIMEZONE = "UTC";

/**
 * useAppTimezone returns the operator-chosen IANA timezone identifier
 * (e.g. "America/New_York", "Europe/London"). Falls back to "UTC"
 * while the settings query is loading or if the server omits the
 * field — the fallback is deliberately the same default the backend
 * seeds, so a freshly-installed SPA against an old backend renders
 * exactly the same as a freshly-installed SPA against a new backend
 * with default settings.
 *
 * Stage 1 of the task scheduling plan introduces the field; later
 * stages (3–5) call this hook from every operator-facing timestamp
 * render so a single PATCH /settings { display_timezone } re-renders
 * the whole SPA in the chosen zone via React Query invalidation.
 */
export function useAppTimezone(): string {
  const { settings } = useAppSettings();
  if (!settings) return DEFAULT_APP_TIMEZONE;
  const tz = settings.display_timezone;
  if (typeof tz !== "string" || tz.length === 0) {
    return DEFAULT_APP_TIMEZONE;
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
