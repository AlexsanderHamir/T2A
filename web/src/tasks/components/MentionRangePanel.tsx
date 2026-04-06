import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { fetchRepoFile, type RepoFileResult } from "@/api/repo";
import { lineRangeFromSelection } from "@/lib/lineRangeFromSelection";
import { filePreviewLanguageFromPath } from "./filePreviewLanguage";
import { highlightPreviewContent } from "./filePreviewHighlight";

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
  const codeContentRef = useRef<HTMLElement>(null);
  const [selStart, setSelStart] = useState(0);
  const [selEnd, setSelEnd] = useState(0);
  const [startLineInput, setStartLineInput] = useState("");
  const [endLineInput, setEndLineInput] = useState("");
  const [insertError, setInsertError] = useState<string | null>(null);

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

  const syncScroll = useCallback(() => {
    const ta = taRef.current;
    const codeContent = codeContentRef.current;
    if (!ta || !codeContent) return;
    codeContent.style.transform = `translate(${-ta.scrollLeft}px, ${-ta.scrollTop}px)`;
  }, []);

  const range = useMemo(() => {
    if (!file || file.binary) return null;
    return lineRangeFromSelection(file.content, selStart, selEnd);
  }, [file, selStart, selEnd]);

  const manualRange = useMemo(() => {
    const start = Number(startLineInput);
    const end = Number(endLineInput);
    if (!Number.isInteger(start) || !Number.isInteger(end)) return null;
    if (start < 1 || end < 1 || start > end) return null;
    return { startLine: start, endLine: end };
  }, [endLineInput, startLineInput]);

  const activeRange = manualRange ?? range;

  const showLargeHint = useMemo(() => {
    if (!file || file.binary) return false;
    return file.size_bytes > LARGE_BYTES || file.line_count > LARGE_LINES;
  }, [file]);

  const canInsertRange =
    Boolean(
      !disabled &&
        file &&
        !file.binary &&
        activeRange &&
        activeRange.startLine <= activeRange.endLine,
    );

  const handleInsertWithRange = useCallback(async () => {
    if (!activeRange) return;
    setInsertError(null);
    try {
      await onInsertWithRange(activeRange.startLine, activeRange.endLine);
    } catch (e: unknown) {
      console.error("[MentionRangePanel] insert line range failed", e);
      setInsertError(
        e instanceof Error
          ? e.message
          : "Insert failed. Please try again.",
      );
    }
  }, [activeRange, onInsertWithRange]);

  const taId = `${id}-mention-file-preview`;
  const detectedLanguage = useMemo(() => filePreviewLanguageFromPath(path), [path]);
  const highlightedPreview = useMemo(() => {
    if (!file || file.binary) return "";
    return highlightPreviewContent(file.content, detectedLanguage.prism);
  }, [detectedLanguage.prism, file]);
  const previewLineCount = useMemo(() => {
    if (!file || file.binary) return 10;
    return Math.max(8, Math.min(file.line_count, 18));
  }, [file]);

  useEffect(() => {
    syncScroll();
  }, [highlightedPreview, syncScroll]);

  return (
    <div
      className="mention-range-panel"
      role="region"
      aria-label="File mention and line range"
    >
      <div className="mention-range-header">
        <p className="mention-range-path">
          <code>{path}</code>
        </p>
        <p className="muted mention-range-hint">
          Optional: select text or type line numbers to insert a specific range.
        </p>
      </div>

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
          <div className="mention-range-content">
            <label className="mention-range-preview-label" htmlFor={taId}>
              Preview
              <span className="mention-range-lang-pill" aria-label="Detected file language">
                {detectedLanguage.label}
              </span>
            </label>
            <div
              className="mention-file-preview-shell"
              style={{ "--mention-preview-lines": previewLineCount } as Record<string, string | number>}
            >
              <pre
                className={`mention-file-preview-code mention-file-preview-code--${detectedLanguage.prism}`}
                aria-hidden="true"
              >
                <code
                  ref={codeContentRef}
                  dangerouslySetInnerHTML={{ __html: highlightedPreview }}
                />
              </pre>
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
                onScroll={syncScroll}
              />
            </div>
            <p id={`${taId}-hint`} className="visually-hidden">
              Select text to set the start and end line for this file mention.
            </p>
            <div className="row mention-range-row mention-range-inputs">
              <label className="field">
                <span>From line</span>
                <input
                  type="number"
                  min={1}
                  inputMode="numeric"
                  value={startLineInput}
                  onChange={(e) => setStartLineInput(e.target.value)}
                  placeholder={range ? String(range.startLine) : "1"}
                  disabled={disabled}
                />
              </label>
              <label className="field">
                <span>To line</span>
                <input
                  type="number"
                  min={1}
                  inputMode="numeric"
                  value={endLineInput}
                  onChange={(e) => setEndLineInput(e.target.value)}
                  placeholder={range ? String(range.endLine) : "1"}
                  disabled={disabled}
                />
              </label>
            </div>
            <p className="mention-range-selection" aria-live="polite">
              {activeRange ? (
                <>
                  Range{" "}
                  <strong>
                    {activeRange.startLine}–{activeRange.endLine}
                  </strong>
                </>
              ) : (
                <span className="muted">No range selected</span>
              )}
            </p>
          </div>
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
      {insertError ? (
        <p className="mention-warn" role="alert">
          {insertError}
        </p>
      ) : null}

      <div className="row stack-row-actions mention-range-actions">
        <button type="button" className="secondary" disabled={disabled} onClick={onInsertPathOnly}>
          Insert file only
        </button>
        <button
          type="button"
          className="mention-range-action-primary"
          disabled={disabled || !canInsertRange}
          onClick={() => {
            void handleInsertWithRange();
          }}
        >
          Insert line range
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
