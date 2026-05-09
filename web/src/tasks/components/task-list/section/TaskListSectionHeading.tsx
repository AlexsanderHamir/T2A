import type { ReactNode } from "react";

type Props = {
  /** Optional toolbar on the title row (e.g. home “New task”). */
  actions?: ReactNode;
};

export function TaskListSectionHeading({ actions }: Props) {
  return (
    <div className="task-list-section-head">
      <div className="task-list-section-head__text">
        <h2 id="task-list-heading" className="task-list-section-title">
          All tasks
        </h2>
      </div>
      {actions ? (
        <div className="task-list-section-actions">{actions}</div>
      ) : null}
    </div>
  );
}
