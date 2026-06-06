import type { UseQueryResult } from "@tanstack/react-query";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import type { ListCursorModelsResult } from "@/api/settings";
import type { TimezoneSelectOption } from "@/shared/time/appTimezone";
import { formatTimezoneMenuLabel } from "@/shared/time/appTimezone";
import { TimezoneCombobox } from "./TimezoneCombobox";
import {
  RUNNERS,
  runnerShortLabel,
  type SettingsFormState,
  type SettingsStatus,
} from "./settingsForm";
import {
  DEFAULT_CHECK_COMMAND_TIMEOUT_SECONDS,
  DEFAULT_VERIFY_MAX_RETRIES,
  MAX_CHECK_COMMAND_TIMEOUT_SECONDS,
  MAX_VERIFY_MAX_RETRIES,
  MIN_CHECK_COMMAND_TIMEOUT_SECONDS,
} from "@/types/task";

type HandleField = <K extends keyof SettingsFormState>(
  key: K,
  value: SettingsFormState[K],
) => void;

/**
 * Section ids used both as DOM anchors (for the in-page nav rail)
 * and as test/select hooks. Keep in sync with SETTINGS_NAV_ITEMS in
 * SettingsPage.tsx.
 */
export const SECTION_IDS = {
  workspace: "workspace",
  agentWorker: "agent-worker",
  cursorAgent: "cursor-agent",
  verification: "verification",
  runTimeout: "run-timeout",
  display: "display",
  developer: "developer",
} as const;

