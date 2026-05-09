import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import {
  createProjectStep,
  deleteProjectStep,
  getProject,
  listProjectSteps,
  listTasks,
  patchProjectStep,
} from "@/api";
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { taskQueryKeys } from "@/tasks/task-query";
import type { ProjectStep, ProjectStepCriterion, ProjectStepGateStatus } from "@/types";
import { useProjectGoals } from "./hooks";
import { projectQueryKeys } from "./queryKeys";

type ViewMode = "list" | "graph";

function gateLabel(s: ProjectStepGateStatus): string {
  switch (s) {
    case "locked":
      return "Locked";
    case "active":
      return "Active";
    case "pending_release":
      return "Pending release";
    case "released":
      return "Released";
    default:
      return s;
  }
}

function stepAcceptsNewTasks(s: ProjectStepGateStatus): boolean {
  return s === "active" || s === "pending_release";
}

function uiPhase(step: ProjectStep): "done" | "active" | "pending" | "blocked" {
  if (step.gate_status === "released") return "done";
  if (step.gate_hold) return "blocked";
  if (step.gate_status === "active" || step.gate_status === "pending_release") return "active";
  return "pending";
}

function criterionRatio(criteria: ProjectStepCriterion[]): string {
  if (criteria.length === 0) return "";
  const done = criteria.filter((c) => c.done).length;
  return `${done}/${criteria.length} criteria`;
}

function StepCountdown(props: { deadlineIso: string }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(t);
  }, []);
  const end = Date.parse(props.deadlineIso);
  const left = Number.isFinite(end) ? Math.max(0, Math.floor((end - now) / 1000)) : 0;
  const m = Math.floor(left / 60);
  const sec = left % 60;
  const label =
    left <= 0 ? "Grace elapsed" : `${m}:${sec.toString().padStart(2, "0")} until auto-release`;
  return (
    <p className="ps__countdown muted stack-tight-zero" aria-live="polite">
      {label}
    </p>
  );
}

