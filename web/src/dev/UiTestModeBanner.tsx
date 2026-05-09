import { isUiTestMode } from "./uiTestMode";

function envLocked(): boolean {
  const v = import.meta.env.VITE_UI_TEST_MODE;
  return v === "true" || v === "1";
}

export function UiTestModeBanner() {
  if (!isUiTestMode()) return null;
  const locked = envLocked();
  return (
    <div className="app-ui-test-banner" role="status" aria-live="polite">
      <span className="app-ui-test-banner__text">
        UI test mode: tasks and projects load from built-in demo data. Mutations and settings still call the
        real API when reachable.
        {locked ? " (locked on via VITE_UI_TEST_MODE.)" : ""}
      </span>
    </div>
  );
}
