import { useEffect, useState, type FormEvent } from "react";
import type { PriorityChoice } from "@/types";
import { FieldRequirementBadge } from "@/shared/FieldLabel";
import { Modal } from "../../shared/Modal";
import type { PendingSubtaskDraft } from "../pendingSubtaskDraft";
import { TaskComposeFields } from "./TaskComposeFields";

type Props = {
  instanceKey: number;
  initialDraft: PendingSubtaskDraft | null;
  onClose: () => void;
  onSave: (draft: PendingSubtaskDraft) => void;
};

export function NestedSubtaskDraftModal({
  instanceKey,
  initialDraft,
  onClose,
  onSave,
}: Props) {
  const [title, setTitle] = useState("");
  const [prompt, setPrompt] = useState("");
  const [priority, setPriority] = useState<PriorityChoice>("");
  const [checklistItems, setChecklistItems] = useState<string[]>([]);
  const [checklistInherit, setChecklistInherit] = useState(false);

  useEffect(() => {
    if (initialDraft) {
      setTitle(initialDraft.title);
      setPrompt(initialDraft.initial_prompt);
      setPriority(initialDraft.priority);
      setChecklistInherit(initialDraft.checklist_inherit);
      setChecklistItems(
        initialDraft.checklist_inherit ? [] : [...initialDraft.checklistItems],
      );
    } else {
      setTitle("");
      setPrompt("");
      setPriority("");
      setChecklistInherit(false);
      setChecklistItems([]);
    }
  }, [instanceKey, initialDraft]);

  const hideChecklist = checklistInherit;
  const idsPrefix = `nested-sub-${instanceKey}`;

  function appendCriterion(text: string) {
    const t = text.trim();
    if (!t) return;
    setChecklistItems((prev) => [...prev, t]);
  }

  function removeRow(index: number) {
    setChecklistItems((prev) => prev.filter((_, i) => i !== index));
  }

  function updateRow(index: number, text: string) {
    const t = text.trim();
    if (!t) return;
    setChecklistItems((prev) => prev.map((x, i) => (i === index ? t : x)));
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!title.trim() || !priority) return;
    onSave({
      title: title.trim(),
      initial_prompt: prompt,
      priority,
      checklistItems: checklistItems.map((x) => x.trim()).filter(Boolean),
      checklist_inherit: checklistInherit,
    });
  }

  return (
    <Modal
      onClose={onClose}
      labelledBy="nested-subtask-draft-title"
      size="wide"
      stack="nested"
      lockBodyScroll={false}
    >
      <section className="panel modal-sheet modal-sheet--edit task-subtask-modal-sheet task-create">
        <h2 id="nested-subtask-draft-title">New subtask</h2>
        <form
          className="task-subtask-modal-form task-create-form"
          onSubmit={handleSubmit}
        >
          <TaskComposeFields
            idsPrefix={idsPrefix}
            editorKey={`nested-sub-editor-${instanceKey}`}
            title={title}
            prompt={prompt}
            priority={priority}
            checklistItems={checklistItems}
            hideChecklist={hideChecklist}
            disabled={false}
            onTitleChange={setTitle}
            onPromptChange={setPrompt}
            onPriorityChange={setPriority}
            onAppendChecklistCriterion={appendCriterion}
            onUpdateChecklistRow={updateRow}
            onRemoveChecklistRow={removeRow}
          />
          <label className="checkbox-label task-subtask-inherit">
            <input
              type="checkbox"
              checked={checklistInherit}
              onChange={(ev) => {
                const v = ev.target.checked;
                setChecklistInherit(v);
                if (v) {
                  setChecklistItems([]);
                }
              }}
            />
            <span className="checkbox-label-body">
              <span>Inherit parent task&apos;s checklist criteria</span>
              <FieldRequirementBadge requirement="optional" />
            </span>
          </label>
          <div className="row stack-row-actions task-subtask-modal-actions">
            <button type="button" className="secondary" onClick={onClose}>
              Cancel
            </button>
            <button
              type="submit"
              className="task-create-submit"
              disabled={!title.trim() || !priority}
            >
              Add subtask
            </button>
          </div>
        </form>
      </section>
    </Modal>
  );
}
