import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import type { SettingsStatus } from "../settingsForm";

export function SettingsStatusMessage({ status }: { status: SettingsStatus }) {
  if (status?.kind === "success") {
    return (
      <p role="status" data-testid="settings-status" className="settings-status">
        <svg
          className="settings-status-icon"
          viewBox="0 0 20 20"
          fill="none"
          aria-hidden="true"
        >
          <circle cx="10" cy="10" r="8.25" stroke="currentColor" strokeWidth="1.5" />
          <path
            d="m6 10.25 2.75 2.75L14 7.75"
            stroke="currentColor"
            strokeWidth="1.7"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        <span>{status.message}</span>
      </p>
    );
  }
  if (status?.kind === "error") {
    return (
      <div data-testid="settings-status-error">
        <MutationErrorBanner
          error={status.message}
          className="settings-status-err"
        />
      </div>
    );
  }
  return null;
}