export function ProjectStepsPage() {
  const { projectId = "" } = useParams();
  const [searchParams] = useSearchParams();
  const goalId = (searchParams.get("goal_id") ?? "").trim();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [view, setView] = useState<ViewMode>("list");
  const [newTitle, setNewTitle] = useState("");
  const [newDescription, setNewDescription] = useState("");
  const [criterionDrafts, setCriterionDrafts] = useState<string[]>([""]);

  const project = useQuery({
    queryKey: projectQueryKeys.detail(projectId),
    queryFn: ({ signal }) => getProject(projectId, { signal }),
    enabled: Boolean(projectId),
  });

  const goalsPicker = useProjectGoals(projectId, {
    enabled: Boolean(projectId && !goalId),
  });

  const stepsQuery = useQuery({
    queryKey: projectQueryKeys.steps(projectId, goalId),
    queryFn: ({ signal }) =>
      listProjectSteps(projectId, { signal, goalId: goalId || undefined }),
    enabled: Boolean(projectId && goalId),
  });

  const tasksQuery = useQuery({
    queryKey: taskQueryKeys.listRoot(),
    queryFn: ({ signal }) => listTasks(200, 0, { signal }),
    enabled: Boolean(projectId),
  });

  const title = project.data?.name ? `${project.data.name} · Steps` : "Project steps";
  useDocumentTitle(title);

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: projectQueryKeys.detail(projectId) });
  };

  const createMut = useMutation({
    mutationFn: () =>
      createProjectStep(projectId, {
        goal_id: goalId,
        title: newTitle.trim(),
        description: newDescription.trim(),
        criteria: criterionDrafts
          .map((s) => s.trim())
          .filter(Boolean)
          .map((text, i) => ({ text, done: false, sort_order: i + 1 })),
      }),
    onSuccess: async () => {
      setNewTitle("");
      setNewDescription("");
      setCriterionDrafts([""]);
      await invalidate();
    },
  });

  const patchMut = useMutation({
    mutationFn: (input: {
      stepId: string;
      body: Parameters<typeof patchProjectStep>[2];
    }) => patchProjectStep(projectId, input.stepId, input.body),
    onSuccess: async () => {
      await invalidate();
    },
  });

  const deleteMut = useMutation({
    mutationFn: (stepId: string) => deleteProjectStep(projectId, stepId),
    onSuccess: async () => {
      await invalidate();
    },
  });

  const ordered = useMemo(
    () => [...(stepsQuery.data?.steps ?? [])].sort((a, b) => a.sort_order - b.sort_order),
    [stepsQuery.data?.steps],
  );

  const tasksByStepId = useMemo(() => {
    const m = new Map<string, { total: number; done: number }>();
    const members = (tasksQuery.data?.tasks ?? []).filter((t) => t.project_id === projectId);
    for (const t of members) {
      const sid = t.project_step_id?.trim();
      if (!sid) continue;
      const cur = m.get(sid) ?? { total: 0, done: 0 };
      cur.total += 1;
      if (t.status === "done") cur.done += 1;
      m.set(sid, cur);
    }
    return m;
  }, [tasksQuery.data?.tasks, projectId]);

  const prevTitleByStepId = useMemo(() => {
    const m = new Map<string, string>();
    for (let i = 0; i < ordered.length; i++) {
      if (i > 0) m.set(ordered[i].id, ordered[i - 1].title);
    }
    return m;
  }, [ordered]);

  if (!projectId) {
    return (
      <section className="panel task-detail-panel">
        <EmptyState
          title="Missing project id"
          description="Choose a project from the project list."
          density="compact"
          hideIcon
        />
      </section>
    );
  }

  if (!goalId) {
    return (
      <section className="panel task-detail-panel ps">
        <header className="ps__header">
          <Link to={`/projects/${encodeURIComponent(projectId)}`} className="pd__back project-context-back-link">
            <span aria-hidden="true">&#8249;</span>
            Back to project
          </Link>
        </header>
        <h2 className="ps__title">Choose a goal</h2>
        <p className="ps__subtitle muted">
          Steps are scoped to a goal. Pick one below or{" "}
          <Link to={`/projects/${encodeURIComponent(projectId)}/goals`}>manage goals</Link>.
        </p>
        {goalsPicker.isLoading ? <p className="muted">Loading goals…</p> : null}
        {goalsPicker.error ? (
          <p className="pd__error-message" role="alert">
            {goalsPicker.error.message}
          </p>
        ) : null}
        <ul className="ps__list">
          {(goalsPicker.data?.goals ?? []).map((g) => (
            <li key={g.id} className="ps__card">
              <Link
                className="ps__row-link"
                to={`/projects/${encodeURIComponent(projectId)}/steps?goal_id=${encodeURIComponent(g.id)}`}
              >
                <span className="ps__row-title">{g.title}</span>
                <span className="muted" aria-hidden="true">
                  →
                </span>
              </Link>
            </li>
          ))}
        </ul>
        {(goalsPicker.data?.goals ?? []).length === 0 && !goalsPicker.isLoading ? (
          <EmptyState
            title="No goals yet"
            description="Create a goal first, then return here to add steps."
            density="compact"
            action={{
              label: "Open goals",
              onClick: () => {
                void navigate(`/projects/${encodeURIComponent(projectId)}/goals`);
              },
            }}
          />
        ) : null}
      </section>
    );
  }

  return (
    <section className="panel task-detail-panel ps">
      {project.data ? <h1 className="visually-hidden">{project.data.name}</h1> : null}

      <header className="ps__header">
        <Link to={`/projects/${encodeURIComponent(projectId)}`} className="pd__back project-context-back-link">
          <span aria-hidden="true">&#8249;</span>
          Back to project
        </Link>
      </header>

      {project.isLoading ? <p className="muted">Loading project…</p> : null}
      {project.error ? (
        <div className="pd__error" role="alert">
          <div className="pd__error-dot" aria-hidden="true" />
          <div>
            <p className="pd__error-title">Unable to load this project</p>
            <p className="pd__error-message">{project.error.message}</p>
          </div>
        </div>
      ) : null}

      {project.data ? (
        <>
          <div className="ps__lede">
            <div>
              <p className="ps__crumb muted">
                <Link to={`/projects/${encodeURIComponent(projectId)}`}>{project.data.name}</Link>
                <span aria-hidden="true"> · </span>
                <span>Steps</span>
              </p>
              <h2 className="ps__title">Stages and completion</h2>
              <p className="ps__subtitle muted">
                Gates advance when every task in the step is done and every criterion is checked off.
              </p>
            </div>
            <div className="ps__view-toggle" role="group" aria-label="View mode">
              <button
                type="button"
                className="ps__toggle-btn"
                aria-pressed={view === "list"}
                onClick={() => setView("list")}
              >
                List
              </button>
              <button
                type="button"
                className="ps__toggle-btn"
                aria-pressed={view === "graph"}
                onClick={() => setView("graph")}
              >
                Graph
              </button>
            </div>
          </div>

          <div className="ps__legend" aria-label="Legend">
            <span className="ps__legend-item">
              <span className="ps__dot ps__dot--done" aria-hidden="true" /> Done
            </span>
            <span className="ps__legend-item">
              <span className="ps__dot ps__dot--active" aria-hidden="true" /> In progress
            </span>
            <span className="ps__legend-item">
              <span className="ps__dot ps__dot--pending" aria-hidden="true" /> Pending
            </span>
            <span className="ps__legend-item">
              <span className="ps__dot ps__dot--blocked" aria-hidden="true" /> Blocked
            </span>
            <span className="ps__legend-item ps__legend-item--sep">Solid arrow: depends on prior stage</span>
          </div>

          {stepsQuery.isLoading ? <p className="muted">Loading steps…</p> : null}
          {stepsQuery.error ? (
            <p className="pd__error-message" role="alert">
              {stepsQuery.error.message}
            </p>
          ) : null}

          {view === "list" ? (
            <ul className="ps__list">
              {ordered.map((step) => (
                <li key={step.id} className="ps__card">
                  <ProjectStepListBody
                    step={step}
                    projectId={projectId}
                    phase={uiPhase(step)}
                    taskStats={tasksByStepId.get(step.id)}
                    afterTitle={prevTitleByStepId.get(step.id)}
                    patchPending={patchMut.isPending}
                    deletePending={deleteMut.isPending}
                    onPatch={(body) => patchMut.mutate({ stepId: step.id, body })}
                    onDelete={() => {
                      if (window.confirm(`Delete step “${step.title}”?`)) {
                        deleteMut.mutate(step.id);
                      }
                    }}
                  />
                </li>
              ))}
            </ul>
          ) : (
            <div className="ps__graph" role="list">
              {ordered.map((step, i) => (
                <div key={step.id} className="ps__graph-cell" role="listitem">
                  {i > 0 ? <div className="ps__graph-arrow" aria-hidden="true" /> : null}
                  <article className={`ps__graph-card ps__graph-card--${uiPhase(step)}`}>
                    <header className="ps__graph-card-head">
                      <span className={`ps__dot ps__dot--${uiPhase(step)}`} aria-hidden="true" />
                      <span className="muted">{gateLabel(step.gate_status)}</span>
                    </header>
                    <h3 className="ps__graph-card-title">{step.title}</h3>
                    {step.description.trim() ? (
                      <p className="ps__graph-card-desc muted">{step.description}</p>
                    ) : null}
                  </article>
                </div>
              ))}
            </div>
          )}

          <section className="ps__create" aria-labelledby="ps-create-title">
            <h3 id="ps-create-title" className="ps__create-title">
              Add step
            </h3>
            <form
              className="ps__create-form"
              onSubmit={(e) => {
                e.preventDefault();
                if (!newTitle.trim()) return;
                void createMut.mutateAsync();
              }}
            >
              <label className="field grow">
                <span className="settings-field-label">Title</span>
                <input
                  value={newTitle}
                  onChange={(ev) => setNewTitle(ev.target.value)}
                  placeholder="e.g. JWT implementation"
                  disabled={createMut.isPending}
                  required
                />
              </label>
              <label className="field grow">
                <span className="settings-field-label">Description</span>
                <textarea
                  value={newDescription}
                  onChange={(ev) => setNewDescription(ev.target.value)}
                  placeholder="What this stage covers"
                  rows={2}
                  disabled={createMut.isPending}
                />
              </label>
              <fieldset className="ps__criteria-fieldset">
                <legend className="settings-field-label">Criteria (optional)</legend>
                <p className="muted ps__criteria-help">Each line becomes a checklist item before the gate can advance.</p>
                {criterionDrafts.map((line, idx) => (
                  <div key={idx} className="ps__criteria-row">
                    <input
                      value={line}
                      onChange={(ev) => {
                        const next = [...criterionDrafts];
                        next[idx] = ev.target.value;
                        setCriterionDrafts(next);
                      }}
                      placeholder="Criterion text"
                      disabled={createMut.isPending}
                    />
                    {criterionDrafts.length > 1 ? (
                      <button
                        type="button"
                        className="secondary ps__criteria-remove"
                        disabled={createMut.isPending}
                        onClick={() =>
                          setCriterionDrafts((rows) => rows.filter((_, j) => j !== idx))
                        }
                      >
                        Remove
                      </button>
                    ) : null}
                  </div>
                ))}
                <button
                  type="button"
                  className="secondary ps__criteria-add"
                  disabled={createMut.isPending}
                  onClick={() => setCriterionDrafts((rows) => [...rows, ""])}
                >
                  Add criterion line
                </button>
              </fieldset>
              <div className="ps__create-actions">
                <button type="submit" className="primary" disabled={createMut.isPending || !newTitle.trim()}>
                  Create step
                </button>
              </div>
            </form>
            {createMut.error ? (
              <p className="pd__error-message" role="alert">
                {createMut.error.message}
              </p>
            ) : null}
          </section>

          {patchMut.error ? (
            <p className="pd__error-message" role="alert">
              {patchMut.error.message}
            </p>
          ) : null}
          {deleteMut.error ? (
            <p className="pd__error-message" role="alert">
              {deleteMut.error.message}
            </p>
          ) : null}
        </>
      ) : null}
    </section>
  );
}

