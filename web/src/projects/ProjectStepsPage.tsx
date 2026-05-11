import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import type { useTasksApp } from "@/tasks/hooks/useTasksApp";
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
import { ProjectStepCreateModal } from "./ProjectStepCreateModal";
import { ProjectStepsGraphView } from "./ProjectStepsGraphView";
import {
  truncateListDependencySummary,
  truncateListDescription,
  truncateListTitle,
} from "./projectListDisplayText";

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

type TasksApp = ReturnType<typeof useTasksApp>;

type ProjectStepsPageProps = {
  app: TasksApp;
};

export function ProjectStepsPage({ app }: ProjectStepsPageProps) {
  const { projectId = "" } = useParams();
  const [searchParams] = useSearchParams();
  const goalId = (searchParams.get("goal_id") ?? "").trim();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [view, setView] = useState<ViewMode>("list");
  const [createStepModalOpen, setCreateStepModalOpen] = useState(false);
  const [newTitle, setNewTitle] = useState("");
  const [newDescription, setNewDescription] = useState("");
  const [criterionDrafts, setCriterionDrafts] = useState<string[]>([""]);

  const project = useQuery({
    queryKey: projectQueryKeys.detail(projectId),
    queryFn: ({ signal }) => getProject(projectId, { signal }),
    enabled: Boolean(projectId),
  });

  const projectGoalsQuery = useProjectGoals(projectId, {
    enabled: Boolean(projectId),
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

  const stepsCrumbPrimaryName = useMemo(() => {
    if (!goalId) return project.data?.name ?? "";
    const goals = projectGoalsQuery.data?.goals;
    if (!goals) return "";
    const g = goals.find((x) => x.id === goalId);
    return g?.title?.trim() || "Goal";
  }, [goalId, project.data?.name, projectGoalsQuery.data?.goals]);

  const stepsDocumentTitle = useMemo(() => {
    if (!project.data?.name) return "Project steps";
    if (goalId && stepsCrumbPrimaryName)
      return `${stepsCrumbPrimaryName} · Steps · ${project.data.name}`;
    return `${project.data.name} · Steps`;
  }, [project.data?.name, goalId, stepsCrumbPrimaryName]);

  useDocumentTitle(stepsDocumentTitle);

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
      setCreateStepModalOpen(false);
      setNewTitle("");
      setNewDescription("");
      setCriterionDrafts([""]);
      await invalidate();
    },
  });

  const dismissCreateStepModal = useCallback(() => {
    createMut.reset();
    setNewTitle("");
    setNewDescription("");
    setCriterionDrafts([""]);
    setCreateStepModalOpen(false);
  }, [createMut]);

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
      <section className="panel ps">
        <EmptyState title="Missing project id" description="" density="compact" hideIcon />
      </section>
    );
  }

  const projectBase = `/projects/${encodeURIComponent(projectId)}`;
  const stepsBackHref = goalId ? `${projectBase}/goals` : projectBase;
  const stepsBackLabel = goalId ? "Back to goals" : "Back to project";

  if (!goalId) {
    return (
      <section className="panel ps">
        <header className="pg__header">
          <Link to={stepsBackHref} className="pd__back project-context-back-link">
            <span aria-hidden="true">&#8249;</span>
            {stepsBackLabel}
          </Link>
        </header>
        <h2 className="ps__title">Choose a goal</h2>
        <p className="muted stack-tight-zero">
          Pick a goal, or{" "}
          <Link to={`/projects/${encodeURIComponent(projectId)}/goals`}>open goals</Link>.
        </p>
        {projectGoalsQuery.isLoading ? <p className="muted">Loading goals…</p> : null}
        {projectGoalsQuery.error ? (
          <p className="pd__error-message" role="alert">
            {projectGoalsQuery.error.message}
          </p>
        ) : null}
        <ul className="ps__list">
          {(projectGoalsQuery.data?.goals ?? []).map((g) => (
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
        {(projectGoalsQuery.data?.goals ?? []).length === 0 && !projectGoalsQuery.isLoading ? (
          <EmptyState
            title="No goals yet"
            description=""
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
    <section className="panel ps">
      {project.data ? (
        <h1 className="visually-hidden">
          {goalId && stepsCrumbPrimaryName
            ? `${stepsCrumbPrimaryName} · Steps`
            : `${project.data.name} · Steps`}
        </h1>
      ) : null}

      <header className="pg__header">
        <Link to={stepsBackHref} className="pd__back project-context-back-link">
          <span aria-hidden="true">&#8249;</span>
          {stepsBackLabel}
        </Link>
        <div className="pg__header-actions">
          <button
            type="button"
            className="pg__header-new-goal"
            onClick={() => setCreateStepModalOpen(true)}
          >
            Add step
          </button>
          <div className="pg__toggle" role="group" aria-label="List or graph layout">
            <button
              type="button"
              className={view === "list" ? "pg__toggle-btn is-active" : "pg__toggle-btn"}
              aria-pressed={view === "list"}
              onClick={() => setView("list")}
            >
              List
            </button>
            <button
              type="button"
              className={view === "graph" ? "pg__toggle-btn is-active" : "pg__toggle-btn"}
              aria-pressed={view === "graph"}
              onClick={() => setView("graph")}
            >
              Graph
            </button>
          </div>
        </div>
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
          <p className="pg__crumb muted">
            <span className="pg__crumb-name">
              {stepsCrumbPrimaryName || project.data.name}
            </span>
            <span aria-hidden="true"> · </span>
            Steps
          </p>

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
            <span className="ps__legend-item ps__legend-item--sep">
              <svg
                className="ps__legend-edge"
                width={28}
                height={10}
                viewBox="0 0 28 10"
                aria-hidden="true"
              >
                <line
                  x1="0"
                  y1="5"
                  x2="18"
                  y2="5"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeDasharray="3 3"
                  strokeLinecap="round"
                />
                <path d="M18 1.25 L26.5 5 L18 8.75 Z" fill="currentColor" />
              </svg>
              Independent
            </span>
            <span className="ps__legend-item">
              <svg
                className="ps__legend-edge"
                width={28}
                height={10}
                viewBox="0 0 28 10"
                aria-hidden="true"
              >
                <line
                  x1="0"
                  y1="5"
                  x2="18"
                  y2="5"
                  stroke="currentColor"
                  strokeWidth={2}
                  strokeLinecap="round"
                />
                <path d="M18 1 L27 5 L18 9 Z" fill="currentColor" />
              </svg>
              Dependent
            </span>
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
                    onNewTask={() =>
                      app.openCreateModal({
                        projectID: step.project_id,
                        projectStepID: step.id,
                        lockProjectAssignment: true,
                      })
                    }
                    newTaskDisabled={app.createModalOpen || app.draftPickerOpen}
                  />
                </li>
              ))}
            </ul>
          ) : (
            <div className="ps__graph-wrap">
              <ProjectStepsGraphView
                steps={ordered}
                phaseOf={uiPhase}
                gateLabel={gateLabel}
                tasksByStepId={tasksByStepId}
              />
            </div>
          )}

          <ProjectStepCreateModal
            open={createStepModalOpen}
            onDismiss={dismissCreateStepModal}
            draftTitle={newTitle}
            onDraftTitleChange={setNewTitle}
            draftDescription={newDescription}
            onDraftDescriptionChange={setNewDescription}
            criterionDrafts={criterionDrafts}
            onCriterionDraftsChange={setCriterionDrafts}
            createPending={createMut.isPending}
            createError={createMut.error}
            onCreate={() => createMut.mutateAsync()}
          />

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
  phase: ReturnType<typeof uiPhase>;
  taskStats?: { total: number; done: number };
  afterTitle?: string;
  patchPending: boolean;
  deletePending: boolean;
  onPatch: (body: Parameters<typeof patchProjectStep>[2]) => void;
  onDelete: () => void;
  onNewTask: () => void;
  newTaskDisabled: boolean;
};

function ProjectStepListBody({
  step,
  phase,
  taskStats,
  afterTitle,
  patchPending,
  deletePending,
  onPatch,
  onDelete,
  onNewTask,
  newTaskDisabled,
}: ListBodyProps) {
  const stats = taskStats ?? { total: 0, done: 0 };
  const critLabel = criterionRatio(step.criteria);
  const titleFull = step.title.trim();
  const titleShown = truncateListTitle(step.title);
  const descFull = step.description.trim();
  const descShown = descFull ? truncateListDescription(step.description) : "";
  const afterFull = afterTitle?.trim() ?? "";
  const afterShown = afterFull ? truncateListDependencySummary(afterFull) : "";

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
              <span
                className="ps__row-title"
                title={titleFull !== titleShown ? titleFull : undefined}
              >
                {titleShown}
              </span>
              <span className={`pd__chip pd__chip--gate pd__chip--${step.gate_status}`}>
                {gateLabel(step.gate_status)}
              </span>
              {step.gate_hold ? <span className="pd__chip pd__chip--hold">On hold</span> : null}
            </div>
            {descShown ? (
              <p
                className="ps__row-desc"
                title={descFull !== descShown ? descFull : undefined}
              >
                {descShown}
              </p>
            ) : null}
            <div className="ps__row-meta">
              <span className="ps__task-pill">
                {stats.total === 0 ? "No tasks yet" : `${stats.done}/${stats.total} tasks`}
              </span>
              {critLabel ? <span className="ps__task-pill">{critLabel}</span> : null}
              {afterTitle ? (
                <span
                  className="ps__step-linkage ps__step-linkage--depends"
                  title={afterFull !== afterShown ? afterFull : undefined}
                >
                  <span className="ps__step-linkage__lead">After</span>
                  <span className="ps__step-linkage__target">{afterShown}</span>
                </span>
              ) : (
                <span className="ps__step-linkage">Independent</span>
              )}
            </div>
            {step.gate_status === "pending_release" && step.pending_release_deadline ? (
              <StepCountdown deadlineIso={step.pending_release_deadline} />
            ) : null}
          </div>
        </div>
        <div className="ps__row-actions">
          {stepAcceptsNewTasks(step.gate_status) ? (
            <button
              type="button"
              className="ps__row-cta"
              onClick={onNewTask}
              disabled={newTaskDisabled}
            >
              New task
            </button>
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
