import { useMemo, useState } from "react";
import { getUiTestModeSessionEnabled, setUiTestModeSessionEnabled } from "@/dev/uiTestMode";
import { SECTION_IDS } from "./SettingsSections";

/**
 * Developer section. Visually identical to the other settings cards
 * (same `.settings-section` shape, same heading style) so the page
 * reads as one consistent vocabulary instead of "form sections + a
 * dev block at the bottom that looks different." Sits at the end of
 * the form because it's the only section that takes effect without
 * hitting Save.
 */
export function UiTestModeSettingsSection() {
  const envForced = useMemo(() => {
    const v = import.meta.env.VITE_UI_TEST_MODE;
    return v === "true" || v === "1";
  }, []);
  const [sessionOn, setSessionOn] = useState(() => getUiTestModeSessionEnabled());

  const titleId = `${SECTION_IDS.developer}-title`;
  return (
    <section
      id={SECTION_IDS.developer}
      className="settings-section"
      aria-labelledby={titleId}
    >
      <h2 id={titleId} className="settings-section-title">
        Developer
      </h2>
      <div className="settings-section-body">
        {envForced ? (
          <p className="settings-field-help" role="status">
            Sample data is on for this build via{" "}
            <code>VITE_UI_TEST_MODE</code>. Rebuild without it to turn off.
          </p>
        ) : (
          <>
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
              <span className="settings-field-label">
                Use sample data in this tab
              </span>
            </label>
            <p className="settings-field-help">
              Loads sample tasks and projects for layout review. A banner
              appears under the header while this is on; saves still hit the
              server. Optional team default:{" "}
              <code>VITE_UI_TEST_MODE=true</code> in{" "}
              <code>web/.env.local</code>.
            </p>
          </>
        )}
      </div>
    </section>
  );
}
