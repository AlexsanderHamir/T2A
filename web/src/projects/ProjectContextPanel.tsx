import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { FormEvent } from "react";
import {
  createProjectContext,
  deleteProjectContext,
  patchProjectContext,
} from "@/api";
import { EmptyState } from "@/shared/EmptyState";
import { PROJECT_CONTEXT_KINDS, type ProjectContextKind } from "@/types";
import { useProjectContext } from "./hooks";
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

  const mutationError =
    createContextMutation.error ??
    patchContextMutation.error ??
    deleteContextMutation.error;

  return (
    <section className="task-attempt-section">
      <h3>Project context</h3>
      <form className="project-context-form" onSubmit={submitContext}>
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
          {createContextMutation.isPending ? "Adding..." : "Add context"}
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
      ) : (context.data?.items ?? []).length === 0 ? (
        <EmptyState
          title="No context items yet"
          description="Pinned decisions, constraints, and handoff notes will appear here."
          density="compact"
          hideIcon
        />
      ) : (
        <ol className="task-attempt-phase-list">
          {context.data?.items.map((item) => (
            <li key={item.id} className="task-attempt-phase">
              <div className="project-context-item">
                <strong>{item.title}</strong>
                <p className="muted">
                  {item.kind}
                  {item.pinned ? " · pinned" : ""}
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
      )}
    </section>
  );
}
