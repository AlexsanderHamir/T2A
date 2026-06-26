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
import { CustomSelect, type CustomSelectOption } from "@/components/custom-select";
import { RichPromptEditor } from "@/components/rich-prompt";
import { promptHasVisibleContent } from "@/lib/promptFormat";
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

function invalidateProjectContext(
  queryClient: ReturnType<typeof useQueryClient>,
  projectId: string,
) {
  return queryClient.invalidateQueries({
    queryKey: projectQueryKeys.context(projectId),
  });
}

function useProjectContextMutations(projectId: string) {
  const queryClient = useQueryClient();
  const invalidate = () => invalidateProjectContext(queryClient, projectId);

  const createContextMutation = useMutation({
    mutationFn: (input: {
      kind: ProjectContextKind;
      title: string;
      body: string;
      pinned: boolean;
    }) => createProjectContext(projectId, input),
    onSuccess: invalidate,
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
    onSuccess: invalidate,
  });
  const deleteContextMutation = useMutation({
    mutationFn: (contextId: string) => deleteProjectContext(projectId, contextId),
    onSuccess: invalidate,
  });
  const createEdgeMutation = useMutation({
    mutationFn: (input: {
      source_context_id: string;
      target_context_id: string;
      relation: ProjectContextRelation;
      strength: number;
      note: string;
    }) => createProjectContextEdge(projectId, input),
    onSuccess: invalidate,
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
    onSuccess: invalidate,
  });
  const deleteEdgeMutation = useMutation({
    mutationFn: (edgeId: string) => deleteProjectContextEdge(projectId, edgeId),
    onSuccess: invalidate,
  });

  return {
    createContextMutation,
    patchContextMutation,
    deleteContextMutation,
    createEdgeMutation,
    patchEdgeMutation,
    deleteEdgeMutation,
  };
}

type ProjectContextMutations = ReturnType<typeof useProjectContextMutations>;

