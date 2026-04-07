import { useCallback, useState, type FormEvent } from "react";
import type { PriorityChoice, TaskType } from "@/types";
import { FieldLabel, FieldRequirementBadge } from "@/shared/FieldLabel";
import type { TaskWithDepth } from "../flattenTaskTree";
import type { PendingSubtaskDraft } from "../pendingSubtaskDraft";
import { Modal } from "../../shared/Modal";
import { NestedSubtaskDraftModal } from "./NestedSubtaskDraftModal";
import { ParentTaskSelect } from "./ParentTaskSelect";
import { PrioritySelect } from "./PrioritySelect";
import { TaskComposeFields } from "./TaskComposeFields";
import { TaskTypeSelect } from "./TaskTypeSelect";

type Props = {
  pending: boolean;
  saving: boolean;
  parentOptionsLoading?: boolean;
  draftSaving: boolean;
  draftSaveLabel: string | null;
  draftSaveError: boolean;
  onClose: () => void;
  title: string;
  prompt: string;
  priority: PriorityChoice;
  taskType: TaskType;
  checklistItems: string[];
  parentOptions: TaskWithDepth[];
  parentId: string;
  checklistInherit: boolean;
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: PriorityChoice) => void;
  onTaskTypeChange: (t: TaskType) => void;
  onParentIdChange: (id: string) => void;
  onChecklistInheritChange: (v: boolean) => void;
  onAppendChecklistCriterion: (text: string) => void;
  onUpdateChecklistRow: (index: number, text: string) => void;
  onRemoveChecklistRow: (index: number) => void;
  pendingSubtasks: PendingSubtaskDraft[];
  onAddPendingSubtask: (d: PendingSubtaskDraft) => void;
  onUpdatePendingSubtask: (index: number, d: PendingSubtaskDraft) => void;
  onRemovePendingSubtask: (index: number) => void;
  evaluatePending: boolean;
  evaluation: {
    overallScore: number;
    overallSummary: string;
    sections: Array<{ key: string; score: number }>;
  } | null;
  draftName: string;
  onDraftNameChange: (name: string) => void;
  dmapCommitLimit: string;
  dmapDomain: string;
  dmapDescription: string;
  onDmapCommitLimitChange: (value: string) => void;
  onDmapDomainChange: (value: string) => void;
  onDmapDescriptionChange: (value: string) => void;
  onSaveDraft: () => void;
  onEvaluate: () => void;
  onSubmit: (e: FormEvent) => void;
};

