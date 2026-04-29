import { useId, useMemo } from "react";
import type { ProjectContextEdge, ProjectContextItem } from "@/types";
import { Modal } from "@/shared/Modal";
import {
  expandProjectContextSelection,
  hasProjectContextChildren,
  projectContextShortId,
  type ProjectContextAddMode,
} from "./projectContextRefs";

type Props = {
  /** Node the user picked from the suggestion list or the chooser. */
  item: ProjectContextItem;
  /**
   * Project context edges so the dialog can preview how many items will be
   * added when the user chooses "Reference this node and its children".
   */
  edges: readonly ProjectContextEdge[];
  /**
   * IDs already selected for the task, used to grey out the descendant count
   * preview when every reachable child is already in the selection.
   */
  selectedIds: readonly string[];
  /** Optional `stack="nested"` for when the dialog opens above another modal. */
  modalStack?: "default" | "nested";
  onClose: () => void;
  onConfirm: (mode: ProjectContextAddMode) => void;
};

/**
 * Tiny single-step dialog every "add project context" gesture funnels through
 * (#-mention picker AND the legacy `Choose context` chooser). Asks the user
 * whether to reference just `item` or `item` plus all of its descendants
 * (outgoing project-context edges, with cycle protection).
 *
 * Lives next to the data layer it operates on so the dialog and the
 * `expandProjectContextSelection` helper can never drift apart.
 */
export function ProjectContextChoiceDialog({
  item,
  edges,
  selectedIds,
  modalStack = "nested",
  onClose,
  onConfirm,
}: Props) {
  const baseId = useId();
  const titleId = `${baseId}-title`;
  const descId = `${baseId}-desc`;

  const childrenAvailable = useMemo(
    () => hasProjectContextChildren(item.id, edges),
    [item.id, edges],
  );

  const withChildrenPreview = useMemo(
    () => expandProjectContextSelection(item.id, "withChildren", edges),
    [item.id, edges],
  );

  const newWithChildrenCount = useMemo(() => {
    const seen = new Set(selectedIds);
    let count = 0;
    for (const id of withChildrenPreview) {
      if (!seen.has(id)) count += 1;
    }
    return count;
  }, [selectedIds, withChildrenPreview]);

  const shortId = projectContextShortId(item.id);

  return (
    <Modal
      onClose={onClose}
      labelledBy={titleId}
      describedBy={descId}
      size="default"
      stack={modalStack}
      lockBodyScroll={modalStack !== "nested"}
    >
      <section className="panel modal-sheet project-context-choice">
        <header className="project-context-choice__header">
          <h2 id={titleId}>Reference project context</h2>
          <p id={descId} className="muted">
            Choose how much of <strong>{item.title}</strong>
            {shortId ? <span className="muted"> · {shortId}</span> : null} the
            agent should see when it picks up this task.
          </p>
        </header>

        <div className="project-context-choice__options">
          <button
            type="button"
            className="project-context-choice__option project-context-choice__option--node"
            onClick={() => onConfirm("nodeOnly")}
            data-testid="project-context-choice-node-only"
          >
            <span className="project-context-choice__option-title">
              Reference only this node
            </span>
            <span className="project-context-choice__option-help muted">
              Add a single reference to <strong>{item.title}</strong>.
            </span>
          </button>

          <button
            type="button"
            className="project-context-choice__option project-context-choice__option--children"
            onClick={() => onConfirm("withChildren")}
            disabled={!childrenAvailable}
            data-testid="project-context-choice-with-children"
          >
            <span className="project-context-choice__option-title">
              Reference this node and its children
            </span>
            <span className="project-context-choice__option-help muted">
              {childrenAvailable
                ? `Adds ${withChildrenPreview.length} reference${
                    withChildrenPreview.length === 1 ? "" : "s"
                  } total${
                    newWithChildrenCount === withChildrenPreview.length
                      ? ""
                      : ` (${newWithChildrenCount} new)`
                  }.`
                : "This node has no outgoing connections yet."}
            </span>
          </button>
        </div>

        <footer className="project-context-choice__footer">
          <button
            type="button"
            className="pc__btn-ghost"
            onClick={onClose}
            data-testid="project-context-choice-cancel"
          >
            Cancel
          </button>
        </footer>
      </section>
    </Modal>
  );
}
