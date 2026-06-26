import { useCallback, useMemo, useState } from "react";
import type { ProjectContextEdge, ProjectContextItem } from "@/types";
import { Modal } from "@/shared/Modal";
import { ProjectContextListView } from "./ProjectContextListView";
import { ProjectContextTreeView } from "./ProjectContextTreeView";
import { ProjectContextChoiceDialog } from "@/components/project-context";
import {
  expandProjectContextSelection,
  mergeProjectContextSelection,
  projectContextShortId,
  selectedProjectContextItems,
  type ProjectContextAddMode,
} from "@/lib/projectContextRefs";
import { useProjectContext } from "./hooks";

interface ProjectContextPickerProps {
  projectId: string;
  selectedIds: string[];
  disabled?: boolean;
  /** Shorter copy for the create-task modal. */
  compact?: boolean;
  onChange: (ids: string[]) => void;
}

type ContextChooserView = "list" | "tree";

const EMPTY_CONTEXT_ITEMS: ProjectContextItem[] = [];
const EMPTY_CONTEXT_EDGES: ProjectContextEdge[] = [];

/**
 * Compact summary of the project context items currently attached to the
 * task plus an entry point into the full chooser. The chooser shares the
 * `Reference only this node / Reference this node and its children` choice
 * with the editor's `#` mention flow so both surfaces produce the same
 * canonical `project_context_item_ids`.
 *
 * The displayed labels intentionally include a 6-character short id so
 * operators can disambiguate same-titled nodes from different projects
 * without opening every detail panel.
 */
