import type { VerificationCriterion } from "../../task-events/parseVerificationSnapshot";
import {
  verdictPillClass,
  verifierKindLabel,
} from "../../task-events/parseVerificationSnapshot";

export function VerificationCriteriaList({
  criteria,
  heading,
  attemptSeq,
}: {
  criteria: VerificationCriterion[];
  heading?: string;
  attemptSeq?: number;
}) {
  if (criteria.length === 0) return null;
  return (
    <div
      className="verification-criteria-block"
      data-testid="verification-criteria-list"
    >
      {heading ? (
        <h4 className="verification-criteria-heading">{heading}</h4>
      ) : null}
      {attemptSeq !== undefined ? (
        <p className="verification-criteria-attempt muted">
          Verify attempt #{attemptSeq}
        </p>
      ) : null}
      <ul className="verification-criteria-list">
        {criteria.map((row) => (
          <li
            key={row.criterionId}
            className="verification-criterion-item"
            data-verified={String(row.verified)}
          >
            <header className="verification-criterion-header">
              <span
                className={`cell-pill ${verdictPillClass(row.verified)}`}
              >
                {row.verified ? "Verified" : "Not verified"}
              </span>
              {row.verifierKind ? (
                <span className="verification-criterion-kind muted">
                  {verifierKindLabel(row.verifierKind)}
                </span>
              ) : null}
            </header>
            <p className="verification-criterion-text">
              {row.text ?? row.criterionId}
            </p>
            {row.reasoning ? (
              <p className="verification-criterion-reasoning">{row.reasoning}</p>
            ) : row.evidence ? (
              <p className="verification-criterion-evidence muted">
                Agent-claimed evidence: {row.evidence}
              </p>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  );
}
