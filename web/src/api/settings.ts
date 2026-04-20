import { fetchWithTimeout, jsonHeaders, readError } from "./shared";

/**
 * On-the-wire shape returned by GET /settings and PATCH /settings.
 * Matches pkgs/tasks/handler/handler_settings.go:settingsResponse
 * one-for-one. updated_at is RFC3339 (omitempty when the row was
 * never written, but in practice the first GET seeds defaults so it
 * is always populated by the time the SPA sees the response).
 */
export type AppSettings = {
  worker_enabled: boolean;
  /**
   * Operator-facing soft pause. Distinct from worker_enabled in intent:
   * worker_enabled is the "configured to run at all" master switch
   * (defaults true), agent_paused is the one-click "stop dequeuing for
   * a few minutes" toggle exposed in the SPA header chip (defaults
   * false). Both keep the supervisor idle, but they surface as
   * different idle reasons in the supervisor logs and as different
   * pane states on /observability.
   */
  agent_paused: boolean;
  runner: string;
  repo_root: string;
  cursor_bin: string;
  /** Empty string = Cursor default model (`cursor-agent` omits `--model`). */
  cursor_model: string;
  max_run_duration_seconds: number;
  /** Minimum seconds before the worker runs a new ready task. Default 5; 0 = no wait. */
  agent_pickup_delay_seconds: number;
  /**
   * IANA timezone identifier (e.g. "America/New_York"). Used for
   * EVERY operator-facing timestamp render in the SPA. The wire
   * format for every timestamp stays RFC3339 UTC; this field
   * governs PRESENTATION only. Empty string ("") is the "auto-detect"
   * sentinel — the backend seeds it on first boot so the SPA falls
   * back to the operator's browser timezone via
   * `Intl.DateTimeFormat().resolvedOptions().timeZone`. Any non-empty
   * value is an explicit override validated server-side via
   * time.LoadLocation; the SPA can trust it parses in
   * Intl.DateTimeFormat without further checking.
   */
  display_timezone: string;
  /**
   * Stored for API compatibility; optimistic mutations are always on.
   * Not user-configurable in Settings.
   */
  optimistic_mutations_enabled: boolean;
  /**
   * Stored for API compatibility; lossless SSE replay is always on server-side.
   */
  sse_replay_enabled: boolean;
  updated_at?: string;
};

/**
 * Partial-update body for PATCH /settings. Pointer-typed fields on the
 * Go side: omit a field to leave it unchanged, send an explicit value
 * (including zero) to overwrite. max_run_duration_seconds = 0 means
 * "no limit"; cursor_bin = "" means "auto-detect via PATH at boot".
 */
export type AppSettingsPatch = Partial<{
  worker_enabled: boolean;
  agent_paused: boolean;
  runner: string;
  repo_root: string;
  cursor_bin: string;
  cursor_model: string;
  max_run_duration_seconds: number;
  agent_pickup_delay_seconds: number;
  /**
   * IANA timezone identifier (e.g. "America/New_York"). Empty string
   * clears the override so the SPA falls back to browser auto-detect
   * (see AppSettings.display_timezone for the auto-detect sentinel
   * contract). Non-empty values are validated server-side via
   * time.LoadLocation.
   */
  display_timezone: string;
  optimistic_mutations_enabled: boolean;
  sse_replay_enabled: boolean;
}>;

export type ProbeCursorResult = {
  ok: boolean;
  runner: string;
  /**
   * Absolute binary path that the server actually executed. Populated
   * regardless of ok, so the SPA can show "auto-detected at
   * /usr/local/bin/cursor-agent" on success or "tried /usr/local/bin/cursor
   * — exec failed" on failure. Empty when the server could not resolve
   * the runner at all (e.g. unknown runner id).
   */
  binary_path?: string;
  version?: string;
  error?: string;
};

export type CancelCurrentRunResult = {
  cancelled: boolean;
};

/** Response from POST /settings/list-cursor-models. */
export type ListCursorModelsResult = {
  ok: boolean;
  runner: string;
  binary_path?: string;
  models?: Array<{ id: string; label: string }>;
  error?: string;
};

