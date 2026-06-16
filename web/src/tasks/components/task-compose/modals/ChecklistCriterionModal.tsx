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
  /** Satisfied criteria open in read-only view — no edits or saves. */
  readOnly?: boolean;
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
  readOnly = false,
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
  const controlsDisabled = pending || saving;
  const titleId =
    readOnly
      ? "checklist-criterion-view-modal-title"
      : mode === "add"
        ? "checklist-criterion-modal-title"
        : "checklist-criterion-edit-modal-title";
  const busyLabel =
    mode === "add" ? "Adding criterion…" : "Saving changes…";
  const [verifySectionOpen, setVerifySectionOpen] = useState(
    () => readOnly || verifyCommands.length > 0,
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

  const ensureVerifySectionReady = (open: boolean) => {
    setVerifySectionOpen(open);
    if (open && verifyCommands.length === 0) {
      onVerifyCommandsChange([emptyVerifyCommandRow()]);
    }
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
      <section
        className={
          readOnly
            ? "panel modal-sheet task-checklist-criterion-modal-sheet task-checklist-criterion-modal-sheet--read-only"
            : "panel modal-sheet task-checklist-criterion-modal-sheet"
        }
      >
        <h2 id={titleId}>
          {readOnly
            ? "View criterion"
            : mode === "add"
              ? "New criterion"
              : "Edit criterion"}
        </h2>
        <p
          className="muted task-checklist-criterion-modal-lead"
          id="checklist-criterion-modal-description"
        >
          {readOnly
            ? "This criterion is satisfied and locked. You can review the wording and verify commands, but not change them."
            : mode === "add"
              ? "One clear, testable requirement for done."
              : "Update the wording or verification commands."}
        </p>
        <form
          className="task-checklist-criterion-modal-form task-create-form"
          onSubmit={(e) => {
            e.stopPropagation();
            if (readOnly) {
              e.preventDefault();
              return;
            }
            onSubmit(e);
          }}
        >
          <div className="field">
            <FieldLabel
              htmlFor="checklist-criterion-text"
              requirement={readOnly ? undefined : "required"}
            >
              Criterion
            </FieldLabel>
            <textarea
              id="checklist-criterion-text"
              className="task-checklist-criterion-text-input"
              value={text}
              onChange={(ev) => onTextChange(ev.target.value)}
              placeholder="e.g. All subtasks marked done"
              disabled={controlsDisabled}
              readOnly={readOnly}
              autoFocus={!readOnly}
              required={!readOnly}
              aria-required={readOnly ? undefined : "true"}
              rows={3}
            />
          </div>

          <details
            className="task-create-advanced task-checklist-verify-commands"
            open={verifySectionOpen}
            onToggle={(e) => {
              const open = (e.currentTarget as HTMLDetailsElement).open;
              if (readOnly) {
                setVerifySectionOpen(open);
                return;
              }
              ensureVerifySectionReady(open);
            }}
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
                Shell commands run in the repo during the verify phase. The
                verify agent interprets stdout/stderr against each expected
                outcome — exit code alone does not pass the criterion.
              </p>
              {verifyCommands.length > 0 ? (
                <div
                  className="task-checklist-verify-commands__table"
                  role="table"
                  aria-label="Verify commands"
                >
                  <div
                    className="task-checklist-verify-commands__row task-checklist-verify-commands__row--head"
                    role="row"
                  >
                    <span
                      className="task-checklist-verify-commands__cell task-checklist-verify-commands__cell--command"
                      role="columnheader"
                    >
                      Shell command
                    </span>
                    <span
                      className="task-checklist-verify-commands__cell task-checklist-verify-commands__cell--outcome"
                      role="columnheader"
                    >
                      Expected outcome
                    </span>
                    <span
                      className="task-checklist-verify-commands__cell task-checklist-verify-commands__cell--action visually-hidden"
                      role="columnheader"
                    >
                      Remove
                    </span>
                  </div>
                  {verifyCommands.map((row, index) => (
                    <div
                      key={index}
                      className="task-checklist-verify-commands__row"
                      role="row"
                    >
                      <div
                        className="task-checklist-verify-commands__cell task-checklist-verify-commands__cell--command"
                        role="cell"
                      >
                        <label
                          htmlFor={`checklist-verify-cmd-${index}`}
                          className="visually-hidden"
                        >
                          Shell command {index + 1}
                        </label>
                        <input
                          id={`checklist-verify-cmd-${index}`}
                          className="task-checklist-verify-command-input"
                          value={row.command}
                          onChange={(ev) =>
                            updateCommand(index, {
                              command: ev.target.value,
                            })
                          }
                          placeholder="go test ./pkgs/foo/..."
                          disabled={controlsDisabled}
                          readOnly={readOnly}
                          spellCheck={false}
                          autoComplete="off"
                        />
                      </div>
                      <div
                        className="task-checklist-verify-commands__cell task-checklist-verify-commands__cell--outcome"
                        role="cell"
                      >
                        <label
                          htmlFor={`checklist-verify-outcome-${index}`}
                          className="visually-hidden"
                        >
                          Expected outcome for command {index + 1}
                        </label>
                        <input
                          id={`checklist-verify-outcome-${index}`}
                          className="task-checklist-verify-command-outcome-input"
                          value={row.expected_outcome ?? ""}
                          onChange={(ev) =>
                            updateCommand(index, {
                              expected_outcome: ev.target.value,
                            })
                          }
                          placeholder="All tests pass"
                          disabled={controlsDisabled}
                          readOnly={readOnly}
                        />
                      </div>
                      {!readOnly ? (
                      <div
                        className="task-checklist-verify-commands__cell task-checklist-verify-commands__cell--action"
                        role="cell"
                      >
                        <button
                          type="button"
                          className="task-checklist-verify-command-card__remove"
                          disabled={controlsDisabled}
                          aria-label={`Remove command ${index + 1}`}
                          onClick={() => removeCommandRow(index)}
                        >
                          Remove
                        </button>
                      </div>
                      ) : null}
                    </div>
                  ))}
                </div>
              ) : null}
              {!readOnly ? (
              <button
                type="button"
                className="secondary task-checklist-verify-command-add"
                disabled={
                  controlsDisabled ||
                  verifyCommands.length >= MAX_VERIFY_COMMANDS_PER_ITEM
                }
                onClick={addCommandRow}
              >
                Add command
              </button>
              ) : null}
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
            {readOnly ? (
              <button type="button" className="secondary" onClick={onClose}>
                Close
              </button>
            ) : (
              <>
                <button
                  type="button"
                  className="secondary"
                  disabled={controlsDisabled}
                  onClick={onClose}
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="task-create-submit"
                  disabled={!text.trim() || controlsDisabled}
                >
                  {mode === "add" ? "Add criterion" : "Save changes"}
                </button>
              </>
            )}
          </div>
        </form>
      </section>
    </Modal>
  );
}
