import { useMemo, useState } from "react";
import { getUiTestModeSessionEnabled, setUiTestModeSessionEnabled } from "@/dev/uiTestMode";

export function UiTestModeSettingsSection() {
  const envForced = useMemo(() => {
    const v = import.meta.env.VITE_UI_TEST_MODE;
    return v === "true" || v === "1";
  }, []);
  const [sessionOn, setSessionOn] = useState(() => getUiTestModeSessionEnabled());

  return (
    <section className="settings-ui-test-mode" aria-labelledby="settings-ui-test-mode-title">
      <h3 id="settings-ui-test-mode-title" className="settings-ui-test-mode__title">
        UI test mode
      </h3>
      <p className="settings-ui-test-mode__lede">
        Sample tasks and projects for layout review. A banner appears under the header while this is on; saves still
        use the server.
      </p>

      {envForced ? (
        <p className="settings-ui-test-mode__status" role="status">
          On for this build via <code>VITE_UI_TEST_MODE</code>. Rebuild without it to turn off.
        </p>
      ) : (
        <label className="settings-ui-test-mode__control">
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
          <span>Use sample data in this tab</span>
        </label>
      )}

      {!envForced ? (
        <p className="settings-ui-test-mode__footnote">
          Optional team default: <code>VITE_UI_TEST_MODE=true</code> in <code>web/.env.local</code> (see{" "}
          <code>.env.example</code>).
        </p>
      ) : null}
    </section>
  );
}
