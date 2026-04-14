import type { PendingSubtaskDraft } from "../../pendingSubtaskDraft";
import { NestedSubtaskDraftModal } from "../task-compose/NestedSubtaskDraftModal";

type Props = {
  open: boolean;
  instanceKey: number;
  initialDraft: PendingSubtaskDraft | null;
  onClose: () => void;
  onSave: (d: PendingSubtaskDraft) => void;
};

export function TaskCreateModalNestedSubtaskModal({
  open,
  instanceKey,
  initialDraft,
  onClose,
  onSave,
}: Props) {
  if (!open) return null;

  return (
    <NestedSubtaskDraftModal
      instanceKey={instanceKey}
      initialDraft={initialDraft}
      onClose={onClose}
      onSave={onSave}
    />
  );
}
