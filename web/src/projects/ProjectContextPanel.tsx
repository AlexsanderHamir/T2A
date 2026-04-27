import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState, type FormEvent } from "react";
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
import { CustomSelect, type CustomSelectOption } from "@/tasks/components/custom-select";
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
import { ProjectContextKindPicker } from "./ProjectContextKindPicker";
import { ProjectContextListView } from "./ProjectContextListView";
import { ProjectContextTreeView } from "./ProjectContextTreeView";
import { projectQueryKeys } from "./queryKeys";

type Props = {
  projectId: string;
};

const EMPTY_CONTEXT_ITEMS: ProjectContextItem[] = [];
const EMPTY_CONTEXT_EDGES: ProjectContextEdge[] = [];
type ContextView = "list" | "tree";

export function ProjectContextPanel({ projectId }: Props) {
  const queryClient = useQueryClient();
  const context = useProjectContext(projectId, { enabled: Boolean(projectId) });
  const [contextView, setContextView] = useState<ContextView>("list");
  const [addNodeOpen, setAddNodeOpen] = useState(false);
  const [addEdgeOpen, setAddEdgeOpen] = useState(false);
  const [newNodeBody, setNewNodeBody] = useState("");
  const [newNodeEditorKey, setNewNodeEditorKey] = useState(0);
  const [newEdgeSourceID, setNewEdgeSourceID] = useState("");
  const [newEdgeTargetID, setNewEdgeTargetID] = useState("");
  const [newEdgeRelation, setNewEdgeRelation] =
    useState<ProjectContextRelation>("related");
  const [newEdgeStrength, setNewEdgeStrength] = useState("3");
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
    if (
      !newEdgeSourceID ||
      !newEdgeTargetID ||
      newEdgeSourceID === newEdgeTargetID
    ) {
      return;
    }
    const formEl = event.currentTarget;
    createEdgeMutation.mutate(
      {
        source_context_id: newEdgeSourceID,
        target_context_id: newEdgeTargetID,
        relation: newEdgeRelation,
        strength: Number(newEdgeStrength),
        note: newEdgeNote.trim(),
      },
      {
        onSuccess: () => {
          formEl.reset();
          setNewEdgeSourceID("");
          setNewEdgeTargetID("");
          setNewEdgeRelation("related");
          setNewEdgeStrength("3");
          setNewEdgeNote("");
          setNewEdgeEditorKey((value) => value + 1);
          setAddEdgeOpen(false);
        },
      },
    );
  }

  function openAddEdge(sourceId = "") {
    setNewEdgeSourceID(sourceId);
    setNewEdgeTargetID("");
    setNewEdgeRelation("related");
    setNewEdgeStrength("3");
    setNewEdgeNote("");
    setNewEdgeEditorKey((value) => value + 1);
    setAddEdgeOpen(true);
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
  const memoryOptions = useMemo<CustomSelectOption[]>(() => {
    return [
      { value: "", label: "Select memory" },
      ...items.map((item) => ({ value: item.id, label: item.title })),
    ];
  }, [items]);
  const relationOptions = useMemo<CustomSelectOption[]>(() => {
    return PROJECT_CONTEXT_RELATIONS.map((relation) => ({
      value: relation,
      label: relation.replace("_", " "),
    }));
  }, []);
  const strengthOptions = useMemo<CustomSelectOption[]>(() => {
    return [1, 2, 3, 4, 5].map((strength) => ({
      value: String(strength),
      label: String(strength),
    }));
  }, []);

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
      ) : null}
      {addEdgeOpen ? (
        <Modal
          onClose={() => setAddEdgeOpen(false)}
          labelledBy="project-context-add-edge-title"
          describedBy="project-context-add-edge-desc"
          size="wide"
          busy={createEdgeMutation.isPending}
          busyLabel="Adding connection..."
        >
          <form
            className="panel modal-sheet modal-sheet--edit project-context-form project-context-edge-modal"
            onSubmit={submitEdge}
          >
            <div className="project-context-form__heading">
              <div>
                <h2 id="project-context-add-edge-title">Add connection</h2>
                <p id="project-context-add-edge-desc" className="muted">
                  Link two memory nodes when the relationship helps future work.
                </p>
              </div>
            </div>
            <div className="project-context-edge-grid">
              <CustomSelect
                id="project-context-edge-source"
                label="From"
                value={newEdgeSourceID}
                options={memoryOptions}
                onChange={setNewEdgeSourceID}
              />
              <CustomSelect
                id="project-context-edge-target"
                label="To"
                value={newEdgeTargetID}
                options={memoryOptions}
                onChange={setNewEdgeTargetID}
              />
              <CustomSelect
                id="project-context-edge-relation"
                label="Relation"
                value={newEdgeRelation}
                options={relationOptions}
                onChange={(value) => setNewEdgeRelation(value as ProjectContextRelation)}
              />
              <CustomSelect
                id="project-context-edge-strength"
                label="Strength"
                value={newEdgeStrength}
                options={strengthOptions}
                onChange={setNewEdgeStrength}
              />
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
            <div className="row stack-row-actions">
              <button type="submit" disabled={createEdgeMutation.isPending}>
                {createEdgeMutation.isPending ? "Connecting..." : "Add connection"}
              </button>
              <button
                type="button"
                className="secondary"
                disabled={createEdgeMutation.isPending}
                onClick={() => setAddEdgeOpen(false)}
              >
                Cancel
              </button>
            </div>
          </form>
        </Modal>
      ) : null}
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
              {items.length >= 2 ? (
                <button
                  type="button"
                  className="project-context-add-connection-button"
                  onClick={() => openAddEdge()}
                >
                  Add connection
                </button>
              ) : null}
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
                  aria-selected={contextView === "tree"}
                  onClick={() => setContextView("tree")}
                >
                  Tree
                </button>
              </div>
            </div>
          </div>
          {contextView === "list" ? (
            <ProjectContextListView
              items={items}
              nodeSaving={patchContextMutation.isPending}
              nodeDeleting={deleteContextMutation.isPending}
              onSaveNode={(id, patch) => patchContextMutation.mutate({ id, ...patch })}
              onDeleteNode={(id) => deleteContextMutation.mutate(id)}
              onAddConnection={openAddEdge}
            />
          ) : (
            <ProjectContextTreeView items={items} edges={edges} />
          )}
        </>
      )}
    </section>
  );
}
