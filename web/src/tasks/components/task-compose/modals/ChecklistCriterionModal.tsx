import { useState, type FormEvent } from "react";
import type { ChecklistVerifyCommandInput } from "@/types";
import { FieldLabel } from "@/shared/FieldLabel";
import { Modal } from "../../../../shared/Modal";
import { MutationErrorBanner } from "../../../../shared/MutationErrorBanner";
import {
  emptyVerifyCommandRow,
  MAX_VERIFY_COMMANDS_PER_ITEM,
} from "@/tasks/task-compose/checklistRequirement";

type Props = {
  mode: "add" | "edit";
  pending: boolean;
  saving: boolean;
  onClose: () => void;
  text: string;
  onTextChange: (v: string) => void;
  verifyCommands: ChecklistVerifyCommandInput[];
  onVerifyCommandsChange: (cmds: ChecklistVerifyCommandInput[]) => void;
  onSubmit: (e: FormEvent) => void;
  modalStack?: "default" | "nested";
  lockBodyScroll?: boolean;
  dismissibleWhileBusy?: boolean;
  error?: unknown;
  errorFallback?: string;
};

function verifyCommandsHint(count: number): string {
  if (count === 0) return "Optional";
  if (count === 1) return "1 command";
  return `${count} commands`;
}

export function ChecklistCriterionModal({
  mode,
  pending,
  saving,
  onClose,
  text,
  onTextChange,
  verifyCommands,
  onVerifyCommandsChange,
  onSubmit,
  modalStack = "default",
  lockBodyScroll = true,
  dismissibleWhileBusy = false,
  error = null,
  errorFallback,
}: Props) {
  const disabled = pending || saving;
  const titleId =
    mode === "add"
      ? "checklist-criterion-modal-title"
      : "checklist-criterion-edit-modal-title";
  const busyLabel =
    mode === "add" ? "Adding criterion…" : "Saving changes…";
  const [verifySectionOpen, setVerifySectionOpen] = useState(
    () => verifyCommands.length > 0,
  );

  const updateCommand = (
    index: number,
    patch: Partial<ChecklistVerifyCommandInput>,
  ) => {
    onVerifyCommandsChange(
      verifyCommands.map((row, i) => (i === index ? { ...row, ...patch } : row)),
    );
  };

  const addCommandRow = () => {
    if (verifyCommands.length >= MAX_VERIFY_COMMANDS_PER_ITEM) return;
    setVerifySectionOpen(true);
    onVerifyCommandsChange([...verifyCommands, emptyVerifyCommandRow()]);
  };

  const removeCommandRow = (index: number) => {
    onVerifyCommandsChange(verifyCommands.filter((_, i) => i !== index));
  };

  return (
    <Modal
      onClose={onClose}
      labelledBy={titleId}
      describedBy="checklist-criterion-modal-description"
      busy={pending}
      busyLabel={busyLabel}
      stack={modalStack}
      lockBodyScroll={lockBodyScroll}
      dismissibleWhileBusy={dismissibleWhileBusy}
    >
      <section className="panel modal-sheet task-checklist-criterion-modal-sheet">
        <h2 id={titleId}>
          {mode === "add" ? "New criterion" : "Edit criterion"}
        </h2>
        <p
          className="muted task-checklist-criterion-modal-lead"
          id="checklist-criterion-modal-description"
        >
          {mode === "add"
            ? "One clear, testable requirement for done."
            : "Update the wording or verification commands."}
        </p>
        <form
          className="task-checklist-criterion-modal-form task-create-form"
          onSubmit={(e) => {
            e.stopPropagation();
            onSubmit(e);
          }}
        >
          <div className="field">
            <FieldLabel htmlFor="checklist-criterion-text" requirement="required">
              Criterion
            </FieldLabel>
            <textarea
              id="checklist-criterion-text"
              className="task-checklist-criterion-text-input"
              value={text}
              onChange={(ev) => onTextChange(ev.target.value)}
              placeholder="e.g. All subtasks marked done"
              disabled={disabled}
              autoFocus
              required
              aria-required="true"
              rows={3}
            />
          </div>

          <details
            className="task-create-advanced task-checklist-verify-commands"
            open={verifySectionOpen}
            onToggle={(e) =>
              setVerifySectionOpen((e.currentTarget as HTMLDetailsElement).open)
            }
          >
            <summary
              className="task-create-advanced__summary"
              data-testid="checklist-verify-commands-toggle"
            >
              <span
                className="task-create-advanced__chevron"
                aria-hidden="true"
              />
              <span className="task-create-advanced__label">
                Verify commands
              </span>
              <span className="task-create-advanced__hint">
                {verifyCommandsHint(verifyCommands.length)}
              </span>
            </summary>
            <div className="task-checklist-verify-commands__body">
              <p className="task-checklist-verify-commands__note">
                Shell checks run in the repo during verify. Output is saved for
                the verifier — exit code alone does not pass the criterion.
              </p>
              {verifyCommands.length > 0 ? (
                <div
                  className="task-checklist-verify-commands__list"
                  role="list"
                >
                  {verifyCommands.map((row, index) => (
                    <div
                      key={index}
                      className="task-checklist-verify-command-card"
                      role="listitem"
                    >
                      <div className="task-checklist-verify-command-card__head">
                        <span className="task-checklist-verify-command-card__index">
                          Command {index + 1}
                        </span>
                        <button
                          type="button"
                          className="task-checklist-verify-command-card__remove"
                          disabled={disabled}
                          onClick={() => removeCommandRow(index)}
                        >
                          Remove
                        </button>
                      </div>
                      <div className="task-checklist-verify-command-card__fields">
                        <div className="field">
                          <FieldLabel htmlFor={`checklist-verify-cmd-${index}`}>
                            Shell command
                          </FieldLabel>
                          <input
                            id={`checklist-verify-cmd-${index}`}
                            className="task-checklist-verify-command-input"
                            value={row.command}
                            onChange={(ev) =>
                              updateCommand(index, { command: ev.target.value })
                            }
                            placeholder="go test ./pkgs/foo/..."
                            disabled={disabled}
                            spellCheck={false}
                            autoComplete="off"
                          />
                        </div>
                        <div className="field">
                          <FieldLabel
                            htmlFor={`checklist-verify-outcome-${index}`}
                          >
                            Expected outcome
                          </FieldLabel>
                          <input
                            id={`checklist-verify-outcome-${index}`}
                            value={row.expected_outcome ?? ""}
                            onChange={(ev) =>
                              updateCommand(index, {
                                expected_outcome: ev.target.value,
                              })
                            }
                            placeholder="All tests pass"
                            disabled={disabled}
                          />
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              ) : null}
              <button
                type="button"
                className="secondary task-checklist-verify-command-add"
                disabled={
                  disabled ||
                  verifyCommands.length >= MAX_VERIFY_COMMANDS_PER_ITEM
                }
                onClick={addCommandRow}
              >
                Add command
              </button>
            </div>
          </details>

          <MutationErrorBanner
            error={error}
            fallback={
              errorFallback ??
              (mode === "add"
                ? "Could not add criterion."
                : "Could not save changes.")
            }
            className="task-checklist-criterion-modal-err"
          />
          <div className="row stack-row-actions task-checklist-criterion-modal-actions">
            <button
              type="button"
              className="secondary"
              disabled={disabled}
              onClick={onClose}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="task-create-submit"
              disabled={!text.trim() || disabled}
            >
              {mode === "add" ? "Add criterion" : "Save changes"}
            </button>
          </div>
        </form>
      </section>
    </Modal>
  );
}
