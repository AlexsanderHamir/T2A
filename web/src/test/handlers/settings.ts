import { http, HttpResponse } from "msw";
import type { AppSettings } from "@/api/settings";
import { APP_SETTINGS_DEFAULTS } from "@/test/settingsDefaults";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";

export function appSettingsOk(overrides: Partial<AppSettings> = {}) {
  return http.get("/settings", () =>
    HttpResponse.json({ ...APP_SETTINGS_DEFAULTS, ...overrides }),
  );
}

export function listCursorModelsOk() {
  return http.post("/settings/list-cursor-models", () =>
    HttpResponse.json({
      ok: true,
      runner: TASK_TEST_DEFAULTS.runner,
      models: [{ id: "test", label: "Test" }],
    }),
  );
}
