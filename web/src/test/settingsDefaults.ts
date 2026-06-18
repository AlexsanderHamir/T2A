import type { AppSettings } from "@/api/settings";
import { DEFAULT_VERIFY_MAX_RETRIES } from "@/types/task";

/**
 * Full AppSettings fixture with all required fields. Tests override
 * individual fields via the spread operator.
 */
export const APP_SETTINGS_DEFAULTS: AppSettings = {
  agent_paused: false,
  runner: "cursor",
  repo_root: "",
  cursor_bin: "",
  cursor_model: "",
  max_run_duration_seconds: 0,
  agent_pickup_delay_seconds: 5,
  display_timezone: "UTC",
  optimistic_mutations_enabled: true,
  sse_replay_enabled: false,
  verify_max_retries: DEFAULT_VERIFY_MAX_RETRIES,
  verify_runner_name: "",
  verify_runner_model: "",
  verify_command_timeout_seconds: 120,
};
