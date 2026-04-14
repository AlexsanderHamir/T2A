import { FieldRequirementBadge } from "@/shared/FieldLabel";

type Props = {
  checklistInherit: boolean;
  disabled: boolean;
  onChecklistInheritChange: (value: boolean) => void;
};

export function TaskCreateModalInheritChecklistField({
  checklistInherit,
  disabled,
  onChecklistInheritChange,
}: Props) {
  return (
    <label className="checkbox-label task-create-inherit-field">
      <input
        type="checkbox"
        checked={checklistInherit}
        onChange={(ev) => onChecklistInheritChange(ev.target.checked)}
        disabled={disabled}
      />
      <span className="checkbox-label-body">
        <span>Inherit parent&apos;s checklist criteria</span>
        <FieldRequirementBadge requirement="optional" />
      </span>
    </label>
  );
}
