import type { AppSettings, AppSettingsPatch } from "@/api/settings";

export const RUNNERS = [{ id: "cursor", label: "Cursor (cursor-agent CLI)" }] as const;

export type SettingsFormState = {
  workerEnabled: boolean;
  runner: string;
  repoRoot: string;
  cursorBin: string;
  cursorModel: string;
  maxRunDurationSeconds: string;
  agentPickupDelaySeconds: string;
  displayTimezone: string;
};

export type SettingsStatus =
  | { kind: "success"; message: string }
  | { kind: "error"; message: string }
  | null;

/** Matches `AUTO_DISMISS_MS` in `@/shared/toast/ToastProvider` — ephemeral success feedback. */
export const SETTINGS_SUCCESS_DISMISS_MS = 4_000;

export function toFormState(s: AppSettings): SettingsFormState {
  return {
    workerEnabled: s.worker_enabled,
    runner: s.runner,
    repoRoot: s.repo_root,
    cursorBin: s.cursor_bin,
    cursorModel: s.cursor_model,
    maxRunDurationSeconds: String(s.max_run_duration_seconds),
    agentPickupDelaySeconds: String(s.agent_pickup_delay_seconds),
    displayTimezone: s.display_timezone,
  };
}

export function diffPatch(
  initial: AppSettings,
  form: SettingsFormState,
): AppSettingsPatch {
  const out: AppSettingsPatch = {};
  if (initial.worker_enabled !== form.workerEnabled) {
    out.worker_enabled = form.workerEnabled;
  }
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
  return out;
}
