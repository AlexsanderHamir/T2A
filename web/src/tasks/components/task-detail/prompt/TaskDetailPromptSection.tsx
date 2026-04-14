import { promptHasVisibleContent } from "../../../promptFormat";

type TaskDetailPromptSectionProps = {
  initialPrompt: string;
  sanitizedInitialPrompt: string;
};

export function TaskDetailPromptSection({
  initialPrompt,
  sanitizedInitialPrompt,
}: TaskDetailPromptSectionProps) {
  return (
    <div className="task-detail-section task-detail-prompt">
      <h3 className="task-detail-section-heading" id="task-detail-prompt-heading">
        Initial prompt
      </h3>
      {!promptHasVisibleContent(initialPrompt) ? (
        <p
          className="muted task-detail-prompt-empty"
          aria-labelledby="task-detail-prompt-heading"
        >
          —
        </p>
      ) : (
        <details className="task-detail-prompt-details">
          <summary className="task-detail-prompt-summary">
            <span className="task-detail-prompt-summary-open-label">
              Show full initial prompt
            </span>
            <span className="task-detail-prompt-summary-close-label">
              Hide initial prompt
            </span>
            <span
              className="task-detail-prompt-summary-chevron"
              aria-hidden="true"
            >
              ▾
            </span>
          </summary>
          <div
            className="task-detail-prompt-body"
            dangerouslySetInnerHTML={{ __html: sanitizedInitialPrompt }}
          />
        </details>
      )}
    </div>
  );
}
