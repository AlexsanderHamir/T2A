import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { FormEvent } from "react";
import {
  createProjectContext,
  createProjectContextEdge,
  deleteProjectContext,
  deleteProjectContextEdge,
  patchProjectContext,
  patchProjectContextEdge,
} from "@/api";
import { EmptyState } from "@/shared/EmptyState";
import {
  PROJECT_CONTEXT_KINDS,
  PROJECT_CONTEXT_RELATIONS,
  type ProjectContextKind,
  type ProjectContextRelation,
} from "@/types";
import { useProjectContext } from "./hooks";
import { ProjectContextEdgeEditor } from "./ProjectContextEdgeEditor";
import { ProjectContextItemEditor } from "./ProjectContextItemEditor";
import { projectQueryKeys } from "./queryKeys";

type Props = {
  projectId: string;
};

export function ProjectContextPanel({ projectId }: Props) {
  const queryClient = useQueryClient();
  const context = useProjectContext(projectId, { enabled: Boolean(projectId) });
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
    const body = String(form.get("body") ?? "").trim();
    if (!title || !body) return;
    const formEl = event.currentTarget;
    createContextMutation.mutate(
      {
        kind: String(form.get("kind") ?? "note") as ProjectContextKind,
        title,
        body,
        pinned: form.get("pinned") === "on",
      },
      { onSuccess: () => formEl.reset() },
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
        note: String(form.get("note") ?? "").trim(),
      },
      { onSuccess: () => formEl.reset() },
    );
  }

  const mutationError =
    createContextMutation.error ??
    patchContextMutation.error ??
    deleteContextMutation.error ??
    createEdgeMutation.error ??
    patchEdgeMutation.error ??
    deleteEdgeMutation.error;
  const items = context.data?.items ?? [];
  const edges = context.data?.edges ?? [];

  return (
    <section className="task-attempt-section">
      <h3>Project context</h3>
      <form className="project-context-form" onSubmit={submitContext}>
        <div className="project-context-form__heading">
          <div>
            <strong>Add memory node</strong>
            <p className="muted">
              Nodes are project-owned facts, decisions, constraints, or handoff
              notes. Add them anytime as the project evolves.
            </p>
          </div>
        </div>
        <div className="row">
          <div className="field grow">
            <label htmlFor="project-context-kind">Kind</label>
            <select id="project-context-kind" name="kind" defaultValue="note">
              {PROJECT_CONTEXT_KINDS.map((kind) => (
                <option key={kind} value={kind}>
                  {kind}
                </option>
              ))}
            </select>
          </div>
          <label className="checkbox-label project-context-pin">
            <input type="checkbox" name="pinned" />
            <span>Pinned</span>
          </label>
        </div>
        <div className="field grow">
          <label htmlFor="project-context-title">Title</label>
          <input id="project-context-title" name="title" required />
        </div>
        <div className="field grow">
          <label htmlFor="project-context-body">Body</label>
          <textarea id="project-context-body" name="body" rows={4} required />
        </div>
        <button type="submit" disabled={createContextMutation.isPending}>
          {createContextMutation.isPending ? "Adding..." : "Add node"}
        </button>
      </form>
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
              disabled={items.length < 2}
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
              disabled={items.length < 2}
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
              disabled={items.length < 2}
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
              disabled={items.length < 2}
            >
              {[1, 2, 3, 4, 5].map((strength) => (
                <option key={strength} value={strength}>
                  {strength}
                </option>
              ))}
            </select>
          </div>
          <div className="field grow project-context-edge-note">
            <label htmlFor="project-context-edge-note">Note</label>
            <input
              id="project-context-edge-note"
              name="note"
              disabled={items.length < 2}
              placeholder="Why does this connection matter?"
            />
          </div>
        </div>
        <button
          type="submit"
          disabled={createEdgeMutation.isPending || items.length < 2}
        >
          {createEdgeMutation.isPending ? "Connecting..." : "Add connection"}
        </button>
      </form>
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
        <div className="project-context-graph">
          <div>
            <h4>Nodes</h4>
            <ol className="task-attempt-phase-list">
              {items.map((item) => (
                <li key={item.id} className="task-attempt-phase">
                  <div className="project-context-item">
                    <strong>{item.title}</strong>
                    <p className="muted">
                      {item.kind}
                      {item.pinned ? " - pinned" : ""}
                    </p>
                    <p>{item.body}</p>
                    <ProjectContextItemEditor
                      item={item}
                      saving={patchContextMutation.isPending}
                      deleting={deleteContextMutation.isPending}
                      onSave={(id, patch) =>
                        patchContextMutation.mutate({ id, ...patch })
                      }
                      onDelete={(id) => deleteContextMutation.mutate(id)}
                    />
                  </div>
                </li>
              ))}
            </ol>
          </div>
          <div>
            <h4>Connections</h4>
            {edges.length === 0 ? (
              <p className="muted">
                No connections yet. Add one once two nodes influence each other.
              </p>
            ) : (
              <div className="project-context-edge-list">
                {edges.map((edge) => (
                  <ProjectContextEdgeEditor
                    key={edge.id}
                    edge={edge}
                    items={items}
                    saving={patchEdgeMutation.isPending}
                    deleting={deleteEdgeMutation.isPending}
                    onSave={(id, patch) =>
                      patchEdgeMutation.mutate({ id, ...patch })
                    }
                    onDelete={(id) => deleteEdgeMutation.mutate(id)}
                  />
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </section>
  );
}