function useProjectContextFormState(
  mutations: ProjectContextMutations,
) {
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

  function submitContext(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const title = String(form.get("title") ?? "").trim();
    const body = newNodeBody.trim();
    if (!title || !promptHasVisibleContent(body)) return;
    const formEl = event.currentTarget;
    mutations.createContextMutation.mutate(
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
    mutations.createEdgeMutation.mutate(
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

  return {
    contextView,
    setContextView,
    addNodeOpen,
    setAddNodeOpen,
    addEdgeOpen,
    setAddEdgeOpen,
    newNodeBody,
    setNewNodeBody,
    newNodeEditorKey,
    setNewNodeEditorKey,
    newEdgeSourceID,
    setNewEdgeSourceID,
    newEdgeTargetID,
    setNewEdgeTargetID,
    newEdgeRelation,
    setNewEdgeRelation,
    newEdgeStrength,
    setNewEdgeStrength,
    newEdgeNote,
    setNewEdgeNote,
    newEdgeEditorKey,
    setNewEdgeEditorKey,
    submitContext,
    submitEdge,
    openAddEdge,
  };
}

function firstProjectContextMutationError(
  mutations: ProjectContextMutations,
): Error | null {
  return (
    (mutations.createContextMutation.error as Error | null) ??
    (mutations.patchContextMutation.error as Error | null) ??
    (mutations.deleteContextMutation.error as Error | null) ??
    (mutations.createEdgeMutation.error as Error | null) ??
    (mutations.patchEdgeMutation.error as Error | null) ??
    (mutations.deleteEdgeMutation.error as Error | null)
  );
}

function buildMemorySelectOptions(items: ProjectContextItem[]): CustomSelectOption[] {
  return [
    { value: "", label: "Select memory" },
    ...items.map((item) => ({ value: item.id, label: item.title })),
  ];
}

function buildRelationSelectOptions(): CustomSelectOption[] {
  return PROJECT_CONTEXT_RELATIONS.map((relation) => ({
    value: relation,
    label: relation.replace("_", " "),
  }));
}

function buildStrengthSelectOptions(): CustomSelectOption[] {
  return [1, 2, 3, 4, 5].map((strength) => ({
    value: String(strength),
    label: String(strength),
  }));
}

type ProjectContextAddNodeModalProps = {
  open: boolean;
  onClose: () => void;
  isPending: boolean;
  newNodeBody: string;
  newNodeEditorKey: number;
  onBodyChange: (body: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

function ProjectContextAddNodeModal({
  open,
  onClose,
  isPending,
  newNodeBody,
  newNodeEditorKey,
  onBodyChange,
  onSubmit,
}: ProjectContextAddNodeModalProps) {
  if (!open) return null;

  return (
    <Modal
      onClose={onClose}
      labelledBy="project-context-add-node-title"
      describedBy="project-context-add-node-desc"
      size="wide"
      busy={isPending}
      busyLabel="Adding node..."
    >
      <form
        className="panel modal-sheet modal-sheet--edit project-context-form project-context-node-modal"
        onSubmit={onSubmit}
      >
        <div className="project-context-form__heading">
          <div>
            <h2 id="project-context-add-node-title">Add memory node</h2>
            <p id="project-context-add-node-desc" className="muted">
              Nodes are project-owned facts, decisions, constraints, or custom
              context. All fields are required.
            </p>
          </div>
        </div>
        <ProjectContextKindPicker
          idPrefix="project-context-kind"
          disabled={isPending}
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
              onChange={onBodyChange}
              disabled={isPending}
              placeholder="Write markdown-style context. Type @ to reference a repo file."
            />
          </div>
        </div>
        <div className="row stack-row-actions">
          <button type="submit" disabled={isPending}>
            {isPending ? "Adding..." : "Add node"}
          </button>
          <button
            type="button"
            className="secondary"
            disabled={isPending}
            onClick={onClose}
          >
            Cancel
          </button>
        </div>
      </form>
    </Modal>
  );
}

type ProjectContextAddEdgeModalProps = {
  open: boolean;
  onClose: () => void;
  isPending: boolean;
  memoryOptions: CustomSelectOption[];
  relationOptions: CustomSelectOption[];
  strengthOptions: CustomSelectOption[];
  newEdgeSourceID: string;
  newEdgeTargetID: string;
  newEdgeRelation: ProjectContextRelation;
  newEdgeStrength: string;
  newEdgeNote: string;
  newEdgeEditorKey: number;
  onSourceChange: (id: string) => void;
  onTargetChange: (id: string) => void;
  onRelationChange: (relation: ProjectContextRelation) => void;
  onStrengthChange: (strength: string) => void;
  onNoteChange: (note: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

function ProjectContextAddEdgeModal({
  open,
  onClose,
  isPending,
  memoryOptions,
  relationOptions,
  strengthOptions,
  newEdgeSourceID,
  newEdgeTargetID,
  newEdgeRelation,
  newEdgeStrength,
  newEdgeNote,
  newEdgeEditorKey,
  onSourceChange,
  onTargetChange,
  onRelationChange,
  onStrengthChange,
  onNoteChange,
  onSubmit,
}: ProjectContextAddEdgeModalProps) {
  if (!open) return null;

  return (
    <Modal
      onClose={onClose}
      labelledBy="project-context-add-edge-title"
      describedBy="project-context-add-edge-desc"
      size="wide"
      busy={isPending}
      busyLabel="Adding connection..."
    >
      <form
        className="panel modal-sheet modal-sheet--edit project-context-form project-context-edge-modal"
        onSubmit={onSubmit}
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
            onChange={onSourceChange}
          />
          <CustomSelect
            id="project-context-edge-target"
            label="To"
            value={newEdgeTargetID}
            options={memoryOptions}
            onChange={onTargetChange}
          />
          <CustomSelect
            id="project-context-edge-relation"
            label="Relation"
            value={newEdgeRelation}
            options={relationOptions}
            onChange={(value) => onRelationChange(value as ProjectContextRelation)}
          />
          <CustomSelect
            id="project-context-edge-strength"
            label="Strength"
            value={newEdgeStrength}
            options={strengthOptions}
            onChange={onStrengthChange}
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
                onChange={onNoteChange}
                disabled={isPending}
                placeholder="Why does this link matter? Type @ for files."
              />
            </div>
          </div>
        </div>
        <div className="row stack-row-actions">
          <button type="submit" disabled={isPending}>
            {isPending ? "Connecting..." : "Add connection"}
          </button>
          <button
            type="button"
            className="secondary"
            disabled={isPending}
            onClick={onClose}
          >
            Cancel
          </button>
        </div>
      </form>
    </Modal>
  );
}

type ProjectContextPanelWorkspaceProps = {
  contextView: ContextView;
  onContextViewChange: (view: ContextView) => void;
  items: ProjectContextItem[];
  edges: ProjectContextEdge[];
  isLoading: boolean;
  error: Error | null;
  mutations: ProjectContextMutations;
  onAddNode: () => void;
  onAddEdge: (sourceId?: string) => void;
};

function ProjectContextPanelWorkspace({
  contextView,
  onContextViewChange,
  items,
  edges,
  isLoading,
  error,
  mutations,
  onAddNode,
  onAddEdge,
}: ProjectContextPanelWorkspaceProps) {
  if (isLoading) {
    return (
      <div className="pc__skeleton" aria-hidden="true">
        <div className="pd__shimmer pd__shimmer--card" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="pd__inline-error" role="alert">
        {error.message}
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <EmptyState
        title="No context nodes yet"
        description="Add memory nodes and connect them as the work evolves."
        action={{
          label: "Add memory",
          onClick: onAddNode,
        }}
        density="compact"
        hideIcon
      />
    );
  }

  return (
    <>
      <div className="pc__action-bar">
        <div className="pc__actions-left">
          <button type="button" className="pc__btn-primary" onClick={onAddNode}>
            Add memory
          </button>
          {items.length >= 2 ? (
            <button
              type="button"
              className="pc__btn-secondary"
              onClick={() => onAddEdge()}
            >
              Add connection
            </button>
          ) : null}
        </div>
        <div className="pc__view-toggle" role="tablist" aria-label="Context view">
          <button
            type="button"
            role="tab"
            aria-selected={contextView === "list"}
            onClick={() => onContextViewChange("list")}
          >
            List
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={contextView === "tree"}
            onClick={() => onContextViewChange("tree")}
          >
            Tree
          </button>
        </div>
      </div>
      {contextView === "list" ? (
        <ProjectContextListView
          items={items}
          nodeSaving={mutations.patchContextMutation.isPending}
          nodeDeleting={mutations.deleteContextMutation.isPending}
          onSaveNode={(id, patch) =>
            mutations.patchContextMutation.mutate({ id, ...patch })
          }
          onDeleteNode={(id) => mutations.deleteContextMutation.mutate(id)}
          onAddConnection={onAddEdge}
        />
      ) : (
        <ProjectContextTreeView items={items} edges={edges} />
      )}
    </>
  );
}

export function ProjectContextPanel({ projectId }: Props) {
  const context = useProjectContext(projectId, { enabled: Boolean(projectId) });
  const mutations = useProjectContextMutations(projectId);
  const form = useProjectContextFormState(mutations);

  const mutationError = firstProjectContextMutationError(mutations);
  const items = context.data?.items ?? EMPTY_CONTEXT_ITEMS;
  const edges = context.data?.edges ?? EMPTY_CONTEXT_EDGES;
  const memoryOptions = useMemo(() => buildMemorySelectOptions(items), [items]);
  const relationOptions = useMemo(() => buildRelationSelectOptions(), []);
  const strengthOptions = useMemo(() => buildStrengthSelectOptions(), []);

  return (
    <section className="pc__workspace">
      <ProjectContextAddNodeModal
        open={form.addNodeOpen}
        onClose={() => form.setAddNodeOpen(false)}
        isPending={mutations.createContextMutation.isPending}
        newNodeBody={form.newNodeBody}
        newNodeEditorKey={form.newNodeEditorKey}
        onBodyChange={form.setNewNodeBody}
        onSubmit={form.submitContext}
      />
      {items.length < 2 ? (
        <p className="pc__hint">
          Add at least two memory nodes to start connecting them.
        </p>
      ) : null}
      <ProjectContextAddEdgeModal
        open={form.addEdgeOpen}
        onClose={() => form.setAddEdgeOpen(false)}
        isPending={mutations.createEdgeMutation.isPending}
        memoryOptions={memoryOptions}
        relationOptions={relationOptions}
        strengthOptions={strengthOptions}
        newEdgeSourceID={form.newEdgeSourceID}
        newEdgeTargetID={form.newEdgeTargetID}
        newEdgeRelation={form.newEdgeRelation}
        newEdgeStrength={form.newEdgeStrength}
        newEdgeNote={form.newEdgeNote}
        newEdgeEditorKey={form.newEdgeEditorKey}
        onSourceChange={form.setNewEdgeSourceID}
        onTargetChange={form.setNewEdgeTargetID}
        onRelationChange={form.setNewEdgeRelation}
        onStrengthChange={form.setNewEdgeStrength}
        onNoteChange={form.setNewEdgeNote}
        onSubmit={form.submitEdge}
      />
      {mutationError ? (
        <div className="pd__inline-error" role="alert">
          {mutationError.message}
        </div>
      ) : null}
      <ProjectContextPanelWorkspace
        contextView={form.contextView}
        onContextViewChange={form.setContextView}
        items={items}
        edges={edges}
        isLoading={context.isLoading}
        error={context.error}
        mutations={mutations}
        onAddNode={() => form.setAddNodeOpen(true)}
        onAddEdge={form.openAddEdge}
      />
    </section>
  );
}
