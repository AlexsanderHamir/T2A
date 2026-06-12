import { FieldRequirementBadge } from "@/shared/FieldLabel";

export type SubtaskSiblingOption = {
  id: string;
  label: string;
};

type Props = {
  idsPrefix: string;
  disabled?: boolean;
  waitForParent: boolean;
  onWaitForParentChange: (v: boolean) => void;
  /** When false, hide the parent checkbox (create flow uses batch toggle elsewhere). */
  showWaitForParent?: boolean;
  siblingOptions: SubtaskSiblingOption[];
  selectedSiblingIds: string[];
  onSelectedSiblingIdsChange: (ids: string[]) => void;
};

export function SubtaskSchedulingFields({
  idsPrefix,
  disabled = false,
  waitForParent,
  onWaitForParentChange,
  showWaitForParent = true,
  siblingOptions,
  selectedSiblingIds,
  onSelectedSiblingIdsChange,
}: Props) {
  const groupId = `${idsPrefix}-sibling-deps`;

  function toggleSibling(id: string, checked: boolean) {
    if (checked) {
      onSelectedSiblingIdsChange([...selectedSiblingIds, id]);
      return;
    }
    onSelectedSiblingIdsChange(selectedSiblingIds.filter((x) => x !== id));
  }

  return (
    <fieldset className="task-subtask-scheduling" disabled={disabled}>
      <legend className="task-subtask-scheduling__legend">
        Execution order
        <FieldRequirementBadge requirement="optional" />
      </legend>
      {showWaitForParent ? (
        <label className="checkbox-label task-subtask-scheduling__row">
          <input
            type="checkbox"
            checked={waitForParent}
            onChange={(ev) => onWaitForParentChange(ev.target.checked)}
            disabled={disabled}
          />
          <span className="checkbox-label-body">
            Start after parent criteria pass
          </span>
        </label>
      ) : null}
      {siblingOptions.length > 0 ? (
        <div
          className="task-subtask-scheduling__siblings"
          role="group"
          aria-labelledby={groupId}
        >
          <p id={groupId} className="task-subtask-scheduling__hint">
            Start after these subtasks complete
          </p>
          <ul className="task-subtask-scheduling__list">
            {siblingOptions.map((opt) => (
              <li key={opt.id}>
                <label className="checkbox-label task-subtask-scheduling__row">
                  <input
                    type="checkbox"
                    checked={selectedSiblingIds.includes(opt.id)}
                    onChange={(ev) => toggleSibling(opt.id, ev.target.checked)}
                    disabled={disabled}
                  />
                  <span className="checkbox-label-body">{opt.label}</span>
                </label>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </fieldset>
  );
}

/** Draft-local sibling picker (create flow nested modal). */
type DraftIndexProps = {
  idsPrefix: string;
  disabled?: boolean;
  pendingSubtasks: ReadonlyArray<{ title: string }>;
  selfIndex: number | null;
  selectedIndices: number[];
  onSelectedIndicesChange: (indices: number[]) => void;
};

export function PendingSubtaskSiblingPicker({
  idsPrefix,
  disabled = false,
  pendingSubtasks,
  selfIndex,
  selectedIndices,
  onSelectedIndicesChange,
}: DraftIndexProps) {
  const groupId = `${idsPrefix}-pending-sibling-deps`;
  const options = pendingSubtasks
    .map((st, index) => ({ index, label: st.title.trim() || `Subtask ${index + 1}` }))
    .filter((opt) => selfIndex === null || opt.index !== selfIndex);

  if (options.length === 0) return null;

  function toggle(index: number, checked: boolean) {
    if (checked) {
      onSelectedIndicesChange([...selectedIndices, index].sort((a, b) => a - b));
      return;
    }
    onSelectedIndicesChange(selectedIndices.filter((i) => i !== index));
  }

  return (
    <div
      className="task-subtask-scheduling__siblings"
      role="group"
      aria-labelledby={groupId}
    >
      <p id={groupId} className="task-subtask-scheduling__hint">
        Start after these subtasks complete
      </p>
      <ul className="task-subtask-scheduling__list">
        {options.map((opt) => (
          <li key={opt.index}>
            <label className="checkbox-label task-subtask-scheduling__row">
              <input
                type="checkbox"
                checked={selectedIndices.includes(opt.index)}
                onChange={(ev) => toggle(opt.index, ev.target.checked)}
                disabled={disabled}
              />
              <span className="checkbox-label-body">{opt.label}</span>
            </label>
          </li>
        ))}
      </ul>
    </div>
  );
}
