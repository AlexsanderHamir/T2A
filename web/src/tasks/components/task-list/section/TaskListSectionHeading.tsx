import type { ReactNode } from "react";

type Props = {
  /** Optional toolbar on the title row (e.g. home “New task”). */
  actions?: ReactNode;
};

export function TaskListSectionHeading({ actions }: Props) {
  // Keep the heading text "All tasks" verbatim — region tests
  // (TaskListSection.test.tsx) match the accessible name strictly
  // and the `term-prompt` class adds the "$" glyph as a CSS
  // pseudo-element, which jsdom does not include in the
  // accessible-name computation.
  if (actions) {
    return (
      <div className="task-list-section-head">
        <h2 id="task-list-heading" className="term-prompt">
          <span>All tasks</span>
        </h2>
        <div className="task-list-section-actions">{actions}</div>
      </div>
    );
  }
  return (
    <h2 id="task-list-heading" className="term-prompt">
      <span>All tasks</span>
    </h2>
  );
}
