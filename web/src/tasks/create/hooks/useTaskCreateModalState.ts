import { useCallback, useRef, useState, type Dispatch, type SetStateAction } from "react";
import { listChecklist } from "@/api";
import { DEFAULT_NEW_TASK_STATUS, type ChecklistItemDraft, type Status, type Task } from "@/types";
import type { CreateModalPrefill } from "../types";

export function useTaskCreateModalState(
  resetFormFields: () => void,
  populateFromTask: (t: Task) => void,
  setNewChecklistItems: Dispatch<SetStateAction<ChecklistItemDraft[]>>,
  setNewProjectID: (id: string) => void,
) {
  const createModalPrefillRef = useRef<CreateModalPrefill | null>(null);
  const [draftPickerOpen, setDraftPickerOpen] = useState(false);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [editingTaskId, setEditingTaskId] = useState<string | null>(null);
  const [editingTemplateId, setEditingTemplateId] = useState<string | null>(null);
  const [composeTarget, setComposeTarget] = useState<"task" | "template">("task");
  const [composeOperation, setComposeOperation] = useState<"create" | "edit">("create");
  const [editingTaskRunner, setEditingTaskRunner] = useState("");
  const [composeStatus, setComposeStatus] = useState<Status>(DEFAULT_NEW_TASK_STATUS);
  const [createModalAssignmentLocked, setCreateModalAssignmentLocked] = useState(false);
  const [createEntryDraftErrorHint, setCreateEntryDraftErrorHint] = useState<
    string | null
  >(null);
  const [repositorySetupPromptOpen, setRepositorySetupPromptOpen] = useState(false);

  const resetNewTaskForm = useCallback(() => {
    resetFormFields();
    setCreateModalAssignmentLocked(false);
    setEditingTaskId(null);
    setEditingTemplateId(null);
    setComposeTarget("task");
    setComposeOperation("create");
    setEditingTaskRunner("");
    setComposeStatus(DEFAULT_NEW_TASK_STATUS);
  }, [resetFormFields]);

  const applyCreateModalPrefill = useCallback(() => {
    const prefill = createModalPrefillRef.current;
    if (!prefill?.projectID) return;
    setNewProjectID(prefill.projectID);
    setCreateModalAssignmentLocked(prefill.lockProjectAssignment);
    createModalPrefillRef.current = null;
  }, [setNewProjectID]);

  const closeCreateModal = useCallback(() => {
    createModalPrefillRef.current = null;
    setCreateModalOpen(false);
    setDraftPickerOpen(false);
    setCreateEntryDraftErrorHint(null);
    setRepositorySetupPromptOpen(false);
    resetNewTaskForm();
  }, [resetNewTaskForm]);

  const beginEditSession = useCallback(
    async (t: Task) => {
      populateFromTask(t);
      setEditingTaskId(t.id);
      setEditingTemplateId(null);
      setComposeTarget("task");
      setComposeOperation("edit");
      setEditingTaskRunner(t.runner);
      setComposeStatus(t.status);
      setNewChecklistItems([]);
      setCreateModalOpen(true);
      setDraftPickerOpen(false);
      setCreateEntryDraftErrorHint(null);
      try {
        const { items } = await listChecklist(t.id);
        setNewChecklistItems(
          items.map((item) => ({
            text: item.text,
            verify_commands: item.verify_commands,
          })),
        );
      } catch {
        // Checklist is display-only in edit; leave empty on fetch failure.
      }
    },
    [populateFromTask, setNewChecklistItems],
  );

  return {
    createModalPrefillRef,
    draftPickerOpen,
    setDraftPickerOpen,
    createModalOpen,
    setCreateModalOpen,
    editingTaskId,
    editingTemplateId,
    setEditingTemplateId,
    composeTarget,
    setComposeTarget,
    composeOperation,
    setComposeOperation,
    editingTaskRunner,
    composeStatus,
    setComposeStatus,
    createModalAssignmentLocked,
    setCreateModalAssignmentLocked,
    createEntryDraftErrorHint,
    setCreateEntryDraftErrorHint,
    repositorySetupPromptOpen,
    setRepositorySetupPromptOpen,
    applyCreateModalPrefill,
    resetNewTaskForm,
    closeCreateModal,
    beginEditSession,
  };
}
