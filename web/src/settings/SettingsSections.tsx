import type { UseQueryResult } from "@tanstack/react-query";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import type { ListCursorModelsResult } from "@/api/settings";
import type { TimezoneSelectOption } from "@/shared/time/appTimezone";
import { formatTimezoneMenuLabel } from "@/shared/time/appTimezone";
import { TimezoneCombobox } from "./TimezoneCombobox";
import {
  RUNNERS,
  type SettingsFormState,
  type SettingsStatus,
} from "./settingsForm";

type HandleField = <K extends keyof SettingsFormState>(
  key: K,
  value: SettingsFormState[K],
) => void;

export function SettingsLoadingState({
  error,
  onRetry,
}: {
  error: Error | null;
  onRetry: () => void;
}) {
  return (
    <section className="settings-page" aria-busy="true">
      <h2 className="settings-page-title term-arrow">
        <span>Settings</span>
      </h2>
      <p>{error ? `Error: ${error.message}` : "Loading settings…"}</p>
      {error ? (
        <button type="button" onClick={onRetry}>
          Retry
        </button>
      ) : null}
    </section>
  );
}

export function SettingsHeader({
  lastUpdated,
  lastUpdatedFormatted,
}: {
  lastUpdated: string;
  lastUpdatedFormatted: string;
}) {
  return (
    <header className="settings-page-header">
      <div className="settings-page-heading">
        <h2 className="settings-page-title term-arrow">
          <span>Settings</span>
        </h2>
        <p className="settings-page-subtitle">
          Runtime and workspace configuration for this installation.
        </p>
      </div>
      {lastUpdated ? (
        <span
          className="settings-page-saved-chip"
          data-testid="settings-last-updated"
        >
          <span className="settings-page-saved-chip-label">Last saved</span>
          <time className="settings-page-saved-chip-time" dateTime={lastUpdated}>
            {lastUpdatedFormatted || lastUpdated}
          </time>
        </span>
      ) : null}
    </header>
  );
}

