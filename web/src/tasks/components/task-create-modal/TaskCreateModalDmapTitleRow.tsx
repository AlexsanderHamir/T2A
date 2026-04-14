import type { PriorityChoice, TaskType } from "@/types";
import { FieldLabel } from "@/shared/FieldLabel";
import { PrioritySelect, TaskTypeSelect } from "../task-compose";

type Props = {
  title: string;
  onTitleChange: (value: string) => void;
  priority: PriorityChoice;
  onPriorityChange: (value: PriorityChoice) => void;
  taskType: TaskType;
  onTaskTypeChange: (value: TaskType) => void;
  disabled: boolean;
};

export function TaskCreateModalDmapTitleRow({
  title,
  onTitleChange,
  priority,
  onPriorityChange,
  taskType,
  onTaskTypeChange,
  disabled,
}: Props) {
  return (
    <div className="task-create-title-row">
      <div className="field grow">
        <FieldLabel htmlFor="task-new-title" requirement="required">
          Title
        </FieldLabel>
        <input
          id="task-new-title"
          value={title}
          onChange={(ev) => onTitleChange(ev.target.value)}
          placeholder="What should get done?"
          required
          aria-required="true"
          disabled={disabled}
        />
      </div>
      <PrioritySelect
        id="task-new-priority"
        value={priority}
        compact
        onChange={onPriorityChange}
      />
      <TaskTypeSelect
        id="task-new-task-type"
        value={taskType}
        onChange={onTaskTypeChange}
        disabled={disabled}
      />
    </div>
  );
}
