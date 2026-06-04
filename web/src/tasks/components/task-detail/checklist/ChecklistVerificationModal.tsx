import { Modal } from "@/shared/Modal";
import { CHECKLIST_EVIDENCE_DISPLAY_CAP } from "@/types/task";

type Props = {
  /** The criterion text — shown as the modal heading. */
  criterionText: string;
  /** Verifier identity (e.g. `verify_agent`). Surfaced as a sub-label. */
  verifiedBy?: string;
  /** Raw evidence payload captured at verification time. */
  evidence?: string;
  /** Free-form verifier reasoning. */
  verifierReasoning?: string;
  onClose: () => void;
};

/**
 * Popup that consolidates "Evidence" and "Verifier reasoning" for a
 * satisfied checklist criterion. Inline `<details>` disclosures made
 * each criterion row visually heavy and pushed the checklist past
 * single-glance comprehension once 4+ items completed; promoting that
 * detail into an on-demand sheet keeps the row scannable while still
 * making the verification audit trail one click away.
 */
export function ChecklistVerificationModal({
  criterionText,
  verifiedBy,
  evidence,
  verifierReasoning,
  onClose,
}: Props) {
  const hasEvidence = typeof evidence === "string" && evidence.length > 0;
  const hasReasoning =
    typeof verifierReasoning === "string" && verifierReasoning.length > 0;

  return (
    <Modal
      onClose={onClose}
      labelledBy="checklist-verification-title"
      describedBy="checklist-verification-desc"
      size="wide"
    >
      <section className="panel modal-sheet checklist-verification-modal">
        <header className="checklist-verification-modal-head">
          <p className="checklist-verification-modal-eyebrow">
            Verification details
            {verifiedBy ? (
              <>
                {" · "}
                <span className="checklist-verification-modal-verifier">
                  by <code>{verifiedBy}</code>
                </span>
              </>
            ) : null}
          </p>
          <h2
            id="checklist-verification-title"
            className="checklist-verification-modal-title"
          >
            {criterionText}
          </h2>
          <p
            id="checklist-verification-desc"
            className="checklist-verification-modal-lede muted"
          >
            Captured at the moment the verifier accepted this criterion. The
            payload below is read-only — editing or removing a satisfied
            criterion would rewrite the audit trail.
          </p>
        </header>

        <div className="checklist-verification-modal-body">
          {hasEvidence ? (
            <section
              className="checklist-verification-modal-section"
              aria-labelledby="checklist-verification-evidence-heading"
            >
              <h3
                id="checklist-verification-evidence-heading"
                className="checklist-verification-modal-section-heading"
              >
                Evidence
              </h3>
              <pre className="checklist-verification-modal-pre">
                {evidence!.slice(0, CHECKLIST_EVIDENCE_DISPLAY_CAP)}
              </pre>
              {evidence!.length > CHECKLIST_EVIDENCE_DISPLAY_CAP ? (
                <p className="checklist-verification-modal-truncated">
                  Truncated to {CHECKLIST_EVIDENCE_DISPLAY_CAP.toLocaleString()}{" "}
                  characters for display. The full payload is preserved on the
                  server.
                </p>
              ) : null}
            </section>
          ) : null}

          {hasReasoning ? (
            <section
              className="checklist-verification-modal-section"
              aria-labelledby="checklist-verification-reasoning-heading"
            >
              <h3
                id="checklist-verification-reasoning-heading"
                className="checklist-verification-modal-section-heading"
              >
                Verifier reasoning
              </h3>
              <pre className="checklist-verification-modal-pre">
                {verifierReasoning}
              </pre>
            </section>
          ) : null}

          {!hasEvidence && !hasReasoning ? (
            <p className="checklist-verification-modal-empty muted">
              No additional verification detail was captured for this
              criterion.
            </p>
          ) : null}
        </div>

        <div className="row stack-row-actions checklist-verification-modal-footer">
          <button
            type="button"
            className="secondary"
            onClick={onClose}
            autoFocus
          >
            Close
          </button>
        </div>
      </section>
    </Modal>
  );
}
