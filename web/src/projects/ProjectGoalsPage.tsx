import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { createProjectGoal, getProject, listProjectGoals, patchProjectGoal } from "@/api";
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import type { ProjectGoal, ProjectGoalCriterion, ProjectStepGateStatus } from "@/types";
import { projectQueryKeys } from "./queryKeys";
import { ProjectGoalCreateModal } from "./ProjectGoalCreateModal";
import { ProjectGoalsGraphView } from "./ProjectGoalsGraphView";
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

function goalTitleById(goals: ProjectGoal[], id: string): string {
  const g = goals.find((x) => x.id === id);
  return g?.title?.trim() || id;
}

function criterionRatio(criteria: ProjectGoalCriterion[]): string {
  if (criteria.length === 0) return "";
  const done = criteria.filter((c) => c.done).length;
  return `${done}/${criteria.length} criteria`;
}

function uiPhase(goal: ProjectGoal): "done" | "active" | "pending" | "blocked" {
  if (goal.gate_status === "released") return "done";
  if (goal.gate_hold) return "blocked";
  if (goal.gate_status === "active" || goal.gate_status === "pending_release") return "active";
  return "pending";
}

function GoalCountdown(props: { deadlineIso: string }) {
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
    <p className="pg__countdown muted stack-tight-zero" aria-live="polite">
      {label}
    </p>
  );
}

