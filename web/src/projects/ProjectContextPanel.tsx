import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import {
  createProjectContext,
  createProjectContextEdge,
  deleteProjectContext,
  deleteProjectContextEdge,
  patchProjectContext,
  patchProjectContextEdge,
} from "@/api";
import { EmptyState } from "@/shared/EmptyState";
import { FieldLabel } from "@/shared/FieldLabel";
import { Modal } from "@/shared/Modal";
import { RichPromptEditor } from "@/tasks/components/rich-prompt";
import { promptHasVisibleContent } from "@/tasks/task-prompt";
import {
  PROJECT_CONTEXT_RELATIONS,
  type ProjectContextEdge,
  type ProjectContextKind,
  type ProjectContextItem,
  type ProjectContextRelation,
} from "@/types";
import { useProjectContext } from "./hooks";
import { ProjectContextGraphView } from "./ProjectContextGraphView";
import { ProjectContextKindPicker } from "./ProjectContextKindPicker";
import { ProjectContextListView } from "./ProjectContextListView";
import { projectQueryKeys } from "./queryKeys";

type Props = {
  projectId: string;
};

const EMPTY_CONTEXT_ITEMS: ProjectContextItem[] = [];
const EMPTY_CONTEXT_EDGES: ProjectContextEdge[] = [];
type ContextView = "list" | "graph";

