import type { UseQueryResult } from "@tanstack/react-query";
import { useMemo } from "react";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import {
  filterCursorModelsForSelect,
  normalizeCursorModelSelectValue,
} from "@/api/cursorModels";
import type { ListCursorModelsResult } from "@/api/settings";
import type { TimezoneSelectOption } from "@/shared/time/appTimezone";
import { formatTimezoneMenuLabel } from "@/shared/time/appTimezone";
import { TimezoneCombobox } from "./TimezoneCombobox";
import { SettingsSelect, groupModelSelectRows, type SettingsSelectOption } from "./SettingsSelect";
import {
  RUNNERS,
  type SettingsFormState,
  type SettingsStatus,
} from "./settingsForm";
import { DEFAULT_VERIFY_MAX_RETRIES } from "@/types/task";

type HandleField = <K extends keyof SettingsFormState>(
  key: K,
  value: SettingsFormState[K],
) => void;

/**
 * Section ids used both as DOM anchors (for the in-page nav rail)
 * and as test/select hooks. Keep in sync with SETTINGS_NAV_ITEMS in
 * SettingsPage.tsx.
 *
 * `cursorAgent`, `agentWorker`, `verification`, and `runTimeout` are
 * retained so existing deep links still scroll to a meaningful target:
 * `#cursor-agent` → Cursor runner card; `#agent-worker` → Execute
 * phase block; `#run-timeout` → max execute duration field;
 * `#verification` → Verify phase block.
 */
