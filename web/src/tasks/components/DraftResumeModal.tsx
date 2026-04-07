import { Modal } from "@/shared/Modal";
import type { TaskDraftSummary } from "@/types";

type Props = {
  drafts: TaskDraftSummary[];
  onStartFresh: () => void;
  onResume: (draftId: string) => void;
  onClose: () => void;
  resumePending?: boolean;
  resumeError?: string | null;
};

export function DraftResumeModal({
  drafts,
  onStartFresh,
  onResume,
  onClose,
  resumePending = false,
  resumeError = null,
}: Props) {
  return (
    <Modal onClose={onClose} labelledBy="draft-resume-modal-title" size="wide">
      <section className="panel modal-sheet modal-sheet--edit">
        <h2 id="draft-resume-modal-title">Resume a draft or start fresh</h2>
        <p className="muted">Pick an existing draft to continue, or start a new one.</p>
        {resumeError ? <p role="alert">{resumeError}</p> : null}
        <div className="stack">
          {drafts.map((d) => (
            <button
              key={d.id}
              type="button"
              className="secondary"
              onClick={() => onResume(d.id)}
              disabled={resumePending}
            >
              Resume: {d.name}
            </button>
          ))}
        </div>
        <div className="row stack-row-actions task-create-modal-actions">
          <button type="button" className="secondary" onClick={onClose} disabled={resumePending}>
            Cancel
          </button>
          <button
            type="button"
            className="task-create-submit"
            onClick={onStartFresh}
            disabled={resumePending}
          >
            Start fresh
          </button>
        </div>
      </section>
    </Modal>
  );
}
