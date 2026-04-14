type Props = {
  draftName: string;
  onDraftNameChange: (value: string) => void;
  disabled: boolean;
  draftSaveLabel: string | null;
  draftSaveError: boolean;
};

export function TaskCreateModalDraftNameField({
  draftName,
  onDraftNameChange,
  disabled,
  draftSaveLabel,
  draftSaveError,
}: Props) {
  return (
    <div className="field grow">
      <label htmlFor="task-draft-name">Draft name</label>
      <input
        id="task-draft-name"
        value={draftName}
        onChange={(ev) => onDraftNameChange(ev.target.value)}
        placeholder="Name this draft"
        disabled={disabled}
      />
      {draftSaveLabel ? (
        <p
          className={[
            "task-create-draft-status",
            draftSaveError ? "task-create-draft-status--error" : "muted",
          ]
            .filter(Boolean)
            .join(" ")}
          aria-live={draftSaveError ? "assertive" : "polite"}
        >
          {draftSaveLabel}
        </p>
      ) : null}
    </div>
  );
}
