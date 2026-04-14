export type TaskCreateModalEvaluation = {
  overallScore: number;
  overallSummary: string;
  sections: Array<{ key: string; score: number }>;
};

type Props = {
  evaluation: TaskCreateModalEvaluation | null;
};

export function TaskCreateModalEvaluationSummary({ evaluation }: Props) {
  return (
    <section
      className="task-create-evaluation-summary"
      aria-label="Draft evaluation summary"
      aria-live="polite"
    >
      <div className="task-create-evaluation-head">
        <h3 className="task-create-evaluation-title">
          Latest evaluation score
        </h3>
        {evaluation ? (
          <p className="task-create-evaluation-score-badge">
            {evaluation.overallScore}
            <span>/100</span>
          </p>
        ) : null}
      </div>
      {evaluation ? (
        <>
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
      ) : (
        <p className="muted task-create-evaluation-empty">
          No score yet. Click <strong>Evaluate</strong> and your result appears
          here before you create the task.
        </p>
      )}
    </section>
  );
}
