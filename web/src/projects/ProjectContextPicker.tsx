import { useMemo, useState } from "react";
import type { ProjectContextEdge, ProjectContextItem } from "@/types";
import { Modal } from "@/shared/Modal";
import { ProjectContextListView } from "./ProjectContextListView";
import { ProjectContextTreeView } from "./ProjectContextTreeView";
import { useProjectContext } from "./hooks";

interface ProjectContextPickerProps {
  projectId: string;
  selectedIds: string[];
  disabled?: boolean;
  onChange: (ids: string[]) => void;
}

type ContextChooserView = "list" | "tree";

const EMPTY_CONTEXT_ITEMS: ProjectContextItem[] = [];
const EMPTY_CONTEXT_EDGES: ProjectContextEdge[] = [];

export function ProjectContextPicker({
  projectId,
  selectedIds,
  disabled,
  onChange,
}: ProjectContextPickerProps) {
  const [chooserOpen, setChooserOpen] = useState(false);
  const [contextView, setContextView] = useState<ContextChooserView>("list");
  const contextQuery = useProjectContext(projectId, {
    enabled: Boolean(projectId),
    limit: 100,
    pinnedOnly: false,
  });
  const items = contextQuery.data?.items ?? EMPTY_CONTEXT_ITEMS;
  const edges = contextQuery.data?.edges ?? EMPTY_CONTEXT_EDGES;
  const selected = useMemo(() => new Set(selectedIds), [selectedIds]);
  const selectedItems = useMemo(() => {
    const byID = new Map(items.map((item) => [item.id, item]));
    return selectedIds.map((id) => byID.get(id)).filter(Boolean) as ProjectContextItem[];
  }, [items, selectedIds]);
  const selectedCountLabel =
    selectedIds.length === 1 ? "1 node selected" : `${selectedIds.length} nodes selected`;

  if (!projectId) return null;

  function toggle(item: ProjectContextItem) {
    if (disabled) return;
    if (selected.has(item.id)) {
      onChange(selectedIds.filter((id) => id !== item.id));
      return;
    }
    onChange([...selectedIds, item.id]);
  }

  return (
    <section className="project-context-picker" aria-labelledby="task-context-picker-title">
      <div className="project-context-picker__head">
        <div>
          <h3 id="task-context-picker-title">Context for this task</h3>
          <p>
            Choose the exact project context nodes the agent may use. Nothing is
            selected automatically.
          </p>
        </div>
        <button
          type="button"
          className="pc__btn-secondary project-context-picker__button"
          disabled={disabled}
          onClick={() => setChooserOpen(true)}
        >
          Choose context
        </button>
      </div>

      <div className="project-context-picker__summary" aria-live="polite">
        <strong>{selectedCountLabel}</strong>
        {contextQuery.isPending ? (
          <span>Loading project context...</span>
        ) : items.length === 0 ? (
          <span>This project has no context nodes yet.</span>
        ) : selectedItems.length > 0 ? (
          <span>{selectedItems.map((item) => item.title).join(", ")}</span>
        ) : (
          <span>Open the chooser to search the list or inspect the tree.</span>
        )}
      </div>

      {chooserOpen ? (
        <Modal
          onClose={() => setChooserOpen(false)}
          labelledBy="task-context-chooser-title"
          describedBy="task-context-chooser-desc"
          size="wide"
        >
          <section className="panel modal-sheet modal-sheet--edit project-context-chooser pc">
            <div className="project-context-chooser__header">
              <div>
                <h2 id="task-context-chooser-title">Choose task context</h2>
                <p id="task-context-chooser-desc" className="muted">
                  Search the project memory list or inspect the tree before
                  selecting what this task can use.
                </p>
              </div>
              <button
                type="button"
                className="pc__btn-ghost"
                onClick={() => setChooserOpen(false)}
              >
                Done
              </button>
            </div>

            <div className="pc__action-bar project-context-chooser__bar">
              <span className="pc__count">{selectedCountLabel}</span>
              <div className="pc__view-toggle" role="tablist" aria-label="Context chooser view">
                <button
                  type="button"
                  role="tab"
                  aria-selected={contextView === "list"}
                  onClick={() => setContextView("list")}
                >
                  List
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={contextView === "tree"}
                  onClick={() => setContextView("tree")}
                >
                  Tree
                </button>
              </div>
            </div>

            <div className="project-context-chooser__body">
              {contextQuery.isLoading ? (
                <div className="pc__skeleton" aria-hidden="true">
                  <div className="pd__shimmer pd__shimmer--card" />
                </div>
              ) : contextQuery.error ? (
                <div className="pd__inline-error" role="alert">
                  {contextQuery.error.message}
                </div>
              ) : items.length === 0 ? (
                <div className="pc__empty">
                  <p>No context nodes yet</p>
                  <span>Add project memory from the project context page first.</span>
                </div>
              ) : contextView === "list" ? (
                <ProjectContextListView
                  items={items}
                  onAddConnection={() => undefined}
                  selection={{
                    selectedIds: selected,
                    disabled,
                    onToggle: toggle,
                  }}
                />
              ) : (
                <ProjectContextTreeView
                  items={items}
                  edges={edges}
                  selection={{
                    selectedIds: selected,
                    disabled,
                    onToggle: toggle,
                  }}
                />
              )}
            </div>

            <div className="project-context-chooser__footer">
              <button
                type="button"
                className="pc__btn-secondary"
                disabled={disabled || selectedIds.length === 0}
                onClick={() => onChange([])}
              >
                Clear selection
              </button>
              <button
                type="button"
                className="pc__btn-primary"
                onClick={() => setChooserOpen(false)}
              >
                Done
              </button>
            </div>
          </section>
        </Modal>
      ) : null}
    </section>
  );
}
