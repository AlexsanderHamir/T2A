import { useEffect, useState, type FormEvent } from "react";
import type { Priority } from "@/types";
import { Modal } from "../../shared/Modal";
import {
  emptyPendingSubtaskDraft,
  type PendingSubtaskDraft,
} from "../pendingSubtaskDraft";
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
  const [priority, setPriority] = useState<Priority>("medium");
  const [checklistDraft, setChecklistDraft] = useState("");
  const [checklistItems, setChecklistItems] = useState<string[]>([]);
  const [checklistInherit, setChecklistInherit] = useState(false);

  useEffect(() => {
    const base = initialDraft ?? emptyPendingSubtaskDraft();
    setTitle(base.title);
    setPrompt(base.initial_prompt);
    setPriority(base.priority);
    setChecklistInherit(base.checklist_inherit);
    setChecklistItems(
      base.checklist_inherit ? [] : [...base.checklistItems],
    );
    setChecklistDraft("");
  }, [instanceKey, initialDraft]);

  const hideChecklist = checklistInherit;
  const idsPrefix = `nested-sub-${instanceKey}`;

  function addRow() {
    const t = checklistDraft.trim();
    if (!t) return;
    setChecklistItems((prev) => [...prev, t]);
    setChecklistDraft("");
  }

  function removeRow(index: number) {
    setChecklistItems((prev) => prev.filter((_, i) => i !== index));
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;
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
            checklistDraft={checklistDraft}
            checklistItems={checklistItems}
            hideChecklist={hideChecklist}
            disabled={false}
            onTitleChange={setTitle}
            onPromptChange={setPrompt}
            onPriorityChange={setPriority}
            onChecklistDraftChange={setChecklistDraft}
            onAddChecklistRow={addRow}
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
                  setChecklistDraft("");
                  setChecklistItems([]);
                }
              }}
            />
            <span>Inherit parent task&apos;s checklist criteria</span>
          </label>
          <div className="row stack-row-actions task-subtask-modal-actions">
            <button type="button" className="secondary" onClick={onClose}>
              Cancel
            </button>
            <button
              type="submit"
              className="task-create-submit"
              disabled={!title.trim()}
            >
              Add subtask
            </button>
          </div>
        </form>
      </section>
    </Modal>
  );
}
