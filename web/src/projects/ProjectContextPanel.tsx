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
    <section className="task-attempt-section">
      <div className="project-context-panel-header">
        <div>
          <h3>Project context</h3>
          <p className="muted">
            Add project-owned memory, then inspect it as a list or graph.
          </p>
        </div>
        <button type="button" onClick={() => setAddNodeOpen(true)}>
          Add node
        </button>
      </div>
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
                  handoff notes. Add them anytime as the project evolves.
                </p>
              </div>
            </div>
            <ProjectContextKindPicker
              idPrefix="project-context-kind"
              disabled={createContextMutation.isPending}
            />
            <div className="field grow">
              <label htmlFor="project-context-title">Title</label>
              <input id="project-context-title" name="title" required />
            </div>
            <div className="field grow">
              <FieldLabel
                id="project-context-body-label"
                htmlFor="project-context-body"
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
            <strong>Add two nodes to unlock connections</strong>
            <p>
              Connections describe how project memory relates. Once this project
              has at least two nodes, you can connect them with a relation and
              strength.
            </p>
          </div>
        </div>
      ) : (
        <form className="project-context-form" onSubmit={submitEdge}>
          <div className="project-context-form__heading">
            <div>
              <strong>Add connection</strong>
              <p className="muted">
                Connect two project nodes with an explicit relationship. Tasks only
                receive connections between nodes the user selected.
              </p>
            </div>
          </div>
          <div className="project-context-edge-grid">
            <div className="field grow">
              <label htmlFor="project-context-edge-source">From node</label>
              <select
                id="project-context-edge-source"
                name="source_context_id"
                required
              >
                <option value="">Select source</option>
                {items.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.title}
                  </option>
                ))}
              </select>
            </div>
            <div className="field grow">
              <label htmlFor="project-context-edge-target">To node</label>
              <select
                id="project-context-edge-target"
                name="target_context_id"
                required
              >
                <option value="">Select target</option>
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
                  placeholder="Why does this connection matter? Type @ to reference a repo file."
                />
              </div>
            </div>
          </div>
          <button type="submit" disabled={createEdgeMutation.isPending}>
            {createEdgeMutation.isPending ? "Connecting..." : "Add connection"}
          </button>
        </form>
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
          density="compact"
          hideIcon
        />
      ) : (
        <>
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
