import { useMemo, useState } from "react";
import {
  getUiTestModeSessionEnabled,
  isUiTestMode,
  setUiTestModeSessionEnabled,
} from "@/dev/uiTestMode";

export function UiTestModeSettingsSection() {
  const envForced = useMemo(() => {
    const v = import.meta.env.VITE_UI_TEST_MODE;
    return v === "true" || v === "1";
  }, []);
  const [sessionOn, setSessionOn] = useState(() => getUiTestModeSessionEnabled());
  const active = isUiTestMode();

  return (
    <fieldset className="settings-fieldset">
      <legend className="settings-fieldset-legend">UI test mode</legend>
      <p className="settings-section-subtitle">
        Load synthetic tasks, projects, goals, steps, and context for layout review. Matching GET requests are
        answered locally; saves and worker probes still use the server.
      </p>

      {envForced ? (
        <p className="settings-field-help" role="status">
          Enabled via <code>VITE_UI_TEST_MODE</code> in the web build. Remove it and rebuild to disable.
        </p>
      ) : (
        <label className="settings-field settings-field--inline">
          <input
            type="checkbox"
            checked={sessionOn}
            onChange={(e) => {
              const on = e.target.checked;
              setSessionOn(on);
              setUiTestModeSessionEnabled(on);
              window.setTimeout(() => {
                window.location.reload();
              }, 0);
            }}
          />
          <span className="settings-field-label">Use demo data for tasks and projects (this tab only)</span>
        </label>
      )}

      {active ? (
        <p className="settings-field-help" role="status">
          UI test mode is on — a banner appears under the app header.
        </p>
      ) : null}

      {!envForced ? (
        <p className="settings-field-help">
          Optional: set <code>VITE_UI_TEST_MODE=true</code> in <code>web/.env.local</code> (see repo{" "}
          <code>.env.example</code>) so the mode defaults on for everyone hitting that dev server.
        </p>
      ) : null}
    </fieldset>
  );
}