export function ProjectContextPicker({
  projectId,
  selectedIds,
  disabled,
  compact = false,
  onChange,
}: ProjectContextPickerProps) {
  const [chooserOpen, setChooserOpen] = useState(false);
  const [contextView, setContextView] = useState<ContextChooserView>("list");
  const [pendingChoice, setPendingChoice] = useState<ProjectContextItem | null>(
    null,
  );

  const contextQuery = useProjectContext(projectId, {
    enabled: Boolean(projectId),
    limit: 100,
    pinnedOnly: false,
  });
  const items = contextQuery.data?.items ?? EMPTY_CONTEXT_ITEMS;
  const edges = contextQuery.data?.edges ?? EMPTY_CONTEXT_EDGES;
  const selected = useMemo(() => new Set(selectedIds), [selectedIds]);
  const selectedItems = useMemo(
    () => selectedProjectContextItems(items, selectedIds),
    [items, selectedIds],
  );
  const selectedCountLabel =
    selectedIds.length === 1 ? "1 node selected" : `${selectedIds.length} nodes selected`;

  const handleToggle = useCallback(
    (item: ProjectContextItem) => {
      if (disabled) return;
      if (selected.has(item.id)) {
        // Toggling off is a single-click remove — no confirmation dialog so
        // operators can reverse a misclick fast.
        onChange(selectedIds.filter((id) => id !== item.id));
        return;
      }
      // Adding always funnels through the choice dialog so operators have
      // one consistent mental model whether they're using `#` from the
      // prompt or this chooser.
      setPendingChoice(item);
    },
    [disabled, onChange, selected, selectedIds],
  );

  const handleRemoveSelected = useCallback(
    (id: string) => {
      if (disabled) return;
      const next = selectedIds.filter((existing) => existing !== id);
      if (next.length === selectedIds.length) return;
      onChange(next);
    },
    [disabled, onChange, selectedIds],
  );

  const handleConfirmChoice = useCallback(
    (mode: ProjectContextAddMode) => {
      if (!pendingChoice) return;
      const expanded = expandProjectContextSelection(
        pendingChoice.id,
        mode,
        edges,
      );
      onChange(mergeProjectContextSelection(selectedIds, expanded));
      setPendingChoice(null);
    },
    [edges, onChange, pendingChoice, selectedIds],
  );

  if (!projectId) return null;

  return (
    <section
      className={[
        "project-context-picker",
        compact ? "project-context-picker--compact" : "",
      ]
        .filter(Boolean)
        .join(" ")}
      aria-labelledby="task-context-picker-title"
    >
      <div className="project-context-picker__head">
        <div>
          <h3 id="task-context-picker-title">
            {compact ? "Project context" : "Context for this task"}
          </h3>
          {compact ? (
            <p className="project-context-picker__lede">
              Type <kbd>#</kbd> in the prompt or choose nodes below.
            </p>
          ) : (
            <p>
              Reference project memory the agent may use. Add from the prompt
              with <kbd>#</kbd> or open the chooser. Backend resolves the full
              node memory at run time — chips here are display labels only.
            </p>
          )}
        </div>
        <button
          type="button"
          className="pc__btn-secondary project-context-picker__button"
          disabled={disabled}
          onClick={() => setChooserOpen(true)}
        >
          {compact ? "Choose" : "Choose context"}
        </button>
      </div>

      <div
        className="project-context-picker__summary"
        aria-live="polite"
        data-project-references-summary="true"
      >
        <strong>{selectedCountLabel}</strong>
        {contextQuery.isPending ? (
          <span>Loading project context...</span>
        ) : contextQuery.error ? (
          // The error is also surfaced inside the chooser modal with a real
          // `role="alert"`. Keeping the summary line non-alert prevents
          // duplicated alert nodes in tests and avoids shouting at the user
          // before they've taken any action.
          <span className="muted">Project context unavailable.</span>
        ) : items.length === 0 ? (
          <span>This project has no context nodes yet.</span>
        ) : selectedItems.length > 0 ? (
          <ul className="project-context-picker__chips">
            {selectedItems.map((item) => {
              const shortId = projectContextShortId(item.id);
              return (
                <li
                  key={item.id}
                  className="project-context-picker__chip"
                  data-project-context-id={item.id}
                >
                  <span className="project-context-picker__chip-title">
                    {item.title || "(untitled)"}
                  </span>
                  {shortId ? (
                    <span className="project-context-picker__chip-short-id muted">
                      · {shortId}
                    </span>
                  ) : null}
                  <button
                    type="button"
                    className="project-context-picker__chip-remove"
                    onClick={() => handleRemoveSelected(item.id)}
                    disabled={disabled}
                    aria-label={`Remove reference to ${item.title || "context node"}`}
                  >
                    <svg
                      width="10"
                      height="10"
                      viewBox="0 0 12 12"
                      fill="none"
                      aria-hidden="true"
                    >
                      <path
                        d="M3 3l6 6M9 3l-6 6"
                        stroke="currentColor"
                        strokeWidth="1.4"
                        strokeLinecap="round"
                      />
                    </svg>
                  </button>
                </li>
              );
            })}
          </ul>
        ) : (
          <span>
            {compact
              ? "None selected"
              : "Open the chooser to search the list or inspect the tree."}
          </span>
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
                  Search project memory or inspect the tree. Picking a node
                  asks whether to reference just that node or also include
                  its children.
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
              <div
                className="pc__view-toggle"
                role="tablist"
                aria-label="Context chooser view"
              >
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
                  <span>
                    Add project memory from the project context page first.
                  </span>
                </div>
              ) : contextView === "list" ? (
                <ProjectContextListView
                  items={items}
                  onAddConnection={() => undefined}
                  selection={{
                    selectedIds: selected,
                    disabled,
                    onToggle: handleToggle,
                  }}
                />
              ) : (
                <ProjectContextTreeView
                  items={items}
                  edges={edges}
                  selection={{
                    selectedIds: selected,
                    disabled,
                    onToggle: handleToggle,
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

      {pendingChoice ? (
        <ProjectContextChoiceDialog
          item={pendingChoice}
          edges={edges}
          selectedIds={selectedIds}
          onClose={() => setPendingChoice(null)}
          onConfirm={handleConfirmChoice}
        />
      ) : null}
    </section>
  );
}
