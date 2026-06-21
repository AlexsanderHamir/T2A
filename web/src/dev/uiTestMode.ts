const SESSION_KEY = "hamix_ui_test_mode";

function envUiTestMode(): boolean {
  const v = import.meta.env.VITE_UI_TEST_MODE;
  return v === "true" || v === "1";
}

/**
 * When true, `fetchWithTimeout` serves synthetic GET payloads for tasks and
 * projects so layouts can be reviewed without a populated database.
 * Enable via `VITE_UI_TEST_MODE=true` (see docs/configuration.md) or session toggle
 * on the Settings page (persists for the tab until cleared).
 */
export function isUiTestMode(): boolean {
  if (envUiTestMode()) return true;
  if (typeof window === "undefined") return false;
  try {
    return window.sessionStorage.getItem(SESSION_KEY) === "1";
  } catch {
    return false;
  }
}

export function getUiTestModeSessionEnabled(): boolean {
  if (typeof window === "undefined") return false;
  try {
    return window.sessionStorage.getItem(SESSION_KEY) === "1";
  } catch {
    return false;
  }
}

export function setUiTestModeSessionEnabled(enabled: boolean): void {
  if (typeof window === "undefined") return;
  try {
    if (enabled) window.sessionStorage.setItem(SESSION_KEY, "1");
    else window.sessionStorage.removeItem(SESSION_KEY);
  } catch {
    /* ignore quota / private mode */
  }
}
