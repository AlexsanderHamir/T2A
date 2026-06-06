/**
 * Aggregate cold-start payload from `GET /v1/bootstrap`.
 *
 * The endpoint is an *optimization hint*: the server composes the five
 * existing reads (settings, root tasks page, stats, projects page,
 * drafts head) into one round trip so the SPA can seed its TanStack
 * Query cache without fanning out. Clients MUST tolerate the endpoint
 * being absent (older servers, stripped builds) and fall back to the
 * per-endpoint hooks unchanged.
 *
 * Wire-format guarantees:
 * - `settings` mirrors `GET /settings` exactly.
 * - `tasks` mirrors the `GET /tasks?limit=20&offset=0` envelope.
 * - `stats` mirrors `GET /tasks/stats`.
 * - `projects` mirrors `GET /projects?limit=100`.
 * - `drafts` mirrors `GET /task-drafts?limit=50` (i.e. `{ drafts: [...] }`).
 */
import { fetchWithTimeout, apiErrorFromResponse } from "./shared";
import { parseProjectListResponse } from "./projects";
import {
  parseTaskListResponse,
  parseTaskStatsResponse,
} from "./parseTaskApi";
import { parseTaskDraftSummaryList } from "./parseTaskApi";
import type { AppSettings } from "./settings";
import type {
  TaskListResponse,
  TaskStatsResponse,
} from "@/types/task";
import type { ProjectListResponse } from "@/types/project";
import type { TaskDraftSummary } from "@/types/task";

export type Bootstrap = {
  settings: AppSettings;
  tasks: TaskListResponse;
  stats: TaskStatsResponse;
  projects: ProjectListResponse;
  drafts: TaskDraftSummary[];
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

// Local settings parser. We duplicate the structure here rather than
// exporting assertSettings from settings.ts because (a) bootstrap is the
// only secondary consumer today and (b) duplicating the assertions keeps
// the boundary explicit when the wire shape evolves — the call site is
// where contract drift surfaces fastest.
function parseSettingsField(raw: unknown): AppSettings {
  if (!isRecord(raw)) {
    throw new Error("Invalid API response: bootstrap.settings must be object");
  }
  const o = raw;
  const paused = typeof o.agent_paused === "boolean" ? o.agent_paused : false;
  const runner = o.runner;
  const repoRoot = o.repo_root;
  const cursorBin = o.cursor_bin;
  const cursorModel = o.cursor_model;
  const maxDur = o.max_run_duration_seconds;
  const pickupDelay = o.agent_pickup_delay_seconds;
  const tz = typeof o.display_timezone === "string" ? o.display_timezone : "";
  const optimistic = typeof o.optimistic_mutations_enabled === "boolean"
    ? o.optimistic_mutations_enabled
    : true;
  const sseReplay = typeof o.sse_replay_enabled === "boolean"
    ? o.sse_replay_enabled
    : true;
  const verifyMaxRetries =
    typeof o.verify_max_retries === "number" ? o.verify_max_retries : 1;
  const verifyRunnerName =
    typeof o.verify_runner_name === "string" ? o.verify_runner_name : "";
  const verifyRunnerModel =
    typeof o.verify_runner_model === "string" ? o.verify_runner_model : "";
  const checkTimeout =
    typeof o.check_command_timeout_seconds === "number"
      ? o.check_command_timeout_seconds
      : 0;
  if (
    typeof runner !== "string" ||
    typeof repoRoot !== "string" ||
    typeof cursorBin !== "string" ||
    typeof cursorModel !== "string" ||
    typeof maxDur !== "number" ||
    typeof pickupDelay !== "number"
  ) {
    throw new Error("Invalid API response: bootstrap.settings shape");
  }
  const out: AppSettings = {
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
    verify_max_retries: verifyMaxRetries,
    verify_runner_name: verifyRunnerName,
    verify_runner_model: verifyRunnerModel,
    check_command_timeout_seconds: checkTimeout,
  };
  if (typeof o.updated_at === "string") {
    out.updated_at = o.updated_at;
  }
  return out;
}

function parseBootstrap(value: unknown): Bootstrap {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: bootstrap must be an object");
  }
  return {
    settings: parseSettingsField(value.settings),
    tasks: parseTaskListResponse(value.tasks),
    stats: parseTaskStatsResponse(value.stats),
    projects: parseProjectListResponse(value.projects),
    drafts: parseTaskDraftSummaryList(value.drafts),
  };
}

/**
 * Returns `null` when the endpoint is unavailable (404 or 405 from a
 * server that has not been updated). Network and 5xx errors still
 * throw so callers can surface them — the bootstrap hook's contract is
 * "fast path when present, transparent fallback when missing".
 */
export async function fetchBootstrap(
  options?: { signal?: AbortSignal },
): Promise<Bootstrap | null> {
  const res = await fetchWithTimeout("/v1/bootstrap", {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (res.status === 404 || res.status === 405) {
    return null;
  }
  if (!res.ok) {
    throw await apiErrorFromResponse(res);
  }
  const raw: unknown = await res.json();
  return parseBootstrap(raw);
}
