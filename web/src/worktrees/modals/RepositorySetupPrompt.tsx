import { useNavigate } from "react-router-dom";
import { Modal } from "@/shared/Modal";

type Props = {
  open: boolean;
  onClose: () => void;
};

export function RepositorySetupPrompt({ open, onClose }: Props) {
  const navigate = useNavigate();

  if (!open) return null;

  return (
    <Modal
      onClose={onClose}
      labelledBy="repository-setup-prompt-title"
      describedBy="repository-setup-prompt-lead"
    >
      <section className="panel modal-sheet worktrees-form-modal">
        <header className="worktrees-form-modal__header">
          <h2 id="repository-setup-prompt-title">Register a repository first</h2>
          <p id="repository-setup-prompt-lead" className="worktrees-form-modal__lead">
            Hamix needs a registered git checkout before you can create tasks with worktrees
            and branches.
          </p>
        </header>
        <div className="row stack-row-actions">
          <button type="button" className="secondary" onClick={onClose}>
            Cancel
          </button>
          <button
            type="button"
            className="btn-primary"
            onClick={() => {
              onClose();
              navigate("/worktrees?register=1");
            }}
          >
            Register repository
          </button>
        </div>
      </section>
    </Modal>
  );
}