export function TaskCreateModal({
  pending,
  saving,
  parentOptionsLoading = false,
  draftSaving,
  draftSaveLabel,
  draftSaveError,
  onClose,
  title,
  prompt,
  priority,
  taskType,
  checklistItems,
  parentOptions,
  parentId,
  checklistInherit,
  onTitleChange,
  onPromptChange,
  onPriorityChange,
  onTaskTypeChange,
  onParentIdChange,
  onChecklistInheritChange,
  onAppendChecklistCriterion,
  onUpdateChecklistRow,
  onRemoveChecklistRow,
  pendingSubtasks,
  onAddPendingSubtask,
  onUpdatePendingSubtask,
  onRemovePendingSubtask,
  evaluatePending,
  evaluation,
  draftName,
  onDraftNameChange,
  dmapCommitLimit,
  dmapDomain,
  dmapDescription,
  onDmapCommitLimitChange,
  onDmapDomainChange,
  onDmapDescriptionChange,
  onSaveDraft,
  onEvaluate,
  onSubmit,
}: Props) {
  const disabled = pending || saving;
  const hasParent = Boolean(parentId.trim());
  const hideComposeChecklist = hasParent && checklistInherit;
  const dmapMode = taskType === "dmap";
  const parsedCommitLimit = Number.parseInt(dmapCommitLimit, 10);
  const dmapCommitValid =
    Number.isInteger(parsedCommitLimit) && parsedCommitLimit > 0;
  const dmapDomainValid = dmapDomain.trim().length > 0;
  const dmapReady = !dmapMode || (dmapCommitValid && dmapDomainValid);
  const subtasksHeadingId = "task-new-subtasks-heading";

  const [nestedOpen, setNestedOpen] = useState(false);
  const [nestedEditIndex, setNestedEditIndex] = useState<number | null>(null);
  const [nestedInstanceKey, setNestedInstanceKey] = useState(0);
  const [nestedInitial, setNestedInitial] = useState<PendingSubtaskDraft | null>(
    null,
  );

  const openNestedNew = useCallback(() => {
    setNestedEditIndex(null);
    setNestedInitial(null);
    setNestedInstanceKey((k) => k + 1);
    setNestedOpen(true);
  }, []);

  const openNestedEdit = useCallback(
    (index: number) => {
      const d = pendingSubtasks[index];
      setNestedEditIndex(index);
      setNestedInitial({
        title: d.title,
        initial_prompt: d.initial_prompt,
        priority: d.priority,
        task_type: d.task_type,
        checklistItems: [...d.checklistItems],
        checklist_inherit: d.checklist_inherit,
      });
      setNestedInstanceKey((k) => k + 1);
      setNestedOpen(true);
    },
    [pendingSubtasks],
  );

  const handleNestedClose = useCallback(() => {
    setNestedOpen(false);
  }, []);

  const handleNestedSave = useCallback(
    (d: PendingSubtaskDraft) => {
      if (nestedEditIndex !== null) {
        onUpdatePendingSubtask(nestedEditIndex, d);
      } else {
        onAddPendingSubtask(d);
      }
      setNestedOpen(false);
    },
    [nestedEditIndex, onAddPendingSubtask, onUpdatePendingSubtask],
  );

  const busyLabel = hasParent
    ? "Creating subtask…"
    : pendingSubtasks.length > 0
      ? "Creating task and subtasks…"
      : "Creating task…";

  return (
    <>
      <Modal
        onClose={onClose}
        labelledBy="task-create-modal-title"
        size="wide"
        busy={pending}
        busyLabel={busyLabel}
      >
        <section className="panel modal-sheet modal-sheet--edit task-create-modal-sheet task-create">
          <h2 id="task-create-modal-title">
            {hasParent ? "New subtask" : "New task"}
          </h2>
          <form
            className="task-create-modal-form task-create-form"
            onSubmit={onSubmit}
          >
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
            <div className="task-create-parent-field grow">
              {parentOptionsLoading ? (
                <div
                  className="task-create-parent-loading"
                  aria-hidden="true"
                >
                  <span className="skeleton-block task-create-parent-loading-label" />
                  <span className="skeleton-block task-create-parent-loading-input" />
                </div>
              ) : (
                <ParentTaskSelect
                  id="task-new-parent"
                  value={parentId}
                  parentOptions={parentOptions}
                  onChange={onParentIdChange}
                  disabled={disabled}
                />
              )}
              <p className="task-create-parent-hint muted">
                {hasParent ? (
                  <>
                    Prompt, priority, and optional criteria — or inherit the
                    parent&apos;s checklist.
                  </>
                ) : (
                  <>
                    Empty = top-level task. Pick a parent to add a{" "}
                    <strong>subtask</strong>.
                  </>
                )}
              </p>
              {parentOptionsLoading ? (
                <p className="visually-hidden" role="status" aria-live="polite">
                  Loading parent task options…
                </p>
              ) : null}
            </div>

            {dmapMode ? (
              <>
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
                <section
                  className="task-create-dmap"
                  aria-label="DMAP task configuration"
                >
                  <h3 className="task-create-dmap-title">DMAP configuration</h3>
                  <div className="row">
                    <div className="field grow">
                      <FieldLabel
                        htmlFor="task-new-dmap-commit-limit"
                        requirement="required"
                      >
                        Commits until stoppage
                      </FieldLabel>
                      <input
                        id="task-new-dmap-commit-limit"
                        type="number"
                        min={1}
                        step={1}
                        inputMode="numeric"
                        value={dmapCommitLimit}
                        onChange={(ev) => onDmapCommitLimitChange(ev.target.value)}
                        placeholder="e.g. 8"
                        required
                        aria-required="true"
                        disabled={disabled}
                      />
                    </div>
                    <div className="field grow">
                      <FieldLabel htmlFor="task-new-dmap-domain" requirement="required">
                        DMAP domain
                      </FieldLabel>
                      <select
                        id="task-new-dmap-domain"
                        value={dmapDomain}
                        onChange={(ev) => onDmapDomainChange(ev.target.value)}
                        required
                        aria-required="true"
                        disabled={disabled}
                      >
                        <option value="">Choose domain</option>
                        <option value="frontend">Frontend</option>
                        <option value="backend">Backend</option>
                        <option value="fullstack">Fullstack</option>
                        <option value="devops">DevOps</option>
                        <option value="data">Data</option>
                        <option value="qa">QA</option>
                      </select>
                    </div>
                  </div>
                  <div className="field grow">
                    <FieldLabel
                      htmlFor="task-new-dmap-description"
                      requirement="optional"
                    >
                      Direction notes
                    </FieldLabel>
                    <textarea
                      id="task-new-dmap-description"
                      value={dmapDescription}
                      onChange={(ev) => onDmapDescriptionChange(ev.target.value)}
                      placeholder="Optional guidance for this DMAP run."
                      rows={4}
                      disabled={disabled}
                    />
                  </div>
                </section>
              </>
            ) : (
              <TaskComposeFields
                idsPrefix="task-new"
                editorKey="create-prompt-modal"
                title={title}
                prompt={prompt}
                priority={priority}
                taskType={taskType}
                checklistItems={checklistItems}
                hideChecklist={hideComposeChecklist}
                disabled={disabled}
                onTitleChange={onTitleChange}
                onPromptChange={onPromptChange}
                onPriorityChange={onPriorityChange}
                onTaskTypeChange={onTaskTypeChange}
                onAppendChecklistCriterion={onAppendChecklistCriterion}
                onUpdateChecklistRow={onUpdateChecklistRow}
                onRemoveChecklistRow={onRemoveChecklistRow}
              />
            )}

            {hasParent && !dmapMode ? (
              <label className="checkbox-label task-create-inherit-field">
                <input
                  type="checkbox"
                  checked={checklistInherit}
                  onChange={(ev) =>
                    onChecklistInheritChange(ev.target.checked)
                  }
                  disabled={disabled}
                />
                <span className="checkbox-label-body">
                  <span>Inherit parent&apos;s checklist criteria</span>
                  <FieldRequirementBadge requirement="optional" />
                </span>
              </label>
            ) : null}

            {!hasParent && !dmapMode ? (
              <div className="task-create-subtasks">
                <div className="task-create-subtasks-head">
                  <div className="field-heading-with-req task-create-subtasks-heading-row">
                    <h3
                      className="task-create-subtasks-heading"
                      id={subtasksHeadingId}
                    >
                      Subtasks
                    </h3>
                    <FieldRequirementBadge requirement="optional" />
                  </div>
                  <button
                    type="button"
                    className="task-detail-add-subtask-btn"
                    disabled={disabled}
                    aria-label="Open form to add a subtask"
                    onClick={openNestedNew}
                  >
                    New subtask
                  </button>
                </div>
                <p className="task-create-subtasks-hint muted">
                  <strong>New subtask</strong> opens another form. Subtasks are
                  created when you click <strong>Create</strong>.
                </p>
                {pendingSubtasks.length > 0 ? (
                  <ul
                    className="task-checklist-list"
                    aria-labelledby={subtasksHeadingId}
                  >
                    {pendingSubtasks.map((d, index) => (
                      <li
                        key={`${index}-${d.title}`}
                        className="task-checklist-row task-create-pending-subtask-row"
                      >
                        <span className="task-checklist-label">{d.title}</span>
                        <div className="task-create-pending-subtask-actions">
                          <button
                            type="button"
                            className="task-detail-checklist-add-btn"
                            disabled={disabled}
                            onClick={() => openNestedEdit(index)}
                          >
                            Edit
                          </button>
                          <button
                            type="button"
                            className="task-create-checklist-remove"
                            disabled={disabled}
                            onClick={() => onRemovePendingSubtask(index)}
                          >
                            Remove
                          </button>
                        </div>
                      </li>
                    ))}
                  </ul>
                ) : null}
              </div>
            ) : null}

            <section
              className="task-create-evaluation-summary"
              aria-label="Draft evaluation summary"
              aria-live="polite"
            >
              <div className="task-create-evaluation-head">
                <h3 className="task-create-evaluation-title">
                  Latest evaluation score
                </h3>
                {evaluation ? (
                  <p className="task-create-evaluation-score-badge">
                    {evaluation.overallScore}
                    <span>/100</span>
                  </p>
                ) : null}
              </div>
              {evaluation ? (
                <>
                  <p className="task-create-evaluation-overall">
                    <strong>Overall:</strong> {evaluation.overallSummary}
                  </p>
                  <ul className="task-create-evaluation-sections">
                    {evaluation.sections.map((s) => (
                      <li key={s.key}>
                        <span>{s.key.replaceAll("_", " ")}</span>
                        <strong>{s.score}/100</strong>
                      </li>
                    ))}
                  </ul>
                </>
              ) : (
                <p className="muted task-create-evaluation-empty">
                  No score yet. Click <strong>Evaluate</strong> and your result
                  appears here before you create the task.
                </p>
              )}
            </section>

            <div className="row stack-row-actions task-create-modal-actions">
              <button
                type="button"
                className="secondary"
                disabled={disabled}
                onClick={onClose}
              >
                Cancel
              </button>
              <button
                type="button"
                className="secondary"
                disabled={disabled || draftSaving}
                onClick={onSaveDraft}
              >
                {draftSaving ? "Saving draft…" : "Save draft"}
              </button>
              <button
                type="button"
                className="secondary task-create-evaluate-btn"
                disabled={!title.trim() || !priority || !dmapReady || disabled}
                onClick={onEvaluate}
              >
                {evaluatePending ? "Evaluating…" : "Evaluate"}
              </button>
              <button
                type="submit"
                className="task-create-submit"
                disabled={!title.trim() || !priority || !dmapReady || disabled}
              >
                {hasParent ? "Add subtask" : "Create"}
              </button>
            </div>
          </form>
        </section>
      </Modal>

      {nestedOpen ? (
        <NestedSubtaskDraftModal
          instanceKey={nestedInstanceKey}
          initialDraft={nestedInitial}
          onClose={handleNestedClose}
          onSave={handleNestedSave}
        />
      ) : null}
    </>
  );
}
