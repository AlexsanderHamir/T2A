import type { ReactNode } from "react";

type Props = {
  /** Optional toolbar on the title row (e.g. home “New task”). */
  actions?: ReactNode;
};

export function TaskListSectionHeading({ actions }: Props) {
  if (actions) {
    return (
      <div className="task-list-section-head">
        <h2 id="task-list-heading">All tasks</h2>
        <div className="task-list-section-actions">{actions}</div>
      </div>
    );
  }
  return <h2 id="task-list-heading">All tasks</h2>;
}
