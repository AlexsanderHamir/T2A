import { useId, useMemo } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import type { AppSettings } from "@/api/settings";
import {
  filterCursorModelsForSelect,
  normalizeCursorModelSelectValue,
} from "@/api/cursorModels";
import { fetchAppSettings, listCursorModels } from "@/api/settings";
import { settingsQueryKeys } from "@/settings/settingsQueryKeys";
import {
  CustomSelect,
  type CustomSelectOption,
} from "@/components/custom-select";

const RUNNERS = [{ id: "cursor", label: "Cursor CLI" }] as const;

const AGENT_HEADING_ID = "task-create-agent-heading";

const RUNNER_OPTIONS: CustomSelectOption[] = RUNNERS.map((r) => ({
  value: r.id,
  label: r.label,
}));

function runnerDisplayLabel(runnerId: string): string {
  const row = RUNNERS.find((r) => r.id === runnerId);
  return row?.label ?? runnerId;
}

type Props = {
  disabled: boolean;
  /** When true, runner cannot be changed (e.g. edit-task: runner is fixed on the row). */
  lockRunner?: boolean;
  /**
   * "Change model" dialog only: skip the lock banner (the dialog already explains scope)
   * and use shorter helper copy with roomier layout via `.task-create-agent--model-dialog`.
   * `createModal` trims helper copy in the new-task sheet.
   */
  variant?: "default" | "modelDialog" | "createModal";
  runner: string;
  cursorModel: string;
  onRunnerChange: (runner: string) => void;
  onCursorModelChange: (v: string) => void;
};

/**
 * TaskCreateModalAgentSection — runtime configuration panel for the
 * new task. Picks the runner (where the task executes) and the model
 * (which underlying LLM the runner drives).
 *
 * Uses the shared CustomSelect portal dropdown so option lists match
 * Priority and other polished controls (native &lt;select&gt; menus
 * cannot be styled consistently across browsers).
 */
