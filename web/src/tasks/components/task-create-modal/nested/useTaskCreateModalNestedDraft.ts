import { useCallback, useState } from "react";
import type { PendingSubtaskDraft } from "../../../pendingSubtaskDraft";

type Args = {
  pendingSubtasks: PendingSubtaskDraft[];
  onAddPendingSubtask: (d: PendingSubtaskDraft) => void;
  onUpdatePendingSubtask: (index: number, d: PendingSubtaskDraft) => void;
};

export function useTaskCreateModalNestedDraft({
  pendingSubtasks,
  onAddPendingSubtask,
  onUpdatePendingSubtask,
}: Args) {
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

  return {
    nestedOpen,
    nestedInstanceKey,
    nestedInitial,
    openNestedNew,
    openNestedEdit,
    handleNestedClose,
    handleNestedSave,
  };
}
