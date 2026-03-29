export type MentionRangePanelProps = {
  id: string;
  path: string;
  disabled?: boolean;
  lineStart: string;
  lineEnd: string;
  rangeWarning: string | null;
  onLineStartChange: (value: string) => void;
  onLineEndChange: (value: string) => void;
  onInsertWithRange: () => void;
  onInsertPathOnly: () => void;
  onCancel: () => void;
};

export function MentionRangePanel({
  id,
  path,
  disabled,
  lineStart,
  lineEnd,
  rangeWarning,
  onLineStartChange,
  onLineEndChange,
  onInsertWithRange,
  onInsertPathOnly,
  onCancel,
}: MentionRangePanelProps) {
  return (
    <div
      className="mention-range-panel"
      role="region"
      aria-label="Optional line range for file mention"
    >
      <p className="muted stack-tight-zero">
        <code>{path}</code>
      </p>
      <p className="muted stack-tight-zero mention-range-hint">
        Add a line range (optional), or insert the file reference only.
      </p>
      <div className="row mention-range-row">
        <div className="field">
          <label htmlFor={`${id}-line-start`}>From line</label>
          <input
            id={`${id}-line-start`}
            type="number"
            min={1}
            value={lineStart}
            disabled={disabled}
            onChange={(e) => onLineStartChange(e.target.value)}
          />
        </div>
        <div className="field">
          <label htmlFor={`${id}-line-end`}>To line</label>
          <input
            id={`${id}-line-end`}
            type="number"
            min={1}
            value={lineEnd}
            disabled={disabled}
            onChange={(e) => onLineEndChange(e.target.value)}
          />
        </div>
      </div>
      {rangeWarning ? (
        <p className="mention-warn" role="alert">
          {rangeWarning}
        </p>
      ) : null}
      <div className="row stack-row-actions">
        <button
          type="button"
          disabled={disabled}
          onClick={() => void onInsertWithRange()}
        >
          Insert with range
        </button>
        <button
          type="button"
          className="secondary"
          disabled={disabled}
          onClick={onInsertPathOnly}
        >
          Insert file only
        </button>
        <button
          type="button"
          className="secondary"
          disabled={disabled}
          onClick={onCancel}
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
