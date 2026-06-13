import { FieldLabel } from "@/shared/FieldLabel";
import { TaskCreateDependsOnPicker } from "./TaskCreateDependsOnPicker";

type Props = {
  disabled: boolean;
  tagsCsv: string;
  milestone: string;
  /**
   * Project the new task is scoped to. Forwarded to the dependency
   * picker so it filters task lookups by `project_id`. Empty string
   * means "no project bound" — the picker reads as disabled chrome.
   */
  projectId: string;
  dependsOn: string[];
  onTagsCsvChange: (value: string) => void;
  onMilestoneChange: (value: string) => void;
  onDependsOnChange: (value: string[]) => void;
  /** When false, hides the depends-on field (detail page owns dependency edits). */
  showDependsOn?: boolean;
};

export function TaskCreateModalSchedulingFields({
  disabled,
  tagsCsv,
  milestone,
  projectId,
  dependsOn,
  onTagsCsvChange,
  onMilestoneChange,
  onDependsOnChange,
  showDependsOn = true,
}: Props) {
  return (
    <fieldset className="task-create-scheduling" disabled={disabled}>
      <legend className="task-create-scheduling__legend">
        Tags & dependencies
      </legend>
      <div className="task-create-scheduling__grid">
        <div className="task-create-scheduling__field">
          <FieldLabel htmlFor="create-tags">Tags</FieldLabel>
          <input
            id="create-tags"
            className="input"
            value={tagsCsv}
            onChange={(e) => onTagsCsvChange(e.target.value)}
            placeholder="e.g. backend, api"
          />
        </div>
        <div className="task-create-scheduling__field">
          <FieldLabel htmlFor="create-milestone">Milestone</FieldLabel>
          <input
            id="create-milestone"
            className="input"
            value={milestone}
            onChange={(e) => onMilestoneChange(e.target.value)}
            placeholder="e.g. M1 — auth"
          />
        </div>
        {showDependsOn ? (
          <div className="task-create-scheduling__field task-create-scheduling__field--full">
            <TaskCreateDependsOnPicker
              projectId={projectId}
              selected={dependsOn}
              onChange={onDependsOnChange}
              disabled={disabled}
            />
          </div>
        ) : null}
      </div>
    </fieldset>
  );
}
