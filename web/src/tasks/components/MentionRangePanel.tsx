import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { fetchRepoFile, type RepoFileResult } from "@/api/repo";
import { lineRangeFromSelection } from "@/lib/lineRangeFromSelection";

export type MentionRangePanelProps = {
  id: string;
  path: string;
  disabled?: boolean;
  rangeWarning: string | null;
  onInsertWithRange: (startLine: number, endLine: number) => void | Promise<void>;
  onInsertPathOnly: () => void;
  onCancel: () => void;
};

const LARGE_BYTES = 1_000_000;
const LARGE_LINES = 10_000;

export function MentionRangePanel({
  id,
  path,
  disabled,
  rangeWarning,
  onInsertWithRange,
  onInsertPathOnly,
  onCancel,
}: MentionRangePanelProps) {
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [file, setFile] = useState<RepoFileResult | null>(null);
  const taRef = useRef<HTMLTextAreaElement>(null);
  const [selStart, setSelStart] = useState(0);
  const [selEnd, setSelEnd] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setLoadError(null);
    setFile(null);
    void fetchRepoFile(path)
      .then((r) => {
        if (cancelled) return;
        if (r === null) {
          setLoadError("File preview is unavailable.");
          return;
        }
        setFile(r);
      })
      .catch((e: unknown) => {
        if (!cancelled)
          setLoadError(e instanceof Error ? e.message : "Load failed");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [path]);

  const syncSelection = useCallback(() => {
    const ta = taRef.current;
    if (!ta) return;
    setSelStart(ta.selectionStart);
    setSelEnd(ta.selectionEnd);
  }, []);

  const range = useMemo(() => {
    if (!file || file.binary) return null;
    return lineRangeFromSelection(file.content, selStart, selEnd);
  }, [file, selStart, selEnd]);

  const showLargeHint = useMemo(() => {
    if (!file || file.binary) return false;
    return file.size_bytes > LARGE_BYTES || file.line_count > LARGE_LINES;
  }, [file]);

  const canInsertRange =
    Boolean(
      !disabled &&
        file &&
        !file.binary &&
        range &&
        range.startLine <= range.endLine,
    );

  const taId = `${id}-mention-file-preview`;

  return (
    <div
      className="mention-range-panel"
      role="region"
      aria-label="File mention and line range"
    >
      <p className="mention-range-path">
        <code>{path}</code>
      </p>
      <p className="muted mention-range-hint">
        Drag in the preview below to choose a line range (like selecting text in an
        editor), or insert the file reference without a range.
      </p>

      {loading ? (
        <p className="mention-range-status" role="status">
          Loading file…
        </p>
      ) : null}

      {loadError ? (
        <p className="mention-warn" role="alert">
          {loadError}
        </p>
      ) : null}

      {!loading && file?.warning ? (
        <p className="mention-range-banner" role="status">
          {file.warning}
        </p>
      ) : null}

      {showLargeHint ? (
        <p className="mention-range-banner mention-range-banner--soft" role="status">
          Large file — preview may scroll; selection applies to the visible content.
        </p>
      ) : null}

      {!loading && file && !file.binary ? (
        <>
          <label className="mention-range-preview-label" htmlFor={taId}>
            Preview
          </label>
          <textarea
            id={taId}
            ref={taRef}
            className="mention-file-preview"
            readOnly
            spellCheck={false}
            value={file.content}
            disabled={disabled}
            aria-describedby={`${taId}-hint`}
            onSelect={syncSelection}
            onMouseUp={syncSelection}
            onKeyUp={syncSelection}
          />
          <p id={`${taId}-hint`} className="visually-hidden">
            Select text to set the start and end line for this file mention.
          </p>
          <p className="mention-range-selection" aria-live="polite">
            {range ? (
              <>
                Selected lines{" "}
                <strong>
                  {range.startLine}–{range.endLine}
                </strong>
              </>
            ) : (
              <span className="muted">No selection — drag to highlight lines</span>
            )}
          </p>
        </>
      ) : null}

      {!loading && file?.binary ? (
        <p className="muted mention-range-binary-copy">
          Line range is not available for this file type.
        </p>
      ) : null}

      {rangeWarning ? (
        <p className="mention-warn" role="alert">
          {rangeWarning}
        </p>
      ) : null}

      <div className="row stack-row-actions mention-range-actions">
        <button
          type="button"
          disabled={disabled || !canInsertRange}
          onClick={() => {
            if (!range) return;
            void onInsertWithRange(range.startLine, range.endLine);
          }}
        >
          Insert selected range
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
