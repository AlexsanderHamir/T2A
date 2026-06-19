import type { AppSettings, AppSettingsPatch } from "@/api/settings";

/**
 * Registered runners.
 *
 * `label` is the long form rendered inside <select> options ("Cursor
 * (cursor-agent CLI)"); `shortLabel` is a one-word display name
 * (e.g. "Cursor") for places that need to refer to the runner
 * inline without the parenthetical detail.
 */
export const RUNNERS = [
  { id: "cursor", label: "Cursor (cursor-agent CLI)", shortLabel: "Cursor" },
] as const;

/**
 * Resolve a runner id (typically `form.runner`) to its short display
 * name. Falls back to the raw runner id, then to a generic "Runner"
 * so callers never display an empty string.
 */
export function runnerShortLabel(runnerId: string): string {
  const trimmed = runnerId.trim();
  const entry = RUNNERS.find((r) => r.id === trimmed);
  if (entry) return entry.shortLabel;
  return trimmed || "Runner";
}

export type SettingsFormState = {
  runner: string;
  repoRoot: string;
  cursorBin: string;
  cursorModel: string;
  maxRunDurationSeconds: string;
  streamIdleStuckSeconds: string;
  agentPickupDelaySeconds: string;
  displayTimezone: string;
  verifyMaxRetries: string;
  verifyRunnerName: string;
  verifyRunnerModel: string;
};

export type SettingsStatus =
  | { kind: "success"; message: string }
  | { kind: "error"; message: string }
  | null;

/** Matches `AUTO_DISMISS_MS` in `@/shared/toast/ToastProvider` — ephemeral success feedback. */
export const SETTINGS_SUCCESS_DISMISS_MS = 4_000;

export function toFormState(s: AppSettings): SettingsFormState {
  return {
    runner: s.runner,
    repoRoot: s.repo_root,
    cursorBin: s.cursor_bin,
    cursorModel: s.cursor_model,
    maxRunDurationSeconds: String(s.max_run_duration_seconds),
    streamIdleStuckSeconds: String(s.stream_idle_stuck_seconds),
    agentPickupDelaySeconds: String(s.agent_pickup_delay_seconds),
    displayTimezone: s.display_timezone,
    verifyMaxRetries: String(s.verify_max_retries),
    verifyRunnerName: s.verify_runner_name,
    verifyRunnerModel: s.verify_runner_model,
  };
}

export function diffPatch(
  initial: AppSettings,
  form: SettingsFormState,
): AppSettingsPatch {
  const out: AppSettingsPatch = {};
  if (initial.runner !== form.runner.trim()) {
    out.runner = form.runner.trim();
  }
  if (initial.repo_root !== form.repoRoot.trim()) {
    out.repo_root = form.repoRoot.trim();
  }
  if (initial.cursor_bin !== form.cursorBin.trim()) {
    out.cursor_bin = form.cursorBin.trim();
  }
  if (initial.cursor_model !== form.cursorModel.trim()) {
    out.cursor_model = form.cursorModel.trim();
  }
  const parsedMax = Number.parseInt(form.maxRunDurationSeconds.trim() || "0", 10);
  if (Number.isFinite(parsedMax) && parsedMax !== initial.max_run_duration_seconds) {
    out.max_run_duration_seconds = parsedMax;
  }
  const parsedStreamIdle = Number.parseInt(
    form.streamIdleStuckSeconds.trim() || "0",
    10,
  );
  if (
    Number.isFinite(parsedStreamIdle) &&
    parsedStreamIdle !== initial.stream_idle_stuck_seconds
  ) {
    out.stream_idle_stuck_seconds = parsedStreamIdle;
  }
  const parsedPickup = Number.parseInt(
    form.agentPickupDelaySeconds.trim() || "0",
    10,
  );
  if (
    Number.isFinite(parsedPickup) &&
    parsedPickup !== initial.agent_pickup_delay_seconds
  ) {
    out.agent_pickup_delay_seconds = parsedPickup;
  }
  const tzTrimmed = form.displayTimezone.trim();
  if (tzTrimmed !== initial.display_timezone) {
    out.display_timezone = tzTrimmed;
  }
  const parsedRetries = Number.parseInt(form.verifyMaxRetries.trim() || "0", 10);
  if (
    Number.isFinite(parsedRetries) &&
    parsedRetries !== initial.verify_max_retries
  ) {
    out.verify_max_retries = parsedRetries;
  }
  const verifyName = form.verifyRunnerName.trim();
  if (verifyName !== initial.verify_runner_name) {
    out.verify_runner_name = verifyName;
  }
  const verifyModel = form.verifyRunnerModel.trim();
  if (verifyModel !== initial.verify_runner_model) {
    out.verify_runner_model = verifyModel;
  }
  return out;
}
