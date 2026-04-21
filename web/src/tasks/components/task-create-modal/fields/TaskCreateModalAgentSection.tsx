import { useId, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import type { AppSettings } from "@/api/settings";
import { fetchAppSettings, listCursorModels } from "@/api/settings";
import { settingsQueryKeys } from "@/tasks/task-query/queryKeys";

const RUNNERS = [{ id: "cursor", label: "Cursor CLI" }] as const;

const AGENT_HEADING_ID = "task-create-agent-heading";

function runnerDisplayLabel(runnerId: string): string {
  const row = RUNNERS.find((r) => r.id === runnerId);
  return row?.label ?? runnerId;
}

type Props = {
  disabled: boolean;
  /** When true, runner cannot be changed (e.g. edit-task: runner is fixed on the row). */
  lockRunner?: boolean;
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
 * Laid out as an elevated config card (DS §7 / §10) rather than loose
 * selects on a flat row so the "agent runtime" reads as one
 * deliberate configuration unit. Each control gets a leading glyph
 * (terminal for runner, spark for model) for instant visual identity,
 * plus a short helper caption that tells the operator what the field
 * actually controls — the Stripe pattern for form density without
 * hand-holding.
 *
 * The Model select retains an inline spinner during the `listCursorModels`
 * fetch so operators see the field is live (avoids the "stuck on Default"
 * confusion) and an inline polished error banner if discovery fails —
 * never a bare red `<p>`.
 */
export function TaskCreateModalAgentSection({
  disabled,
  lockRunner = false,
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

  const modelFetchError = modelsQuery.isError
    ? modelsQuery.error instanceof Error
      ? modelsQuery.error.message
      : String(modelsQuery.error)
    : null;
  const modelServerError =
    modelsQuery.data && !modelsQuery.data.ok
      ? (modelsQuery.data.error ?? "Model list failed.")
      : null;

  return (
    <section
      className="task-create-agent"
      aria-labelledby={AGENT_HEADING_ID}
    >
      <h3
        id={AGENT_HEADING_ID}
        className="task-create-subtasks-heading term-prompt"
      >
        <span>Agent</span>
      </h3>
      <div className="task-create-agent-panel">
        <div className="task-create-agent-grid">
          <div className="field task-create-agent-field">
            <label htmlFor={runnerId}>Runner</label>
            <div className="task-create-agent-control">
              <span
                className="task-create-agent-control-icon"
                aria-hidden="true"
              >
                <RunnerGlyph />
              </span>
              <select
                id={runnerId}
                className="task-create-agent-select"
                value={runner}
                disabled={disabled || lockRunner}
                onChange={(e) => onRunnerChange(e.target.value)}
              >
                {RUNNERS.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.label}
                  </option>
                ))}
              </select>
            </div>
            <p className="task-create-agent-help">
              Where the task runs — the runtime that executes turns.
            </p>
          </div>
          <div className="field task-create-agent-field">
            <label htmlFor={modelId}>Model</label>
            {runner === "cursor" ? (
              <>
                <div
                  className="task-create-agent-control"
                  data-busy={modelSelectBusy ? "true" : "false"}
                >
                  <span
                    className="task-create-agent-control-icon"
                    aria-hidden="true"
                  >
                    <ModelGlyph />
                  </span>
                  <select
                    id={modelId}
                    className="task-create-agent-select task-create-agent-select--with-trail"
                    data-testid="task-create-cursor-model-select"
                    value={cursorModel}
                    disabled={modelSelectDisabled}
                    aria-busy={modelSelectBusy}
                    onChange={(e) => onCursorModelChange(e.target.value)}
                  >
                    <option value="">Default</option>
                    {modelsQuery.data?.ok && modelsQuery.data.models
                      ? modelsQuery.data.models.map((m) => (
                          <option key={m.id} value={m.id}>
                            {m.label}
                          </option>
                        ))
                      : null}
                    {cursorModel.trim() !== "" &&
                    !modelIdsFromList.has(cursorModel.trim()) ? (
                      <option value={cursorModel.trim()}>
                        {cursorModel.trim()} (saved — not in current list)
                      </option>
                    ) : null}
                  </select>
                  <span
                    className="task-create-agent-control-spinner"
                    aria-hidden="true"
                  />
                </div>
                <p className="task-create-agent-help">
                  {modelSelectBusy
                    ? "Loading available models…"
                    : "Auto uses Cursor's current default unless overridden."}
                </p>
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
                <div className="task-create-agent-control">
                  <span
                    className="task-create-agent-control-icon"
                    aria-hidden="true"
                  >
                    <ModelGlyph />
                  </span>
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

function RunnerGlyph() {
  // Terminal-prompt mark — matches the `$` section heading glyph
  // elsewhere in the create modal and reinforces "this is where the
  // task runs".
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
    >
      <path d="M3.25 5l2.5 3-2.5 3" />
      <path d="M8 11h5" />
    </svg>
  );
}

function ModelGlyph() {
  // 4-point sparkle — the universal shorthand for "AI model" in
  // modern product UIs (Apple Intelligence, Linear, Raycast). Kept
  // stroke-only so it inherits `currentColor` and respects dark mode.
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.4"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M8 2.25L9.35 6.4 13.5 7.75 9.35 9.1 8 13.25 6.65 9.1 2.5 7.75 6.65 6.4z" />
      <path d="M12.5 2.25v2" />
      <path d="M11.5 3.25h2" />
    </svg>
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
