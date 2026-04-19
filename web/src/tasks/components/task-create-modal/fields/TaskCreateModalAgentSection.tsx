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
  runner: string;
  cursorModel: string;
  onRunnerChange: (runner: string) => void;
  onCursorModelChange: (v: string) => void;
};

/** Runner and model for the new task (POST body; does not change Settings). */
export function TaskCreateModalAgentSection({
  disabled,
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
      <div className="task-create-agent-grid">
        <div className="field">
          <label htmlFor={runnerId}>Runner</label>
          <select
            id={runnerId}
            value={runner}
            disabled={disabled}
            onChange={(e) => onRunnerChange(e.target.value)}
          >
            {RUNNERS.map((r) => (
              <option key={r.id} value={r.id}>
                {r.label}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label htmlFor={modelId}>Model</label>
          {runner === "cursor" ? (
            <>
              <select
                id={modelId}
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
              {modelsQuery.isError ? (
                <p role="alert" className="task-create-agent-model-err">
                  Could not load models for {runnerDisplayLabel(runner)}:{" "}
                  {modelsQuery.error instanceof Error
                    ? modelsQuery.error.message
                    : String(modelsQuery.error)}
                </p>
              ) : null}
              {modelsQuery.data && !modelsQuery.data.ok ? (
                <p role="alert" className="task-create-agent-model-err">
                  {modelsQuery.data.error ?? "Model list failed."}
                </p>
              ) : null}
            </>
          ) : (
            <input
              id={modelId}
              type="text"
              value={cursorModel}
              disabled={disabled}
              onChange={(e) => onCursorModelChange(e.target.value)}
              placeholder="Model id (optional)"
              autoComplete="off"
            />
          )}
        </div>
      </div>
    </section>
  );
}