export function SettingsLoadingState({
  error,
  onRetry,
}: {
  error: Error | null;
  onRetry: () => void;
}) {
  return (
    <section className="settings-page" aria-busy="true">
      <h2 className="settings-page-title">Settings</h2>
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
        <h2 className="settings-page-title">Settings</h2>
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
          Set the repository root in <strong>Workspace</strong> below to enable
          the agent worker, file mentions, and the <code>/repo/*</code> endpoints.
        </p>
      </div>
    </div>
  );
}

/**
 * Section card scaffold. Renders a real `<h2>` heading inside an
 * accessible region so the page has proper outline structure for
 * screen readers and the in-page nav. The previous inset `<legend>`
 * pattern was visually decorative chrome; promoting it to an h2
 * gives the section a real anchor a sighted operator can scan and
 * an assistive-tech user can land on.
 *
 */
function SectionCard({
  id,
  title,
  children,
}: {
  id: string;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section
      id={id}
      className="settings-section"
      aria-labelledby={`${id}-title`}
    >
      <h2 id={`${id}-title`} className="settings-section-title">
        {title}
      </h2>
      <div className="settings-section-body">{children}</div>
    </section>
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
    <SectionCard id={SECTION_IDS.workspace} title="Workspace">
      <label className="settings-field">
        <span className="settings-field-label">Repository root</span>
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
        Absolute path. Empty disables repo features until you pick a workspace.
      </p>
      <details className="settings-learn-more">
        <summary>What reads this path?</summary>
        <p>
          The agent worker, <code>/repo/*</code> endpoints, and{" "}
          <code>@file</code> mentions all resolve paths against this root.
        </p>
      </details>
    </SectionCard>
  );
}

export function AgentWorkerSettingsSection({
  form,
  pickupInvalid,
  cursorModelsQuery,
  modelIdsFromList,
  resolvedDefaultBin,
  probePending,
  onField,
  onProbe,
}: {
  form: SettingsFormState;
  pickupInvalid: boolean;
  cursorModelsQuery: UseQueryResult<ListCursorModelsResult, Error>;
  modelIdsFromList: Set<string>;
  resolvedDefaultBin: string | null;
  probePending: boolean;
  onField: HandleField;
  onProbe: () => void;
}) {
  const runnerLabel = runnerShortLabel(form.runner);
  const showCursorRunnerFields = form.runner.trim() === "cursor";

  return (
    <SectionCard id={SECTION_IDS.agentWorker} title="Agent worker">
      <label className="settings-field settings-field--inline">
        <input
          type="checkbox"
          checked={form.workerEnabled}
          onChange={(e) => onField("workerEnabled", e.target.checked)}
        />
        <span className="settings-field-label">Enable agent worker</span>
      </label>
      <p className="settings-field-help">
        Pulls ready tasks and dispatches them to the configured runner.
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
        <span className="settings-field-label">Pickup delay</span>
        <span className="settings-field-input-suffix">
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
          <span className="settings-field-suffix" aria-hidden="true">
            seconds
          </span>
        </span>
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

      {showCursorRunnerFields ? (
        <div
          id={SECTION_IDS.cursorAgent}
          className="settings-runner-group"
          aria-labelledby={`${SECTION_IDS.cursorAgent}-title`}
        >
          <p
            id={`${SECTION_IDS.cursorAgent}-title`}
            className="settings-runner-group-title"
          >
            {runnerLabel} settings
          </p>

          <label className="settings-field">
            <span className="settings-field-label">Model</span>
            <select
              data-testid="settings-cursor-model-select"
              value={form.cursorModel}
              onChange={(e) => onField("cursorModel", e.target.value)}
              disabled={cursorModelsQuery.isFetching}
              aria-busy={cursorModelsQuery.isFetching}
            >
              <option value="">Default (runner picks)</option>
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
            From <code>cursor-agent --list-models</code>. Default omits{" "}
            <code>--model</code>.
          </p>
          <details className="settings-learn-more">
            <summary>Hit a usage-limit error?</summary>
            <p>
              Pick a different model here and save to route new runs through
              it.
            </p>
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
            Empty = auto-detect on PATH. Test before saving.
          </p>
          {form.cursorBin.trim() === "" && resolvedDefaultBin ? (
            <div className="settings-resolved-bin">
              <span className="settings-resolved-bin-label">
                Currently resolves to
              </span>
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
        </div>
      ) : null}
    </SectionCard>
  );
}

export function VerificationSettingsSection({
  form,
  verifyModelsQuery,
  verifyModelIdsFromList,
  onField,
}: {
  form: SettingsFormState;
  verifyModelsQuery: UseQueryResult<ListCursorModelsResult, Error>;
  verifyModelIdsFromList: Set<string>;
  onField: HandleField;
}) {
  const enabled = form.verifyEnabled;
  const verifyRunnerSaved = form.verifyRunnerName.trim();
  const verifyModelSaved = form.verifyRunnerModel.trim();
  const verifyRunnerKnown =
    verifyRunnerSaved === "" ||
    RUNNERS.some((r) => r.id === verifyRunnerSaved);
  /**
   * Resolved execute-runner label, surfaced inside the verify-runner
   * dropdown so the operator can see WHICH runner verify will reuse
   * when the field is left at its default. Per pkgs/agents/worker/
   * verification.go, an empty `verify_runner_name` falls back to
   * `w.runner` (the execute runner) — without surfacing that here
   * the operator has to read backend code to find out what
   * "Same as execute runner" actually means.
   *
   * Phrased as "Same as execute runner: <value>" rather than
   * "<value> (same as execute runner)" so it reads as a live
   * pointer to the Agent worker section above, not a hardcoded
   * default that happens to be Cursor.
   */
  const executeRunnerTrim = form.runner.trim();
  const executeRunnerEntry = RUNNERS.find((r) => r.id === executeRunnerTrim);
  const executeRunnerLabel =
    executeRunnerEntry?.label ??
    (executeRunnerTrim || "(none configured)");
  return (
    <SectionCard id={SECTION_IDS.verification} title="Verification">
      <label className="settings-verify-toggle">
        <input
          type="checkbox"
          role="switch"
          aria-checked={enabled}
          checked={enabled}
          onChange={(e) => onField("verifyEnabled", e.target.checked)}
        />
        <span className="settings-verify-toggle-copy">
          <span className="settings-verify-toggle-title">
            Verify done criteria before marking them complete
          </span>
          <span className="settings-verify-toggle-help">
            The agent must prove each criterion passed — via your{" "}
            <code>check</code> command or an LLM verifier — before it&apos;s
            marked done. When off, criteria are bulk-marked done on a
            successful execute run.
          </span>
        </span>
      </label>

      {enabled ? (
        <div className="settings-verify-details">
          <div className="settings-verify-group">
            <p className="settings-verify-group-title">Budget</p>

            <label className="settings-field">
              <span className="settings-field-label">
                Retry attempts on failure
              </span>
              <span className="settings-field-input-suffix">
                <input
                  type="number"
                  min={0}
                  max={MAX_VERIFY_MAX_RETRIES}
                  step={1}
                  value={form.verifyMaxRetries}
                  onChange={(e) =>
                    onField("verifyMaxRetries", e.target.value)
                  }
                />
                <span className="settings-field-suffix" aria-hidden="true">
                  attempts
                </span>
              </span>
            </label>
            <p className="settings-field-help">
              Re-runs of execute after a verify failure. Default{" "}
              <code>{DEFAULT_VERIFY_MAX_RETRIES}</code> · Range{" "}
              <code>0</code>–<code>{MAX_VERIFY_MAX_RETRIES}</code>.
            </p>

            <label className="settings-field">
              <span className="settings-field-label">
                Check command timeout
              </span>
              <span className="settings-field-input-suffix">
                <input
                  type="number"
                  min={MIN_CHECK_COMMAND_TIMEOUT_SECONDS}
                  max={MAX_CHECK_COMMAND_TIMEOUT_SECONDS}
                  step={1}
                  value={form.checkCommandTimeoutSeconds}
                  onChange={(e) =>
                    onField("checkCommandTimeoutSeconds", e.target.value)
                  }
                />
                <span className="settings-field-suffix" aria-hidden="true">
                  seconds
                </span>
              </span>
            </label>
            <p className="settings-field-help">
              Each criterion&apos;s <code>check</code> shell command is killed
              after this. Default{" "}
              <code>{DEFAULT_CHECK_COMMAND_TIMEOUT_SECONDS}</code>s · Range{" "}
              <code>{MIN_CHECK_COMMAND_TIMEOUT_SECONDS}</code>–
              <code>{MAX_CHECK_COMMAND_TIMEOUT_SECONDS}</code>.
            </p>
          </div>

          <details className="settings-learn-more settings-verify-advanced">
            <summary>Override verifier for this phase</summary>
            <p>
              By default, verify reuses your execute runner and lets it pick
              the model. Override either field below to pin a specific judge
              for the verify phase only.
            </p>

            <label className="settings-field">
              <span className="settings-field-label">Verify runner</span>
              <select
                data-testid="settings-verify-runner-select"
                value={form.verifyRunnerName}
                onChange={(e) => onField("verifyRunnerName", e.target.value)}
              >
                <option value="">
                  Same as execute runner: {executeRunnerLabel}
                </option>
                {RUNNERS.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.label}
                  </option>
                ))}
                {!verifyRunnerKnown ? (
                  <option value={verifyRunnerSaved}>
                    {verifyRunnerSaved} (saved — not registered here)
                  </option>
                ) : null}
              </select>
            </label>
            <p className="settings-field-help">
              {verifyRunnerSaved === ""
                ? "Reads the runner you set in Agent worker above. Pick a runner here to override for verify only."
                : "Override active — verify uses this runner instead of the execute runner."}
            </p>

            <label className="settings-field">
              <span className="settings-field-label">Verify runner model</span>
              <select
                data-testid="settings-verify-model-select"
                value={form.verifyRunnerModel}
                onChange={(e) =>
                  onField("verifyRunnerModel", e.target.value)
                }
                disabled={verifyModelsQuery.isFetching}
                aria-busy={verifyModelsQuery.isFetching}
              >
                <option value="">
                  Runner default (Cursor picks at runtime)
                </option>
                {verifyModelsQuery.data?.ok && verifyModelsQuery.data.models
                  ? verifyModelsQuery.data.models.map((m) => (
                      <option key={m.id} value={m.id}>
                        {m.label}
                      </option>
                    ))
                  : null}
                {verifyModelSaved !== "" &&
                !verifyModelIdsFromList.has(verifyModelSaved) ? (
                  <option value={verifyModelSaved}>
                    {verifyModelSaved} (saved — not in current list)
                  </option>
                ) : null}
              </select>
            </label>
            <p className="settings-field-help">
              {verifyModelSaved === ""
                ? "No --model flag is passed; the runner uses its default."
                : "Override active — verify pins this model regardless of execute."}
            </p>
            {verifyModelsQuery.isError ? (
              <p role="alert" className="settings-field-error">
                Could not load models for this runner:{" "}
                {verifyModelsQuery.error instanceof Error
                  ? verifyModelsQuery.error.message
                  : String(verifyModelsQuery.error)}
              </p>
            ) : null}
            {verifyModelsQuery.data && !verifyModelsQuery.data.ok ? (
              <p role="alert" className="settings-field-error">
                {verifyModelsQuery.data.error ?? "Model list failed."}
              </p>
            ) : null}
          </details>
        </div>
      ) : null}
    </SectionCard>
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
    <SectionCard id={SECTION_IDS.runTimeout} title="Run timeout">
      <label className="settings-field">
        <span className="settings-field-label">Max run duration</span>
        <span className="settings-field-input-suffix">
          <input
            type="number"
            min={0}
            step={1}
            value={form.maxRunDurationSeconds}
            onChange={(e) => onField("maxRunDurationSeconds", e.target.value)}
            aria-invalid={maxInvalid}
          />
          <span className="settings-field-suffix" aria-hidden="true">
            seconds
          </span>
        </span>
      </label>
      <p className="settings-field-help">
        Hard ceiling per run. <code>0</code> = no limit.
      </p>
      {maxInvalid ? (
        <p role="alert" className="settings-field-error">
          Must be a non-negative integer.
        </p>
      ) : null}
    </SectionCard>
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
    <SectionCard id={SECTION_IDS.display} title="Display">
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
      <p className="settings-field-help">
        Used for every operator-facing timestamp. Storage stays in UTC.
      </p>
    </SectionCard>
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
          disabled={
            !isDirty ||
            patchPending ||
            maxInvalid ||
            pickupInvalid
          }
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
