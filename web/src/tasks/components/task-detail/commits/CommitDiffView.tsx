import { useMemo } from "react";
import { Diff, Hunk, parseDiff, type FileData } from "react-diff-view";
import "react-diff-view/style/index.css";

type Props = {
  patch: string;
  className?: string;
};

function fileLabel(file: FileData): string {
  return file.newPath ?? file.oldPath ?? "unknown";
}

export function CommitDiffView({ patch, className }: Props) {
  const files = useMemo(() => {
    const trimmed = patch.trim();
    if (!trimmed) {
      return [];
    }
    try {
      return parseDiff(patch);
    } catch {
      return [];
    }
  }, [patch]);

  if (files.length === 0) {
    return (
      <p className="task-commit-diff-empty muted">No file changes in this commit.</p>
    );
  }

  return (
    <div className={className ?? "task-commit-diff-view"}>
      {files.map((file) => (
        <section
          key={`${file.oldRevision}-${file.newRevision}-${fileLabel(file)}`}
          className="task-commit-diff-file"
        >
          <header className="task-commit-diff-file-head">
            <code className="task-commit-diff-file-path">{fileLabel(file)}</code>
          </header>
          <Diff
            viewType="unified"
            diffType={file.type}
            hunks={file.hunks}
          >
            {(hunks) =>
              hunks.map((hunk) => <Hunk key={hunk.content} hunk={hunk} />)
            }
          </Diff>
        </section>
      ))}
    </div>
  );
}

export function countDiffFiles(patch: string): number {
  const trimmed = patch.trim();
  if (!trimmed) {
    return 0;
  }
  try {
    return parseDiff(patch).length;
  } catch {
    return 0;
  }
}
