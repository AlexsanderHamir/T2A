export type TaskPagerProps = {
  /** Short range description, e.g. "1–20 of 45" or "Page 2" */
  summary: string;
  onPrev: () => void;
  onNext: () => void;
  disablePrev: boolean;
  disableNext: boolean;
  /** `aria-label` on the nav */
  navLabel: string;
};

export function TaskPager({
  summary,
  onPrev,
  onNext,
  disablePrev,
  disableNext,
  navLabel,
}: TaskPagerProps) {
  return (
    <nav className="task-pager" aria-label={navLabel}>
      <button
        type="button"
        className="secondary task-pager-btn"
        onClick={onPrev}
        disabled={disablePrev}
      >
        Previous
      </button>
      <span className="task-pager-summary muted">{summary}</span>
      <button
        type="button"
        className="secondary task-pager-btn"
        onClick={onNext}
        disabled={disableNext}
      >
        Next
      </button>
    </nav>
  );
}