function assertSettings(raw: unknown): AppSettings {
  if (raw === null || typeof raw !== "object") {
    throw new Error("unexpected settings response shape");
  }
  const o = raw as Record<string, unknown>;
  const worker = o.worker_enabled;
  // Default agent_paused to false when the server omits the field so
  // older builds (pre-4a) stay decodable by a freshly-deployed SPA.
  // We could throw instead, but a boolean default that matches the DB
  // default is safer than blocking the whole settings page on a
  // missing key.
  const paused = typeof o.agent_paused === "boolean" ? o.agent_paused : false;
  const runner = o.runner;
  const repoRoot = o.repo_root;
  const cursorBin = o.cursor_bin;
  const cursorModel = o.cursor_model;
  const maxDur = o.max_run_duration_seconds;
  const pickupDelay = o.agent_pickup_delay_seconds;
  // display_timezone is preserved verbatim when the server sends a
  // string. Empty string ("") is the documented auto-detect sentinel
  // (the SPA reads it as "no operator override, use the browser zone"),
  // so we MUST NOT coerce "" to "UTC" here — that would silently
  // override the auto-detect path. When the server omits the field
  // entirely (stale pre-Stage-1 binary still serving GETs) we fall
  // back to "" too, which routes through the same auto-detect path —
  // safer than hard-coding UTC for every operator on a new SPA build.
  const tz = typeof o.display_timezone === "string" ? o.display_timezone : "";
  // Rollout flags default to true when omitted (legacy responses).
  const optimistic = typeof o.optimistic_mutations_enabled === "boolean"
    ? o.optimistic_mutations_enabled
    : true;
  const sseReplay = typeof o.sse_replay_enabled === "boolean"
    ? o.sse_replay_enabled
    : true;
  if (
    typeof worker !== "boolean" ||
    typeof runner !== "string" ||
    typeof repoRoot !== "string" ||
    typeof cursorBin !== "string" ||
    typeof cursorModel !== "string" ||
    typeof maxDur !== "number" ||
    typeof pickupDelay !== "number"
  ) {
    throw new Error("unexpected settings response shape");
  }
  const out: AppSettings = {
    worker_enabled: worker,
    agent_paused: paused,
    runner,
    repo_root: repoRoot,
    cursor_bin: cursorBin,
    cursor_model: cursorModel,
    max_run_duration_seconds: maxDur,
    agent_pickup_delay_seconds: pickupDelay,
    display_timezone: tz,
    optimistic_mutations_enabled: optimistic,
    sse_replay_enabled: sseReplay,
  };
  if (typeof o.updated_at === "string") {
    out.updated_at = o.updated_at;
  }
  return out;
}

export async function listCursorModels(
  body: { runner?: string; binary_path?: string },
  options?: { signal?: AbortSignal },
): Promise<ListCursorModelsResult> {
  const res = await fetchWithTimeout("/settings/list-cursor-models", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(body),
    signal: options?.signal,
  });
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  if (raw === null || typeof raw !== "object") {
    throw new Error("unexpected list-cursor-models response");
  }
  const o = raw as Record<string, unknown>;
  if (typeof o.ok !== "boolean" || typeof o.runner !== "string") {
    throw new Error("unexpected list-cursor-models response shape");
  }
  const out: ListCursorModelsResult = { ok: o.ok, runner: o.runner };
  if (typeof o.binary_path === "string") out.binary_path = o.binary_path;
  if (typeof o.error === "string") out.error = o.error;
  if (Array.isArray(o.models)) {
    out.models = o.models.map((m) => {
      if (m === null || typeof m !== "object") {
        throw new Error("unexpected model entry");
      }
      const e = m as Record<string, unknown>;
      if (typeof e.id !== "string" || typeof e.label !== "string") {
        throw new Error("unexpected model entry shape");
      }
      return { id: e.id, label: e.label };
    });
  }
  return out;
}

export async function fetchAppSettings(
  options?: { signal?: AbortSignal },
): Promise<AppSettings> {
  const res = await fetchWithTimeout(
    "/settings",
    {
      headers: { Accept: "application/json" },
      signal: options?.signal,
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  return assertSettings(await res.json());
}

export async function patchAppSettings(
  patch: AppSettingsPatch,
  options?: { signal?: AbortSignal },
): Promise<AppSettings> {
  const res = await fetchWithTimeout(
    "/settings",
    {
      method: "PATCH",
      headers: jsonHeaders,
      body: JSON.stringify(patch),
      signal: options?.signal,
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  return assertSettings(await res.json());
}

export async function probeCursor(
  body: { runner?: string; binary_path?: string },
  options?: { signal?: AbortSignal },
): Promise<ProbeCursorResult> {
  const res = await fetchWithTimeout(
    "/settings/probe-cursor",
    {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify(body),
      signal: options?.signal,
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  if (raw === null || typeof raw !== "object") {
    throw new Error("unexpected probe-cursor response");
  }
  const o = raw as Record<string, unknown>;
  if (typeof o.ok !== "boolean" || typeof o.runner !== "string") {
    throw new Error("unexpected probe-cursor response shape");
  }
  const out: ProbeCursorResult = { ok: o.ok, runner: o.runner };
  if (typeof o.binary_path === "string") out.binary_path = o.binary_path;
  if (typeof o.version === "string") out.version = o.version;
  if (typeof o.error === "string") out.error = o.error;
  return out;
}

export async function cancelCurrentRun(
  options?: { signal?: AbortSignal },
): Promise<CancelCurrentRunResult> {
  const res = await fetchWithTimeout(
    "/settings/cancel-current-run",
    {
      method: "POST",
      headers: jsonHeaders,
      signal: options?.signal,
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  if (raw === null || typeof raw !== "object") {
    throw new Error("unexpected cancel-current-run response");
  }
  const o = raw as Record<string, unknown>;
  if (typeof o.cancelled !== "boolean") {
    throw new Error("unexpected cancel-current-run response shape");
  }
  return { cancelled: o.cancelled };
}
