import { useMemo } from "react";
import {
  Diff,
  Hunk,
  markEdits,
  tokenize,
  type FileData,
  type ViewType,
} from "react-diff-view";
import { filePreviewLanguageFromPath } from "@/components/file-preview";
import {
  fileAnchorId,
  fileDiffStats,
  fileDisplayPath,
  fileStatusLabel,
  isBinaryDiffFile,
} from "./diffStats";
import { getDiffRefractor, refractorLanguageForPath } from "./diffRefractor";
import { useCopyToClipboard } from "./useCopyToClipboard";

type Props = {
  file: FileData;
  viewMode: ViewType;
  expanded: boolean;
  onToggleExpanded: () => void;
};

export function CommitDiffFile({
  file,
  viewMode,
  expanded,
  onToggleExpanded,
}: Props) {
  const displayPath = fileDisplayPath(file);
  const oldPath = file.oldPath ?? "";
  const newPath = file.newPath ?? "";
  const renamed = oldPath !== "" && newPath !== "" && oldPath !== newPath;
  const stats = fileDiffStats(file);
  const language = filePreviewLanguageFromPath(displayPath);
  const pathCopy = useCopyToClipboard("Copy path");
  const binary = isBinaryDiffFile(file);

  const tokens = useMemo(() => {
    if (binary || !expanded) {
      return undefined;
    }
    const lang = refractorLanguageForPath(displayPath);
    if (!lang) {
      return tokenize(file.hunks, {
        enhancers: [markEdits(file.hunks)],
      });
    }
    return tokenize(file.hunks, {
      highlight: true,
      refractor: getDiffRefractor(),
      language: lang,
    });
  }, [binary, displayPath, expanded, file.hunks]);

  return (
    <section
      id={fileAnchorId(displayPath)}
      className={
        expanded
          ? "task-commit-diff-file"
          : "task-commit-diff-file task-commit-diff-file--collapsed"
      }
      data-testid="task-commit-diff-file"
    >
      <header className="task-commit-diff-file-head">
        <div className="task-commit-diff-file-head-main">
          <span
            className={`task-commit-diff-file-status task-commit-diff-file-status--${file.type}`}
          >
            {fileStatusLabel(file.type)}
          </span>
          {renamed ? (
            <span className="task-commit-diff-file-paths">
              <code className="task-commit-diff-file-path">{oldPath}</code>
              <span className="task-commit-diff-file-rename-arrow" aria-hidden="true">
                →
              </span>
              <code className="task-commit-diff-file-path">{newPath}</code>
            </span>
          ) : (
            <code className="task-commit-diff-file-path">{displayPath}</code>
          )}
          <span className="task-commit-diff-file-lang muted">{language.label}</span>
          {!expanded ? (
            <span className="task-commit-diff-file-collapsed-hint muted">
              {stats.changedLines} changed {stats.changedLines === 1 ? "line" : "lines"}{" "}
              hidden
            </span>
          ) : null}
        </div>
        <div className="task-commit-diff-file-head-actions">
          {stats.additions > 0 ? (
            <span className="task-commit-diff-file-stat task-commit-diff-file-stat--add">
              +{stats.additions}
            </span>
          ) : null}
          {stats.deletions > 0 ? (
            <span className="task-commit-diff-file-stat task-commit-diff-file-stat--del">
              −{stats.deletions}
            </span>
          ) : null}
          <button
            type="button"
            className="btn-utility task-commit-diff-file-copy"
            onClick={() => pathCopy.copy(displayPath)}
          >
            {pathCopy.copyLabel}
          </button>
          <button
            type="button"
            className="btn-utility task-commit-diff-file-toggle"
            aria-expanded={expanded}
            onClick={onToggleExpanded}
          >
            {expanded ? "Collapse file" : "Expand file"}
          </button>
        </div>
      </header>
      {expanded ? (
        binary ? (
          <p className="task-commit-diff-binary muted" role="status">
            Binary file not shown.
          </p>
        ) : (
          <div className="task-commit-diff-file-body task-commit-diff-code">
            <Diff
              viewType={viewMode}
              diffType={file.type}
              hunks={file.hunks}
              tokens={tokens}
            >
              {(hunks) =>
                hunks.map((hunk) => <Hunk key={hunk.content} hunk={hunk} />)
              }
            </Diff>
          </div>
        )
      ) : null}
    </section>
  );
}