export function ProjectContextPanel({ projectId }: Props) {
  const queryClient = useQueryClient();
  const context = useProjectContext(projectId, { enabled: Boolean(projectId) });
  const [contextView, setContextView] = useState<ContextView>("list");
  const [addNodeOpen, setAddNodeOpen] = useState(false);
  const [newNodeBody, setNewNodeBody] = useState("");
  const [newNodeEditorKey, setNewNodeEditorKey] = useState(0);
  const [newEdgeNote, setNewEdgeNote] = useState("");
  const [newEdgeEditorKey, setNewEdgeEditorKey] = useState(0);
  const createContextMutation = useMutation({
    mutationFn: (input: {
      kind: ProjectContextKind;
      title: string;
      body: string;
      pinned: boolean;
    }) => createProjectContext(projectId, input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: projectQueryKeys.context(projectId),
      });
    },
  });
  const patchContextMutation = useMutation({
    mutationFn: (input: {
      id: string;
      kind: ProjectContextKind;
      title: string;
      body: string;
      pinned: boolean;
    }) => {
      const { id, ...patch } = input;
      return patchProjectContext(projectId, id, patch);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: projectQueryKeys.context(projectId),
      });
    },
  });
  const deleteContextMutation = useMutation({
    mutationFn: (contextId: string) => deleteProjectContext(projectId, contextId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: projectQueryKeys.context(projectId),
      });
    },
  });
  const createEdgeMutation = useMutation({
    mutationFn: (input: {
      source_context_id: string;
      target_context_id: string;
      relation: ProjectContextRelation;
      strength: number;
      note: string;
    }) => createProjectContextEdge(projectId, input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: projectQueryKeys.context(projectId),
      });
    },
  });
  const patchEdgeMutation = useMutation({
    mutationFn: (input: {
      id: string;
      relation: ProjectContextRelation;
      strength: number;
      note: string;
    }) => {
      const { id, ...patch } = input;
      return patchProjectContextEdge(projectId, id, patch);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: projectQueryKeys.context(projectId),
      });
    },
  });
  const deleteEdgeMutation = useMutation({
    mutationFn: (edgeId: string) => deleteProjectContextEdge(projectId, edgeId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: projectQueryKeys.context(projectId),
      });
    },
  });

  function submitContext(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const title = String(form.get("title") ?? "").trim();
    const body = newNodeBody.trim();
    if (!title || !promptHasVisibleContent(body)) return;
    const formEl = event.currentTarget;
    createContextMutation.mutate(
      {
        kind: String(form.get("kind") ?? "note") as ProjectContextKind,
        title,
        body,
        pinned: false,
      },
      {
        onSuccess: () => {
          formEl.reset();
          setNewNodeBody("");
          setNewNodeEditorKey((value) => value + 1);
          setAddNodeOpen(false);
        },
      },
    );
  }

  function submitEdge(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const sourceContextID = String(form.get("source_context_id") ?? "").trim();
    const targetContextID = String(form.get("target_context_id") ?? "").trim();
    if (!sourceContextID || !targetContextID || sourceContextID === targetContextID) {
      return;
    }
    const formEl = event.currentTarget;
    createEdgeMutation.mutate(
      {
        source_context_id: sourceContextID,
        target_context_id: targetContextID,
        relation: String(
          form.get("relation") ?? "related",
        ) as ProjectContextRelation,
        strength: Number(form.get("strength") ?? 3),
        note: newEdgeNote.trim(),
      },
      {
        onSuccess: () => {
          formEl.reset();
          setNewEdgeNote("");
          setNewEdgeEditorKey((value) => value + 1);
        },
      },
    );
  }

  const mutationError =
    createContextMutation.error ??
    patchContextMutation.error ??
    deleteContextMutation.error ??
    createEdgeMutation.error ??
    patchEdgeMutation.error ??
    deleteEdgeMutation.error;
  const items = context.data?.items ?? EMPTY_CONTEXT_ITEMS;
  const edges = context.data?.edges ?? EMPTY_CONTEXT_EDGES;

  return (
    <section className="task-attempt-section project-context-workspace">
      <ol className="project-context-guide" aria-label="Project memory workflow">
        <li>
          <span>1</span>
          <strong>Capture</strong>
          <p>Facts, decisions, constraints.</p>
        </li>
        <li>
          <span>2</span>
          <strong>Connect</strong>
          <p>Only when a relationship matters.</p>
        </li>
        <li>
          <span>3</span>
          <strong>Reuse</strong>
          <p>Select memory per task.</p>
        </li>
      </ol>
      {addNodeOpen ? (
        <Modal
          onClose={() => setAddNodeOpen(false)}
          labelledBy="project-context-add-node-title"
          describedBy="project-context-add-node-desc"
          size="wide"
          busy={createContextMutation.isPending}
          busyLabel="Adding node..."
        >
          <form
            className="panel modal-sheet modal-sheet--edit project-context-form project-context-node-modal"
            onSubmit={submitContext}
          >
            <div className="project-context-form__heading">
              <div>
                <h2 id="project-context-add-node-title">Add memory node</h2>
                <p id="project-context-add-node-desc" className="muted">
                  Nodes are project-owned facts, decisions, constraints, or
                  custom context. All fields are required.
                </p>
              </div>
            </div>
            <ProjectContextKindPicker
              idPrefix="project-context-kind"
              disabled={createContextMutation.isPending}
            />
            <div className="field grow">
              <FieldLabel htmlFor="project-context-title" requirement="required">
                Title
              </FieldLabel>
              <input
                id="project-context-title"
                name="title"
                required
                aria-required="true"
              />
            </div>
            <div className="field grow">
              <FieldLabel
                id="project-context-body-label"
                htmlFor="project-context-body"
                requirement="required"
              >
                Body
              </FieldLabel>
              <div className="project-context-editor-shell">
                <RichPromptEditor
                  key={newNodeEditorKey}
                  id="project-context-body"
                  value={newNodeBody}
                  onChange={setNewNodeBody}
                  disabled={createContextMutation.isPending}
                  placeholder="Write markdown-style context. Type @ to reference a repo file."
                />
              </div>
            </div>
            <div className="row stack-row-actions">
              <button type="submit" disabled={createContextMutation.isPending}>
                {createContextMutation.isPending ? "Adding..." : "Add node"}
              </button>
              <button
                type="button"
                className="secondary"
                disabled={createContextMutation.isPending}
                onClick={() => setAddNodeOpen(false)}
              >
                Cancel
              </button>
            </div>
          </form>
        </Modal>
      ) : null}
      {items.length < 2 ? (
        <div className="project-context-ready-card">
          <span className="project-context-ready-card__step">Next</span>
          <div>
            <strong>Add two memories to link them</strong>
            <p>Links are optional. Use them only when the relationship helps a task.</p>
          </div>
        </div>
      ) : (
        <details className="project-context-disclosure">
          <summary>
            <span>Add connection</span>
            <small>Optional</small>
          </summary>
          <form className="project-context-form" onSubmit={submitEdge}>
            <div className="project-context-edge-grid">
              <div className="field grow">
                <label htmlFor="project-context-edge-source">From</label>
                <select
                  id="project-context-edge-source"
                  name="source_context_id"
                  required
                >
                  <option value="">Select memory</option>
                  {items.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.title}
                    </option>
                  ))}
                </select>
              </div>
              <div className="field grow">
                <label htmlFor="project-context-edge-target">To</label>
                <select
                  id="project-context-edge-target"
                  name="target_context_id"
                  required
                >
                  <option value="">Select memory</option>
                  {items.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.title}
                    </option>
                  ))}
                </select>
              </div>
              <div className="field">
                <label htmlFor="project-context-edge-relation">Relation</label>
                <select
                  id="project-context-edge-relation"
                  name="relation"
                  defaultValue="related"
                >
                  {PROJECT_CONTEXT_RELATIONS.map((relation) => (
                    <option key={relation} value={relation}>
                      {relation.replace("_", " ")}
                    </option>
                  ))}
                </select>
              </div>
              <div className="field">
                <label htmlFor="project-context-edge-strength">Strength</label>
                <select
                  id="project-context-edge-strength"
                  name="strength"
                  defaultValue="3"
                >
                  {[1, 2, 3, 4, 5].map((strength) => (
                    <option key={strength} value={strength}>
                      {strength}
                    </option>
                  ))}
                </select>
              </div>
              <div className="field grow project-context-edge-note">
                <FieldLabel
                  id="project-context-edge-note-label"
                  htmlFor="project-context-edge-note"
                >
                  Note
                </FieldLabel>
                <div className="project-context-editor-shell">
                  <RichPromptEditor
                    key={newEdgeEditorKey}
                    id="project-context-edge-note"
                    value={newEdgeNote}
                    onChange={setNewEdgeNote}
                    disabled={createEdgeMutation.isPending}
                    placeholder="Why does this link matter? Type @ for files."
                  />
                </div>
              </div>
            </div>
            <button type="submit" disabled={createEdgeMutation.isPending}>
              {createEdgeMutation.isPending ? "Connecting..." : "Add connection"}
            </button>
          </form>
        </details>
      )}
      {mutationError ? (
        <div className="err" role="alert">
          {mutationError.message}
        </div>
      ) : null}
      {context.isLoading ? (
        <p className="muted">Loading context...</p>
      ) : context.error ? (
        <div className="err" role="alert">
          {context.error.message}
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          title="No context nodes yet"
          description="Add durable project memory nodes, then connect them as the work evolves."
          action={{
            label: "Add memory",
            onClick: () => setAddNodeOpen(true),
          }}
          density="compact"
          hideIcon
        />
      ) : (
        <>
          <div className="project-context-display-bar">
            <div>
              <h4>Browse</h4>
            </div>
            <div className="project-context-display-actions">
              <button
                type="button"
                className="project-context-add-memory-button"
                onClick={() => setAddNodeOpen(true)}
              >
                Add memory
              </button>
              <div className="project-context-view-toggle" role="tablist" aria-label="Context view">
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
                  aria-selected={contextView === "graph"}
                  onClick={() => setContextView("graph")}
                >
                  Graph
                </button>
              </div>
            </div>
          </div>
          {contextView === "list" ? (
            <ProjectContextListView
              items={items}
              edges={edges}
              nodeSaving={patchContextMutation.isPending}
              nodeDeleting={deleteContextMutation.isPending}
              edgeSaving={patchEdgeMutation.isPending}
              edgeDeleting={deleteEdgeMutation.isPending}
              onSaveNode={(id, patch) => patchContextMutation.mutate({ id, ...patch })}
              onDeleteNode={(id) => deleteContextMutation.mutate(id)}
              onSaveEdge={(id, patch) => patchEdgeMutation.mutate({ id, ...patch })}
              onDeleteEdge={(id) => deleteEdgeMutation.mutate(id)}
            />
          ) : (
            <ProjectContextGraphView items={items} edges={edges} />
          )}
        </>
      )}
    </section>
  );
}
