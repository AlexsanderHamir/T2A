import { type UseQueryResult } from "@tanstack/react-query";
import type { ListCursorModelsResult } from "@/api/settings";
import { DEFAULT_VERIFY_MAX_RETRIES } from "@/types/task";
import type { SettingsFormState } from "../settingsForm";
import { SECTION_IDS } from "./sectionIds";
import {
  PhaseFieldGroup,
  PhaseFlowConnector,
  PhaseModelField,
  PhasePanel,
  SectionCard,
} from "./settingsSectionLayout";
import type { HandleField } from "./settingsSectionTypes";

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
  streamIdleInvalid,
  cursorModelsQuery,
  modelIdsFromList,
  verifyModelsQuery,
  verifyModelIdsFromList,
  onField,
}: {
  form: SettingsFormState;
  pickupInvalid: boolean;
  maxInvalid: boolean;
  streamIdleInvalid: boolean;
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

            <div className="settings-field-block">
              <label className="settings-field">
                <span className="settings-field-label">Agent silence limit</span>
                <span className="settings-field-input-suffix">
                  <input
                    type="number"
                    min={0}
                    step={1}
                    value={form.streamIdleStuckSeconds}
                    onChange={(e) =>
                      onField("streamIdleStuckSeconds", e.target.value)
                    }
                    aria-invalid={streamIdleInvalid}
                  />
                  <span className="settings-field-suffix" aria-hidden="true">
                    seconds
                  </span>
                </span>
              </label>
              <div className="settings-field-help-block">
                <p className="settings-field-help">
                  Stops the run when the agent produces no new output for this
                  long. Hamix may use git commits or reports the agent already
                  wrote and move on to verify.
                </p>
                <p className="settings-field-help settings-field-help-meta">
                  Default <code>60</code>s. Set <code>0</code> to disable.
                </p>
              </div>
              {streamIdleInvalid ? (
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
