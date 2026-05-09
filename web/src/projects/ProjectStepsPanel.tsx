import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  createProjectStep,
  deleteProjectStep,
  listProjectSteps,
  patchProjectStep,
} from "@/api";
import type { ProjectStepGateStatus } from "@/types";
import { projectQueryKeys } from "./queryKeys";

type Props = {
  projectId: string;
};

function stepAcceptsNewTasks(s: ProjectStepGateStatus): boolean {
  return s === "active" || s === "pending_release";
}

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
    <p className="pd__step-countdown muted stack-tight-zero" aria-live="polite">
      {label}
    </p>
  );
}

export function ProjectStepsPanel({ projectId }: Props) {
  const queryClient = useQueryClient();
  const [newTitle, setNewTitle] = useState("");

  const stepsQuery = useQuery({
    queryKey: projectQueryKeys.steps(projectId),
    queryFn: ({ signal }) => listProjectSteps(projectId, { signal }),
    enabled: Boolean(projectId),
  });

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: projectQueryKeys.steps(projectId) });
    await queryClient.invalidateQueries({ queryKey: projectQueryKeys.detail(projectId) });
  };

  const createMut = useMutation({
    mutationFn: () => createProjectStep(projectId, { title: newTitle.trim() }),
    onSuccess: async () => {
      setNewTitle("");
      await invalidate();
    },
  });

  const patchMut = useMutation({
    mutationFn: (input: {
      stepId: string;
      body: { gate_action?: "release" | "hold" | "clear_hold"; title?: string };
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

  return (
    <section className="pd__card pd__card--steps" aria-labelledby="pd-steps-title">
      <div className="pd__card-head">
        <div className="pd__icon pd__icon--violet" aria-hidden="true">
          <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
            <path
              d="M4 5.5h10M4 9h10M4 12.5h6"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
            />
            <rect x="2.25" y="2.25" width="13.5" height="13.5" rx="3" stroke="currentColor" strokeWidth="1.2" opacity="0.45" />
          </svg>
        </div>
        <div>
          <h2 id="pd-steps-title" className="pd__card-title">
            Steps
          </h2>
          <p className="pd__card-desc">Ordered stages; gates advance when all step tasks are done.</p>
        </div>
      </div>

      {stepsQuery.isLoading ? <p className="muted">Loading steps…</p> : null}
      {stepsQuery.error ? (
        <p className="pd__error-message" role="alert">
          {stepsQuery.error.message}
        </p>
      ) : null}

      {ordered.length > 0 ? (
        <ul className="pd__step-list">
          {ordered.map((step) => (
            <li key={step.id} className="pd__step-row">
              <div className="pd__step-main">
                <div className="pd__step-title-row">
                  <span className="pd__step-order">{step.sort_order}</span>
                  <span className="pd__step-title">{step.title}</span>
                </div>
                <div className="pd__step-meta">
                  <span className={`pd__chip pd__chip--gate pd__chip--${step.gate_status}`}>
                    {gateLabel(step.gate_status)}
                  </span>
                  {step.gate_hold ? (
                    <span className="pd__chip pd__chip--hold">On hold</span>
                  ) : null}
                </div>
                {step.gate_status === "pending_release" && step.pending_release_deadline ? (
                  <StepCountdown deadlineIso={step.pending_release_deadline} />
                ) : null}
              </div>
              <div className="pd__step-actions">
                {stepAcceptsNewTasks(step.gate_status) ? (
                  <Link
                    className="secondary pd__step-btn"
                    to={`/?create=1&project=${encodeURIComponent(projectId)}&step=${encodeURIComponent(step.id)}`}
                    aria-label={`Create task in step ${step.title}`}
                  >
                    New task
                  </Link>
                ) : null}
                {step.gate_status === "pending_release" && !step.gate_hold ? (
                  <button
                    type="button"
                    className="secondary pd__step-btn"
                    disabled={patchMut.isPending}
                    onClick={() => patchMut.mutate({ stepId: step.id, body: { gate_action: "hold" } })}
                  >
                    Hold
                  </button>
                ) : null}
                {step.gate_status === "pending_release" && step.gate_hold ? (
                  <button
                    type="button"
                    className="secondary pd__step-btn"
                    disabled={patchMut.isPending}
                    onClick={() => patchMut.mutate({ stepId: step.id, body: { gate_action: "clear_hold" } })}
                  >
                    Clear hold
                  </button>
                ) : null}
                {step.gate_status === "pending_release" ? (
                  <button
                    type="button"
                    className="primary pd__step-btn"
                    disabled={patchMut.isPending}
                    onClick={() => patchMut.mutate({ stepId: step.id, body: { gate_action: "release" } })}
                  >
                    Release now
                  </button>
                ) : null}
                {step.gate_status === "released" || step.gate_status === "locked" ? (
                  <button
                    type="button"
                    className="secondary pd__step-btn pd__step-btn--danger"
                    disabled={deleteMut.isPending}
                    onClick={() => {
                      if (window.confirm(`Delete step “${step.title}”?`)) {
                        deleteMut.mutate(step.id);
                      }
                    }}
                  >
                    Delete
                  </button>
                ) : null}
              </div>
            </li>
          ))}
        </ul>
      ) : !stepsQuery.isLoading ? (
        <p className="muted">No steps yet.</p>
      ) : null}

      <form
        className="pd__step-add"
        onSubmit={(e) => {
          e.preventDefault();
          if (!newTitle.trim()) return;
          void createMut.mutateAsync();
        }}
      >
        <label className="field grow">
          <span className="settings-field-label">Add step</span>
          <input
            value={newTitle}
            onChange={(ev) => setNewTitle(ev.target.value)}
            placeholder="Step title"
            disabled={createMut.isPending}
          />
        </label>
        <button type="submit" className="primary" disabled={createMut.isPending || !newTitle.trim()}>
          Add step
        </button>
      </form>
      {createMut.error ? (
        <p className="pd__error-message" role="alert">
          {createMut.error.message}
        </p>
      ) : null}
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
    </section>
  );
}
