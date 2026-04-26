import { useMemo } from "react";
import type { ProjectContextItem } from "@/types";
import { useProjectContext } from "./hooks";

interface ProjectContextPickerProps {
  projectId: string;
  selectedIds: string[];
  disabled?: boolean;
  onChange: (ids: string[]) => void;
}

export function ProjectContextPicker({
  projectId,
  selectedIds,
  disabled,
  onChange,
}: ProjectContextPickerProps) {
  const contextQuery = useProjectContext(projectId, {
    enabled: Boolean(projectId),
    limit: 100,
    pinnedOnly: false,
  });
  const items = contextQuery.data?.items ?? [];
  const selected = useMemo(() => new Set(selectedIds), [selectedIds]);

  if (!projectId) return null;

  function toggle(item: ProjectContextItem) {
    if (disabled) return;
    if (selected.has(item.id)) {
      onChange(selectedIds.filter((id) => id !== item.id));
      return;
    }
    onChange([...selectedIds, item.id]);
  }

  return (
    <fieldset className="project-context-picker" disabled={disabled}>
      <legend>Context for this task</legend>
      <p>
        Choose the exact project context items the agent may use. Nothing is
        selected automatically.
      </p>
      {contextQuery.isPending ? (
        <span className="project-context-picker__status">Loading context…</span>
      ) : items.length === 0 ? (
        <span className="project-context-picker__status">
          This project has no context items yet.
        </span>
      ) : (
        <div className="project-context-picker__items">
          {items.map((item) => (
            <label className="project-context-picker__item" key={item.id}>
              <input
                type="checkbox"
                checked={selected.has(item.id)}
                onChange={() => toggle(item)}
              />
              <span>
                <strong>{item.title}</strong>
                <small>{item.kind}</small>
              </span>
            </label>
          ))}
        </div>
      )}
    </fieldset>
  );
}