export const SECTION_IDS = {
  workspace: "workspace",
  agentWorker: "agent-worker",
  cursorAgent: "cursor-agent",
  phases: "phases",
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

/**
 * Runner — the agent CLI used by every phase. Today only `cursor` is
 * registered, so the picker is single-option, but the section is
 * framed generically so adding a runner later is a matter of pushing
 * a new entry into RUNNERS without rewriting the UI.
 *
 * Holds the runner-level configuration that is independent of any
 * single phase: the runner identity, the binary path, and a probe to
 * verify it works. Per-phase choices (which model to pass for execute
 * vs verify) live under the Phases section. This mirrors the backend:
 * `cursor_bin` is a runner setting (shared across phases), while
 * `cursor_model` and `verify_runner_model` are phase-keyed model
 * overrides handed to that one runner.
 *
 * The card retains `id="cursor-agent"` so legacy deep links from
 * TaskModelConfigModal still land on a meaningful target.
 */
export function RunnerSettingsSection({
  form,
  resolvedDefaultBin,
  probePending,
  onField,
  onProbe,
}: {
  form: SettingsFormState;
  resolvedDefaultBin: string | null;
  probePending: boolean;
  onField: HandleField;
  onProbe: () => void;
}) {
  const runnerOptions = useMemo((): SettingsSelectOption[] => {
    const opts: SettingsSelectOption[] = RUNNERS.map((r) => ({
      value: r.id,
      label: r.label,
    }));
    const saved = form.runner.trim();
    if (saved !== "" && !RUNNERS.some((r) => r.id === saved)) {
      opts.push({ value: saved, label: `${saved} (saved — not registered)` });
    }
    return opts;
  }, [form.runner]);

  return (
    <SectionCard id={SECTION_IDS.cursorAgent} title="Runner">
      <label className="settings-field">
        <span className="settings-field-label">Runner</span>
        <SettingsSelect
          testId="settings-runner-select"
          value={form.runner}
          onChange={(next) => onField("runner", next)}
          options={runnerOptions}
          searchable={false}
        />
      </label>
      <p className="settings-field-help">
        The agent CLI used by every phase. More runners coming soon.
      </p>

      <label className="settings-field">
        <span className="settings-field-label">CLI path</span>
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
          {probePending ? "Testing…" : "Test binary"}
        </button>
      </div>
    </SectionCard>
  );
}

/**
 * Reusable model picker for a phase. Centralises the "Auto + filtered
 * runner models + saved-but-unknown synthetic option + loading and
 * error inline reporting" pattern shared by the execute and verify
 * phase blocks. Both phases drive the same cursor-agent binary, so
 * the wire shape is identical; the only thing that varies per call
 * site is which form field the value writes back to and which
 * react-query observable feeds the option list.
 */
function PhaseModelField({
  testId,
  value,
  onChange,
  query,
  knownIds,
  disabled,
}: {
  testId: string;
  value: string;
  onChange: (next: string) => void;
  query: UseQueryResult<ListCursorModelsResult, Error>;
  knownIds: Set<string>;
  disabled?: boolean;
}) {
  const selectValue = normalizeCursorModelSelectValue(value);
  const models = filterCursorModelsForSelect(
    query.data?.ok ? query.data.models : undefined,
  );
  const modelOptions = useMemo((): SettingsSelectOption[] => {
    const opts: SettingsSelectOption[] = [{ value: "", label: "Auto" }];
    for (const m of models) {
      opts.push({ value: m.id, label: m.label });
    }
    if (selectValue !== "" && !knownIds.has(selectValue)) {
      opts.push({
        value: selectValue,
        label: `${selectValue} (saved — not in current list)`,
      });
    }
    return opts;
  }, [models, selectValue, knownIds]);

  const modelRows = useMemo(
    () => groupModelSelectRows(modelOptions),
    [modelOptions],
  );

  return (
    <>
      <label className="settings-field">
        <span className="settings-field-label">Model</span>
        <SettingsSelect
          testId={testId}
          value={selectValue}
          onChange={onChange}
          options={modelOptions}
          rows={modelRows}
          disabled={disabled || query.isFetching}
          ariaBusy={query.isFetching}
          searchPlaceholder="Search models…"
        />
      </label>
      {query.isError ? (
        <p role="alert" className="settings-field-error">
          Could not load models for this runner:{" "}
          {query.error instanceof Error
            ? query.error.message
            : String(query.error)}
        </p>
      ) : null}
      {query.data && !query.data.ok ? (
        <p role="alert" className="settings-field-error">
          {query.data.error ?? "Model list failed."}
        </p>
      ) : null}
    </>
  );
}

/**
 * Phase icon — outline SVG matching the icon weight used elsewhere
 * in the app (1.6px stroke, 18px viewport). Execute shows a power
 * bolt (active work); Verify shows a shield-check (judgment pass).
 * Inline rather than imported from an icon library so the settings
 * page does not pull a new dependency for two glyphs.
 */
function PhaseIcon({ phase }: { phase: "execute" | "verify" }) {
  if (phase === "execute") {
    return (
      <svg
        className="settings-phase-icon"
        viewBox="0 0 24 24"
        width="18"
        height="18"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M13 3 4.5 13.5h6L11 21l8.5-10.5h-6L13 3Z" />
      </svg>
    );
  }
  return (
    <svg
      className="settings-phase-icon"
      viewBox="0 0 24 24"
      width="18"
      height="18"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M12 3 4 6v6c0 4.5 3.5 8 8 9 4.5-1 8-4.5 8-9V6l-8-3Z" />
      <path d="m9 12 2 2 4-4" />
    </svg>
  );
}

/**
 * Phase flow connector — small vertical chevron + "then" label
 * rendered between the Execute and Verify panels so the lifecycle
 * reads as a sequence, not two coequal cards. The arrow is purely
 * decorative; the screen-reader-only label is "then" so AT users
 * still hear the ordering.
 */
function PhaseFlowConnector() {
  return (
    <div className="settings-phase-flow" aria-hidden="true">
      <span className="settings-phase-flow-line" />
      <span className="settings-phase-flow-label">then</span>
      <svg
        className="settings-phase-flow-icon"
        viewBox="0 0 12 12"
        width="12"
        height="12"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M6 2v8" />
        <path d="m3 7 3 3 3-3" />
      </svg>
    </div>
  );
}

/**
 * Nested phase panel — Execute and Verify each get their own inset
 * card with a phase icon and name so the operator can scan the
 * lifecycle without reading every field label.
 */
function PhasePanel({
  id,
  phase,
  description,
  children,
}: {
  id: string;
  phase: "execute" | "verify";
  description: string;
  children: React.ReactNode;
}) {
  const title = phase === "execute" ? "Execute" : "Verify";
  return (
    <section
      id={id}
      className="settings-phase-panel"
      data-phase={phase}
      aria-labelledby={`${id}-title`}
    >
      <header className="settings-phase-panel-header">
        <span className="settings-phase-panel-glyph" aria-hidden="true">
          <PhaseIcon phase={phase} />
        </span>
        <div className="settings-phase-panel-heading">
          <h3
            id={`${id}-title`}
            className="settings-phase-panel-title"
          >
            {title}
          </h3>
          <p className="settings-phase-panel-desc">{description}</p>
        </div>
      </header>
      <div className="settings-phase-panel-body">{children}</div>
    </section>
  );
}

function PhaseFieldGroup({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="settings-phase-group">
      <p className="settings-phase-group-title">{title}</p>
      {children}
    </div>
  );
}


/**
 * Phases — execute and verify configuration under one section card.
 * Each phase is a nested panel with grouped fields (worker vs runner
 * for execute; policy vs runner vs budget for verify).
 *
 * The runner picker is intentionally absent today: only one runner
 * (Cursor) is registered. When a second runner ships, a per-phase
 * runner select can return alongside the model field.
 */
export function PhasesSettingsSection({
  form,
  pickupInvalid,
  maxInvalid,
  cursorModelsQuery,
  modelIdsFromList,
  verifyModelsQuery,
  verifyModelIdsFromList,
  onField,
}: {
  form: SettingsFormState;
  pickupInvalid: boolean;
  maxInvalid: boolean;
  cursorModelsQuery: UseQueryResult<ListCursorModelsResult, Error>;
  modelIdsFromList: Set<string>;
  verifyModelsQuery: UseQueryResult<ListCursorModelsResult, Error>;
  verifyModelIdsFromList: Set<string>;
  onField: HandleField;
}) {
  return (
    <SectionCard id={SECTION_IDS.phases} title="Phases">
      <div className="settings-phases-stack">
        <PhasePanel
          id={SECTION_IDS.agentWorker}
          phase="execute"
          description="Pulls ready tasks and runs the agent to do the work."
        >
          <PhaseFieldGroup title="Worker">
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
                  onChange={(e) =>
                    onField("agentPickupDelaySeconds", e.target.value)
                  }
                  aria-invalid={pickupInvalid}
                />
                <span className="settings-field-suffix" aria-hidden="true">
                  seconds
                </span>
              </span>
            </label>
            <div className="settings-field-help-block">
              <p className="settings-field-help">
                Minimum wait before the next ready task.
              </p>
              <p className="settings-field-help settings-field-help-meta">
                Default <code>5</code>s
              </p>
            </div>
            {pickupInvalid ? (
              <p role="alert" className="settings-field-error">
                Must be between 0 and 604800 (7 days).
              </p>
            ) : null}
            <label className="settings-field settings-field-row">
              <input
                type="checkbox"
                checked={form.agentCommitExecuteWork === "true"}
                onChange={(e) =>
                  onField(
                    "agentCommitExecuteWork",
                    e.target.checked ? "true" : "false",
                  )
                }
              />
              <span className="settings-field-label">
                Require execute-phase git commits
              </span>
            </label>
            <p className="settings-field-help">
              When enabled, the agent must commit work with a{" "}
              <code>t2a:cycle=&lt;cycle_id&gt;</code> marker before finishing
              execute. Helps resume after a server restart.
            </p>
          </PhaseFieldGroup>

          <PhaseFieldGroup title="Runner">
            <PhaseModelField
              testId="settings-cursor-model-select"
              value={form.cursorModel}
              onChange={(v) => onField("cursorModel", v)}
              query={cursorModelsQuery}
              knownIds={modelIdsFromList}
            />
            <p className="settings-field-help">
              Auto lets cursor-agent choose. Pick a model to pin it for every
              run.
            </p>

            <div id={SECTION_IDS.runTimeout} className="settings-field-block">
              <label className="settings-field">
                <span className="settings-field-label">Max execute duration</span>
                <span className="settings-field-input-suffix">
                  <input
                    type="number"
                    min={0}
                    step={1}
                    value={form.maxRunDurationSeconds}
                    onChange={(e) =>
                      onField("maxRunDurationSeconds", e.target.value)
                    }
                    aria-invalid={maxInvalid}
                  />
                  <span className="settings-field-suffix" aria-hidden="true">
                    seconds
                  </span>
                </span>
              </label>
              <div className="settings-field-help-block">
                <p className="settings-field-help">
                  Cancels the run if it takes longer than this.
                </p>
                <p className="settings-field-help settings-field-help-meta">
                  Default <code>0</code>
                </p>
              </div>
              {maxInvalid ? (
                <p role="alert" className="settings-field-error">
                  Must be a non-negative integer.
                </p>
              ) : null}
            </div>
          </PhaseFieldGroup>
        </PhasePanel>

        <PhaseFlowConnector />

        <PhasePanel
          id={SECTION_IDS.verification}
          phase="verify"
          description="Verifies done criteria after execute."
        >
          <PhaseFieldGroup title="Runner">
            <PhaseModelField
              testId="settings-verify-model-select"
              value={form.verifyRunnerModel}
              onChange={(v) => onField("verifyRunnerModel", v)}
              query={verifyModelsQuery}
              knownIds={verifyModelIdsFromList}
            />
            <p className="settings-field-help">
              Auto lets the verify runner choose. Pick a model to pin for
              verify only.
            </p>
          </PhaseFieldGroup>

          <PhaseFieldGroup title="Budget">
            <label className="settings-field">
              <span className="settings-field-label">
                Retry attempts on failure
              </span>
              <span className="settings-field-input-suffix">
                <input
                  type="number"
                  min={0}
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
            <div className="settings-field-help-block">
              <p className="settings-field-help">
                Re-runs of execute after a verify failure.
              </p>
              <p className="settings-field-help settings-field-help-meta">
                Default: <code>{DEFAULT_VERIFY_MAX_RETRIES}</code>
              </p>
            </div>
          </PhaseFieldGroup>
        </PhasePanel>
      </div>
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
