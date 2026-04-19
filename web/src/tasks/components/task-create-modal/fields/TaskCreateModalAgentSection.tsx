import { useId } from "react";
import { useQuery } from "@tanstack/react-query";
import { listCursorModels } from "@/api/settings";
import { settingsQueryKeys } from "@/tasks/task-query/queryKeys";

const RUNNERS = [{ id: "cursor", label: "Cursor CLI" }] as const;

type Props = {
  disabled: boolean;
  runner: string;
  cursorModel: string;
  onRunnerChange: (runner: string) => void;
  onCursorModelChange: (v: string) => void;
};

/**
 * Runner and model for the new task. Defaults come from app settings; changes
 * apply only to this task (POST body), not global defaults.
 */
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
  const listId = `${baseId}-model-list`;

  const modelsQuery = useQuery({
    queryKey: [
      ...settingsQueryKeys.all,
      "create-modal-cursor-models",
      runner,
    ],
    queryFn: ({ signal }) =>
      listCursorModels({ runner }, { signal }),
    enabled: runner === "cursor",
  });

  const modelOptions =
    modelsQuery.data?.ok && modelsQuery.data.models
      ? modelsQuery.data.models
      : [];

  return (
    <fieldset className="task-create-agent-fieldset">
      <legend className="task-create-agent-legend">Agent</legend>
      <p className="muted task-create-agent-hint">
        Defaults match your settings. You can override for this task only — saving
        here does not change global defaults.
      </p>
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
          <input
            id={modelId}
            type="text"
            value={cursorModel}
            disabled={disabled}
            onChange={(e) => onCursorModelChange(e.target.value)}
            placeholder="Leave empty for Cursor default"
            list={runner === "cursor" ? listId : undefined}
            autoComplete="off"
          />
          {runner === "cursor" ? (
            <datalist id={listId}>
              {modelOptions.map((m) => (
                <option key={m.id} value={m.id} />
              ))}
            </datalist>
          ) : null}
        </div>
      </div>
    </fieldset>
  );
}