export function TaskCreateModalAgentSection({
  disabled,
  lockRunner = false,
  variant = "default",
  runner,
  cursorModel,
  onRunnerChange,
  onCursorModelChange,
}: Props) {
  const baseId = useId();
  const runnerId = `${baseId}-runner`;
  const modelId = `${baseId}-model`;

  const settingsQuery = useQuery<AppSettings>({
    queryKey: settingsQueryKeys.app(),
    queryFn: ({ signal }) => fetchAppSettings({ signal }),
  });

  const cursorBinKey = (settingsQuery.data?.cursor_bin ?? "").trim();

  const modelsQuery = useQuery({
    queryKey: [
      ...settingsQueryKeys.all,
      "create-modal-cursor-models",
      runner,
      cursorBinKey,
    ],
    queryFn: ({ signal }) =>
      listCursorModels(
        {
          runner,
          binary_path: cursorBinKey || undefined,
        },
        { signal },
      ),
    enabled: runner === "cursor",
  });

  const modelIdsFromList = useMemo(() => {
    const m = modelsQuery.data;
    if (!m?.ok || !m.models) return new Set<string>();
    return new Set(m.models.map((x) => x.id));
  }, [modelsQuery.data]);

  const modelSelectBusy = modelsQuery.isFetching;
  const modelSelectDisabled = disabled || modelSelectBusy;
  const cursorModelSelectValue = normalizeCursorModelSelectValue(cursorModel);
  const modelsForSelect = filterCursorModelsForSelect(
    modelsQuery.data?.ok ? modelsQuery.data.models : undefined,
  );

  const modelOptions = useMemo((): CustomSelectOption[] => {
    const opts: CustomSelectOption[] = [{ value: "", label: "Auto" }];
    for (const m of modelsForSelect) {
      opts.push({ value: m.id, label: m.label });
    }
    if (
      cursorModelSelectValue !== "" &&
      !modelIdsFromList.has(cursorModelSelectValue)
    ) {
      opts.push({
        value: cursorModelSelectValue,
        label: `${cursorModelSelectValue} (saved — not in current list)`,
      });
    }
    return opts;
  }, [modelsForSelect, cursorModelSelectValue, modelIdsFromList]);

  const modelFetchError = modelsQuery.isError
    ? modelsQuery.error instanceof Error
      ? modelsQuery.error.message
      : String(modelsQuery.error)
    : null;
  const modelServerError =
    modelsQuery.data && !modelsQuery.data.ok
      ? (modelsQuery.data.error ?? "Model list failed.")
      : null;

  const isModelDialog = variant === "modelDialog";
  const isCreateModal = variant === "createModal";
  const showRunnerHelp = !isCreateModal || lockRunner;
  const showModelHelp =
    !isCreateModal ||
    modelSelectBusy ||
    lockRunner ||
    Boolean(modelFetchError) ||
    Boolean(modelServerError);

  return (
    <section
      className={[
        isModelDialog
          ? "task-create-agent task-create-agent--model-dialog"
          : "task-create-agent",
        isCreateModal ? "task-create-agent--create-modal" : "",
      ]
        .filter(Boolean)
        .join(" ")}
      aria-labelledby={AGENT_HEADING_ID}
    >
      <h3 id={AGENT_HEADING_ID} className="task-create-subtasks-heading">
        Agent
      </h3>
      <div className="task-create-agent-panel">
        {lockRunner && !isModelDialog ? (
          <p className="task-create-agent-lock-notice" role="note">
            <strong>Runner</strong> is fixed—it was chosen when this task was
            created and can&apos;t be changed here.{" "}
            <strong>Model</strong> below can override the workspace default for
            this task only. Workspace CLI path and default model:{" "}
            <Link
              to="/settings#cursor-agent"
              className="task-create-agent-lock-notice-link"
            >
              Settings → Cursor agent
            </Link>
            .
          </p>
        ) : null}
        <div className="task-create-agent-grid">
          <div className="task-create-agent-field">
            <CustomSelect
              id={runnerId}
              label="Runner"
              value={runner}
              options={RUNNER_OPTIONS}
              disabled={disabled || lockRunner}
              onChange={onRunnerChange}
              className="task-create-agent-custom-select"
            />
            {showRunnerHelp ? (
              <p className="task-create-agent-help">
                {lockRunner
                  ? "Set when this task was created; the runner can’t be changed for an existing task."
                  : isCreateModal
                    ? "Saved with the task and fixed after creation."
                    : "Pick the runtime for this task. It’s saved when the task is created and can’t be changed later."}
              </p>
            ) : null}
          </div>
          <div className="task-create-agent-field">
            {runner === "cursor" ? (
              <>
                <CustomSelect
                  id={modelId}
                  label="Model"
                  value={cursorModelSelectValue}
                  options={modelOptions}
                  disabled={modelSelectDisabled}
                  onChange={onCursorModelChange}
                  triggerTestId="task-create-cursor-model-select"
                  className="task-create-agent-custom-select"
                />
                {showModelHelp ? (
                  <p className="task-create-agent-help">
                    {modelSelectBusy
                      ? "Loading available models…"
                      : lockRunner
                        ? isModelDialog
                          ? "Pick a model or Auto. This overrides the workspace default for this task only."
                          : "Per-task: pick a model or Auto. Auto lets cursor-agent choose for this task only."
                        : isCreateModal
                          ? "Auto uses the workspace default."
                          : "Auto uses Cursor's current default unless overridden."}
                  </p>
                ) : null}
                {modelFetchError ? (
                  <div
                    role="alert"
                    className="task-create-agent-model-err"
                  >
                    <AlertGlyph />
                    <span>
                      Could not load models for{" "}
                      {runnerDisplayLabel(runner)}: {modelFetchError}
                    </span>
                  </div>
                ) : null}
                {modelServerError ? (
                  <div
                    role="alert"
                    className="task-create-agent-model-err"
                  >
                    <AlertGlyph />
                    <span>{modelServerError}</span>
                  </div>
                ) : null}
              </>
            ) : (
              <>
                <div className="field task-create-agent-text-field">
                  <label htmlFor={modelId}>Model</label>
                  <input
                    id={modelId}
                    className="task-create-agent-input"
                    type="text"
                    value={cursorModel}
                    disabled={disabled}
                    onChange={(e) => onCursorModelChange(e.target.value)}
                    placeholder="Model id (optional)"
                    autoComplete="off"
                  />
                </div>
                <p className="task-create-agent-help">
                  Optional — leave blank to use the runner default.
                </p>
              </>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}

function AlertGlyph() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="8" cy="8" r="6.25" />
      <path d="M8 5v3.5" />
      <path d="M8 10.75v0.25" />
    </svg>
  );
}
