import type { FormEvent } from "react";
import type { Task } from "@/types";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { TaskCreateModalAgentSection } from "../../task-create-modal/fields/TaskCreateModalAgentSection";

type Props = {
  task: Task;
  cursorModel: string;
  onCursorModelChange: (v: string) => void;
  saving: boolean;
  patchPending: boolean;
  error?: string | null;
  onSubmit: (e: FormEvent) => void;
  onCancel: () => void;
};

export function TaskChangeModelModal({
  task,
  cursorModel,
  onCursorModelChange,
  saving,
  patchPending,
  error = null,
  onSubmit,
  onCancel,
}: Props) {
  return (
    <Modal
      onClose={onCancel}
      labelledBy="task-change-model-title"
      describedBy="task-change-model-desc"
      size="wide"
      busy={patchPending}
      dismissibleWhileBusy
    >
      <section className="panel modal-sheet modal-sheet--edit task-change-model-modal">
        <h2 id="task-change-model-title" className="term-arrow">
          <span>Change model</span>
        </h2>
        <p id="task-change-model-desc" className="task-change-model-modal-lede muted">
          Per-task override for <strong>{task.title}</strong>.{" "}
          <span className="task-change-model-modal-lede-sub">
            Choose Default to follow the workspace model from settings.
          </span>
        </p>
        <form onSubmit={(e) => void onSubmit(e)}>
          <TaskCreateModalAgentSection
            key={task.id}
            variant="modelDialog"
            disabled={saving}
            lockRunner
            runner={task.runner}
            cursorModel={cursorModel}
            onRunnerChange={() => {}}
            onCursorModelChange={onCursorModelChange}
          />
          <MutationErrorBanner error={error} className="task-edit-form-err" />
          <div className="row stack-row-actions">
            <button type="submit" disabled={saving}>
              Save
            </button>
            <button
              type="button"
              className="secondary"
              disabled={saving}
              onClick={onCancel}
            >
              Cancel
            </button>
          </div>
        </form>
      </section>
    </Modal>
  );
}