export function WorkspaceWarning() {
  return (
    <div role="alert" className="settings-banner settings-banner--warn">
      <svg
        className="settings-banner-icon"
        viewBox="0 0 24 24"
        fill="none"
        aria-hidden="true"
      >
        <path
          d="M12 3.5 2.75 19.5h18.5L12 3.5Z"
          stroke="currentColor"
          strokeWidth="1.7"
          strokeLinejoin="round"
        />
        <path
          d="M12 10v4"
          stroke="currentColor"
          strokeWidth="1.7"
          strokeLinecap="round"
        />
        <circle cx="12" cy="17" r="1" fill="currentColor" />
      </svg>
      <div className="settings-banner-body">
        <p className="settings-banner-title">
          <strong>Workspace not configured.</strong>
        </p>
        <p className="settings-banner-text">
          Set the repository root below to enable the agent worker, file
          mentions, and the <code>/repo/*</code> endpoints.
        </p>
      </div>
    </div>
  );
}

export function AgentWorkerSettingsSection({
  form,
  pickupInvalid,
  onField,
}: {
  form: SettingsFormState;
  pickupInvalid: boolean;
  onField: HandleField;
}) {
  return (
    <fieldset className="settings-fieldset">
      <legend>Agent worker</legend>
      <p className="settings-section-subtitle">
        Pick up ready tasks and hand them to the configured runner.
      </p>

      <label className="settings-field settings-field--inline">
        <input
          type="checkbox"
          checked={form.workerEnabled}
          onChange={(e) => onField("workerEnabled", e.target.checked)}
        />
        <span className="settings-field-label">Enable agent worker</span>
      </label>
      <p className="settings-field-help">
        When on, the worker pulls ready tasks and dispatches them.
      </p>

      <label className="settings-field">
        <span className="settings-field-label">Runner</span>
        <select
          value={form.runner}
          onChange={(e) => onField("runner", e.target.value)}
        >
          {RUNNERS.map((r) => (
            <option key={r.id} value={r.id}>
              {r.label}
            </option>
          ))}
          {RUNNERS.find((r) => r.id === form.runner) ? null : (
            <option value={form.runner}>{form.runner} (custom)</option>
          )}
        </select>
      </label>

      <label className="settings-field">
        <span className="settings-field-label">Agent pickup delay (seconds)</span>
        <input
          type="number"
          min={0}
          max={604800}
          step={1}
          placeholder="5"
          value={form.agentPickupDelaySeconds}
          onChange={(e) => onField("agentPickupDelaySeconds", e.target.value)}
          aria-invalid={pickupInvalid}
        />
      </label>
      <p className="settings-field-help">
        Minimum wait before the next ready task runs. Default <code>5</code>s ·{" "}
        <code>0</code> = no wait.
      </p>
      {pickupInvalid ? (
        <p role="alert" className="settings-field-error">
          Must be between 0 and 604800 (7 days).
        </p>
      ) : null}
    </fieldset>
  );
}

export function DisplaySettingsSection({
  form,
  browserTz,
  options,
  showCustomTz,
  onField,
}: {
  form: SettingsFormState;
  browserTz: string;
  options: TimezoneSelectOption[];
  showCustomTz: boolean;
  onField: HandleField;
}) {
  const customValue = form.displayTimezone.trim();
  return (
    <fieldset className="settings-fieldset">
      <legend>Display</legend>
      <label className="settings-field">
        <span className="settings-field-label">Timezone</span>
        <TimezoneCombobox
          testId="settings-display-timezone-select"
          value={form.displayTimezone}
          onChange={(v) => onField("displayTimezone", v)}
          browserTz={browserTz}
          options={options}
          customSaved={
            showCustomTz
              ? {
                  value: customValue,
                  label: `${formatTimezoneMenuLabel(customValue)} (saved — not in list)`,
                }
              : null
          }
        />
      </label>
    </fieldset>
  );
}

export function WorkspaceSettingsSection({
  form,
  onField,
}: {
  form: SettingsFormState;
  onField: HandleField;
}) {
  return (
    <fieldset className="settings-fieldset">
      <legend>Workspace</legend>
      <p className="settings-section-subtitle">
        The repository the agent will execute tasks in.
      </p>
      <label className="settings-field">
        <span className="settings-field-label">Repository root (absolute path)</span>
        <input
          type="text"
          value={form.repoRoot}
          onChange={(e) => onField("repoRoot", e.target.value)}
          placeholder="/Users/me/code/my-project"
          spellCheck={false}
          autoComplete="off"
        />
      </label>
      <p className="settings-field-help">
        Leave empty to disable repo features until you pick a workspace.
      </p>
      <details className="settings-learn-more">
        <summary>What reads this path?</summary>
        <p>
          The agent worker, <code>/repo/*</code> endpoints, and{" "}
          <code>@file</code> mentions all resolve paths against this root.
        </p>
      </details>
    </fieldset>
  );
}

export function CursorAgentSettingsSection({
  form,
  cursorModelsQuery,
  modelIdsFromList,
  resolvedDefaultBin,
  probePending,
  onField,
  onProbe,
}: {
  form: SettingsFormState;
  cursorModelsQuery: UseQueryResult<ListCursorModelsResult, Error>;
  modelIdsFromList: Set<string>;
  resolvedDefaultBin: string | null;
  probePending: boolean;
  onField: HandleField;
  onProbe: () => void;
}) {
  return (
    <fieldset className="settings-fieldset" id="cursor-agent">
      <legend>Cursor agent (CLI)</legend>
      <p className="settings-section-subtitle">
        Model override and CLI binary used by the Cursor runner.
      </p>
      <label className="settings-field">
        <span className="settings-field-label">Model override</span>
        <select
          data-testid="settings-cursor-model-select"
          value={form.cursorModel}
          onChange={(e) => onField("cursorModel", e.target.value)}
          disabled={cursorModelsQuery.isFetching}
          aria-busy={cursorModelsQuery.isFetching}
        >
          <option value="">
            Default (omit --model; Cursor chooses for your account)
          </option>
          {cursorModelsQuery.data?.ok && cursorModelsQuery.data.models
            ? cursorModelsQuery.data.models.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.label}
                </option>
              ))
            : null}
          {form.cursorModel.trim() !== "" &&
          !modelIdsFromList.has(form.cursorModel.trim()) ? (
            <option value={form.cursorModel.trim()}>
              {form.cursorModel.trim()} (saved — not in current list)
            </option>
          ) : null}
        </select>
      </label>
      {cursorModelsQuery.isError ? (
        <p role="alert" className="settings-field-error">
          Could not load models from the Cursor CLI:{" "}
          {cursorModelsQuery.error instanceof Error
            ? cursorModelsQuery.error.message
            : String(cursorModelsQuery.error)}
        </p>
      ) : null}
      {cursorModelsQuery.data && !cursorModelsQuery.data.ok ? (
        <p role="alert" className="settings-field-error">
          {cursorModelsQuery.data.error ?? "Model list failed."}
        </p>
      ) : null}
      <p className="settings-field-help">
        List comes from <code>cursor-agent --list-models</code>. Leave{" "}
        <em>Default</em> to omit <code>--model</code>.
      </p>
      <details className="settings-learn-more">
        <summary>Hit a usage-limit error?</summary>
        <p>Pick a different model here and save to route new runs through it.</p>
      </details>

      <label className="settings-field">
        <span className="settings-field-label">Cursor CLI path</span>
        <input
          type="text"
          value={form.cursorBin}
          onChange={(e) => onField("cursorBin", e.target.value)}
          placeholder="/usr/local/bin/cursor-agent"
          spellCheck={false}
          autoComplete="off"
        />
      </label>
      <p className="settings-field-help">
        Leave empty to auto-detect on PATH. Use the test button to verify
        before saving.
      </p>
      {form.cursorBin.trim() === "" && resolvedDefaultBin ? (
        <div className="settings-resolved-bin">
          <span className="settings-resolved-bin-label">Currently resolves to</span>
          <code
            className="settings-resolved-bin-path"
            data-testid="settings-resolved-cursor-bin"
          >
            {resolvedDefaultBin}
          </code>
        </div>
      ) : null}
      <div className="settings-inline-actions">
        <button
          type="button"
          className="settings-btn settings-btn--secondary"
          onClick={onProbe}
          disabled={probePending}
        >
          {probePending ? "Testing…" : "Test cursor binary"}
        </button>
      </div>
    </fieldset>
  );
}

export function RunTimeoutSettingsSection({
  form,
  maxInvalid,
  onField,
}: {
  form: SettingsFormState;
  maxInvalid: boolean;
  onField: HandleField;
}) {
  return (
    <fieldset className="settings-fieldset">
      <legend>Run timeout</legend>
      <p className="settings-section-subtitle">
        Hard ceiling on any single agent run&apos;s wall-clock duration.
      </p>
      <label className="settings-field">
        <span className="settings-field-label">Max run duration (seconds)</span>
        <input
          type="number"
          min={0}
          step={1}
          value={form.maxRunDurationSeconds}
          onChange={(e) => onField("maxRunDurationSeconds", e.target.value)}
          aria-invalid={maxInvalid}
        />
      </label>
      <p className="settings-field-help">
        Set to <code>0</code> for no limit.
      </p>
      {maxInvalid ? (
        <p role="alert" className="settings-field-error">
          Must be a non-negative integer.
        </p>
      ) : null}
    </fieldset>
  );
}

export function SettingsActions({
  isDirty,
  maxInvalid,
  pickupInvalid,
  patchPending,
  onDiscard,
}: {
  isDirty: boolean;
  maxInvalid: boolean;
  pickupInvalid: boolean;
  patchPending: boolean;
  onDiscard: () => void;
}) {
  return (
    <div className="settings-actions" data-dirty={isDirty ? "true" : "false"}>
      <div className="settings-actions-status" aria-hidden="true">
        {maxInvalid || pickupInvalid ? (
          <span className="settings-actions-hint settings-actions-hint--warn">
            Resolve the errors above to save.
          </span>
        ) : isDirty ? (
          <span className="settings-actions-hint settings-actions-hint--dirty">
            <span className="settings-actions-dot" />
            Unsaved changes
          </span>
        ) : (
          <span className="settings-actions-hint settings-actions-hint--clean">
            All changes saved
          </span>
        )}
      </div>
      <div className="settings-actions-buttons">
        {isDirty ? (
          <button
            type="button"
            className="settings-btn settings-btn--ghost"
            onClick={onDiscard}
            disabled={patchPending}
          >
            Discard
          </button>
        ) : null}
        <button
          type="submit"
          className="settings-btn settings-btn--primary"
          disabled={!isDirty || patchPending || maxInvalid || pickupInvalid}
        >
          {patchPending ? "Saving…" : "Save changes"}
        </button>
      </div>
    </div>
  );
}

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
