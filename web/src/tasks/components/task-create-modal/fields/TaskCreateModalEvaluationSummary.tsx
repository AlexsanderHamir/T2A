export type TaskCreateModalEvaluation = {
  overallScore: number;
  overallSummary: string;
  sections: Array<{ key: string; score: number }>;
};

type Props = {
  evaluation: TaskCreateModalEvaluation | null;
};

export function TaskCreateModalEvaluationSummary({ evaluation }: Props) {
  // Keep the live region mounted (so the first evaluation result
  // announces) but render no visible chrome until a score lands —
  // an always-on "no score yet" panel is pure scaffolding noise.
  const stateClass = evaluation
    ? "task-create-evaluation-summary task-create-evaluation-summary--filled"
    : "task-create-evaluation-summary task-create-evaluation-summary--empty";

  return (
    <section
      className={stateClass}
      aria-label="Draft evaluation summary"
      aria-live="polite"
    >
      {evaluation ? (
        <>
          <div className="task-create-evaluation-head">
            <h3 className="task-create-evaluation-title">
              Latest evaluation score
            </h3>
            <p className="task-create-evaluation-score-badge">
              {evaluation.overallScore}
              <span>/100</span>
            </p>
          </div>
          <p className="task-create-evaluation-overall">
            <strong>Overall:</strong> {evaluation.overallSummary}
          </p>
          <ul className="task-create-evaluation-sections">
            {evaluation.sections.map((s) => (
              <li key={s.key}>
                <span>{s.key.replaceAll("_", " ")}</span>
                <strong>{s.score}/100</strong>
              </li>
            ))}
          </ul>
        </>
      ) : null}
    </section>
  );
}