export function ProjectGoalsPage() {
  const { projectId = "" } = useParams();
  const queryClient = useQueryClient();
  const [view, setView] = useState<ViewMode>("list");
  const [createGoalOpen, setCreateGoalOpen] = useState(false);
  const [newTitle, setNewTitle] = useState("");
  const [newDescription, setNewDescription] = useState("");
  const [depsDraft, setDepsDraft] = useState("");
  const [criterionDrafts, setCriterionDrafts] = useState<string[]>([""]);

  const project = useQuery({
    queryKey: projectQueryKeys.detail(projectId),
    queryFn: ({ signal }) => getProject(projectId, { signal }),
    enabled: Boolean(projectId),
  });

  const goalsQuery = useQuery({
    queryKey: projectQueryKeys.goals(projectId),
    queryFn: ({ signal }) => listProjectGoals(projectId, { signal }),
    enabled: Boolean(projectId),
  });

  const title = project.data?.name ? `${project.data.name} · Goals` : "Project goals";
  useDocumentTitle(title);

  const goals = goalsQuery.data?.goals ?? [];

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: projectQueryKeys.detail(projectId) });
  };

  const createMut = useMutation({
    mutationFn: () => {
      const raw = depsDraft
        .split(/[,]+/)
        .map((s) => s.trim())
        .filter(Boolean);
      const critLines = criterionDrafts.map((s) => s.trim()).filter(Boolean);
      const criteria =
        critLines.length > 0
          ? critLines.map((text, i) => ({ text, sort_order: i + 1 }))
          : undefined;
      return createProjectGoal(projectId, {
        title: newTitle.trim(),
        description: newDescription.trim(),
        depends_on_goal_ids: raw.length ? raw : undefined,
        criteria,
      });
    },
    onSuccess: async () => {
      setNewTitle("");
      setNewDescription("");
      setDepsDraft("");
      setCriterionDrafts([""]);
      setCreateGoalOpen(false);
      await invalidate();
    },
  });

  const patchMut = useMutation({
    mutationFn: (input: { goalId: string; body: Parameters<typeof patchProjectGoal>[2] }) =>
      patchProjectGoal(projectId, input.goalId, input.body),
    onSuccess: async () => {
      await invalidate();
    },
  });

  if (!projectId) {
    return (
      <section className="panel pg">
        <EmptyState title="Missing project" description="" density="compact" hideIcon />
      </section>
    );
  }

  return (
    <section className="panel pg">
      <header className="pg__header">
        <Link to={`/projects/${encodeURIComponent(projectId)}`} className="pg__back">
          <span aria-hidden="true">&#8249;</span>
          Back to project
        </Link>
        <div className="pg__header-actions">
          <button
            type="button"
            className="pg__header-new-goal"
            onClick={() => setCreateGoalOpen(true)}
          >
            New goal
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

      {project.data ? (
        <p className="pg__crumb muted">
          <span className="pg__crumb-name">{project.data.name}</span>
          <span aria-hidden="true"> · </span>
          Goals
        </p>
      ) : null}

      <div className="pg__legend ps__legend" aria-label="Legend">
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
          <svg className="ps__legend-edge" width={28} height={10} viewBox="0 0 28 10" aria-hidden="true">
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
          <svg className="ps__legend-edge" width={28} height={10} viewBox="0 0 28 10" aria-hidden="true">
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

      <ProjectGoalCreateModal
        open={createGoalOpen}
        onDismiss={() => setCreateGoalOpen(false)}
        draftTitle={newTitle}
        onDraftTitleChange={setNewTitle}
        draftDescription={newDescription}
        onDraftDescriptionChange={setNewDescription}
        depsDraft={depsDraft}
        onDepsDraftChange={setDepsDraft}
        criterionDrafts={criterionDrafts}
        onCriterionDraftsChange={setCriterionDrafts}
        createPending={createMut.isPending}
        createError={createMut.error}
        onCreate={() => createMut.mutateAsync()}
      />

      {goalsQuery.isLoading ? <p className="muted">Loading goals…</p> : null}
      {goalsQuery.error ? (
        <p className="pg__error" role="alert">
          {goalsQuery.error.message}
        </p>
      ) : null}

      {view === "list" ? (
        <ul className="pg__list">
          {goals.map((g) => {
            const phase = uiPhase(g);
            const critLabel = criterionRatio(g.criteria);
            const toggleCriterion = (id: string, done: boolean) => {
              const next = g.criteria.map((c) => (c.id === id ? { ...c, done } : c));
              void patchMut.mutateAsync({
                goalId: g.id,
                body: {
                  criteria: next.map((c) => ({
                    id: c.id,
                    text: c.text,
                    done: c.done,
                    sort_order: c.sort_order,
                  })),
                },
              });
            };
            const titleFull = g.title.trim();
            const titleShown = truncateListTitle(g.title);
            const descFull = g.description.trim();
            const descShown = descFull ? truncateListDescription(g.description) : "";
            const depFull =
              g.depends_on_goal_ids.length === 0
                ? ""
                : g.depends_on_goal_ids.map((id) => goalTitleById(goals, id)).join(", ");
            const depShown = depFull ? truncateListDependencySummary(depFull) : "";
            return (
              <li key={g.id} className="pg__row">
                <div className="pg__row-head">
                  <div className="pg__row-main">
                    <div className="pg__row-top">
                      <span className={`ps__dot ps__dot--${phase}`} aria-hidden="true" />
                      <div className="pg__row-text">
                        <div className="pg__row-title-line">
                          <p
                            className="pg__row-title"
                            title={titleFull !== titleShown ? titleFull : undefined}
                          >
                            {titleShown}
                          </p>
                        </div>
                        {descShown ? (
                          <p
                            className="pg__row-desc"
                            title={descFull !== descShown ? descFull : undefined}
                          >
                            {descShown}
                          </p>
                        ) : null}
                        <p className="pg__row-meta">
                          {critLabel ? <span className="ps__task-pill">{critLabel}</span> : null}
                          {g.depends_on_goal_ids.length === 0 ? (
                            <span className="ps__dep-badge">Independent</span>
                          ) : (
                            <span className="ps__dep" title={depFull !== depShown ? depFull : undefined}>
                              After: <strong className="ps__dep-strong">{depShown}</strong>
                            </span>
                          )}
                        </p>
                        {g.gate_status === "pending_release" && g.pending_release_deadline ? (
                          <GoalCountdown deadlineIso={g.pending_release_deadline} />
                        ) : null}
                      </div>
                    </div>
                  </div>
                  <div className="pg__row-side">
                    {g.gate_status === "pending_release" && !g.gate_hold ? (
                      <button
                        type="button"
                        className="pg__btn pg__btn--secondary"
                        disabled={patchMut.isPending}
                        onClick={() =>
                          void patchMut.mutateAsync({ goalId: g.id, body: { gate_action: "hold" } })
                        }
                      >
                        Hold
                      </button>
                    ) : null}
                    {g.gate_status === "pending_release" && g.gate_hold ? (
                      <button
                        type="button"
                        className="pg__btn pg__btn--secondary"
                        disabled={patchMut.isPending}
                        onClick={() =>
                          void patchMut.mutateAsync({
                            goalId: g.id,
                            body: { gate_action: "clear_hold" },
                          })
                        }
                      >
                        Clear hold
                      </button>
                    ) : null}
                    {g.gate_status === "pending_release" ? (
                      <button
                        type="button"
                        className="pg__btn pg__btn--primary"
                        disabled={patchMut.isPending}
                        onClick={() =>
                          void patchMut.mutateAsync({ goalId: g.id, body: { gate_action: "release" } })
                        }
                      >
                        Release now
                      </button>
                    ) : null}
                    <div className="pg__row-side-trail">
                      <span className={`pd__chip pd__chip--gate pd__chip--${g.gate_status}`}>
                        {gateLabel(g.gate_status)}
                      </span>
                      {g.gate_hold ? <span className="pd__chip pd__chip--hold">On hold</span> : null}
                      <Link
                        className="pg__btn pg__btn--secondary"
                        to={`/projects/${encodeURIComponent(projectId)}/steps?goal_id=${encodeURIComponent(g.id)}`}
                      >
                        Open steps
                      </Link>
                    </div>
                  </div>
                </div>
                {g.criteria.length > 0 ? (
                  <ul className="ps__criteria-list ps__criteria-list--pg-row">
                    {g.criteria.map((c) => (
                      <li key={c.id} className="ps__criteria-item">
                        <label className="ps__criteria-label">
                          <input
                            type="checkbox"
                            checked={c.done}
                            disabled={patchMut.isPending}
                            onChange={(ev) => toggleCriterion(c.id, ev.target.checked)}
                          />
                          <span>{c.text}</span>
                        </label>
                      </li>
                    ))}
                  </ul>
                ) : null}
              </li>
            );
          })}
        </ul>
      ) : (
        <div className="pg__graph" aria-label="Dependency overview">
          {goals.length === 0 ? (
            <p className="muted">No goals yet.</p>
          ) : (
            <ProjectGoalsGraphView goals={goals} />
          )}
        </div>
      )}
      {patchMut.error ? (
        <p className="pg__error" role="alert">
          {patchMut.error.message}
        </p>
      ) : null}
    </section>
  );
}