type ListBodyProps = {
  step: ProjectStep;
  projectId: string;
  phase: ReturnType<typeof uiPhase>;
  taskStats?: { total: number; done: number };
  afterTitle?: string;
  patchPending: boolean;
  deletePending: boolean;
  onPatch: (body: Parameters<typeof patchProjectStep>[2]) => void;
  onDelete: () => void;
};

function ProjectStepListBody({
  step,
  projectId,
  phase,
  taskStats,
  afterTitle,
  patchPending,
  deletePending,
  onPatch,
  onDelete,
}: ListBodyProps) {
  const stats = taskStats ?? { total: 0, done: 0 };
  const critLabel = criterionRatio(step.criteria);

  const toggleCriterion = (id: string, done: boolean) => {
    const next = step.criteria.map((c) => (c.id === id ? { ...c, done } : c));
    onPatch({
      criteria: next.map((c) => ({
        id: c.id,
        text: c.text,
        done: c.done,
        sort_order: c.sort_order,
      })),
    });
  };

  return (
    <>
      <div className="ps__row-main">
        <div className="ps__row-top">
          <span className={`ps__dot ps__dot--${phase}`} aria-hidden="true" />
          <div className="ps__row-text">
            <div className="ps__row-title-line">
              <span className="ps__row-title">{step.title}</span>
              <span className={`pd__chip pd__chip--gate pd__chip--${step.gate_status}`}>
                {gateLabel(step.gate_status)}
              </span>
              {step.gate_hold ? <span className="pd__chip pd__chip--hold">On hold</span> : null}
            </div>
            {step.description.trim() ? (
              <p className="ps__row-desc muted">{step.description}</p>
            ) : null}
            <div className="ps__row-meta">
              <span className="ps__task-pill muted">
                {stats.total === 0 ? "No tasks" : `${stats.done}/${stats.total} tasks`}
              </span>
              {critLabel ? <span className="ps__task-pill muted">{critLabel}</span> : null}
              {afterTitle ? (
                <span className="ps__dep muted">
                  After: <strong className="ps__dep-strong">{afterTitle}</strong>
                </span>
              ) : (
                <span className="ps__dep-badge">Independent</span>
              )}
            </div>
            {step.gate_status === "pending_release" && step.pending_release_deadline ? (
              <StepCountdown deadlineIso={step.pending_release_deadline} />
            ) : null}
          </div>
        </div>
        <div className="ps__row-actions">
          {stepAcceptsNewTasks(step.gate_status) ? (
            <Link
              className="secondary ps__action-btn"
              to={`/?create=1&project=${encodeURIComponent(projectId)}&step=${encodeURIComponent(step.id)}`}
            >
              New task
            </Link>
          ) : null}
          {step.gate_status === "pending_release" && !step.gate_hold ? (
            <button
              type="button"
              className="secondary ps__action-btn"
              disabled={patchPending}
              onClick={() => onPatch({ gate_action: "hold" })}
            >
              Hold
            </button>
          ) : null}
          {step.gate_status === "pending_release" && step.gate_hold ? (
            <button
              type="button"
              className="secondary ps__action-btn"
              disabled={patchPending}
              onClick={() => onPatch({ gate_action: "clear_hold" })}
            >
              Clear hold
            </button>
          ) : null}
          {step.gate_status === "pending_release" ? (
            <button
              type="button"
              className="primary ps__action-btn"
              disabled={patchPending}
              onClick={() => onPatch({ gate_action: "release" })}
            >
              Release now
            </button>
          ) : null}
          {step.gate_status === "released" || step.gate_status === "locked" ? (
            <button
              type="button"
              className="secondary ps__action-btn ps__action-btn--danger"
              disabled={deletePending}
              onClick={onDelete}
            >
              Delete
            </button>
          ) : null}
        </div>
      </div>
      {step.criteria.length > 0 ? (
        <ul className="ps__criteria-list">
          {step.criteria.map((c) => (
            <li key={c.id} className="ps__criteria-item">
              <label className="ps__criteria-label">
                <input
                  type="checkbox"
                  checked={c.done}
                  disabled={patchPending}
                  onChange={(ev) => toggleCriterion(c.id, ev.target.checked)}
                />
                <span>{c.text}</span>
              </label>
            </li>
          ))}
        </ul>
      ) : null}
    </>
  );
}
