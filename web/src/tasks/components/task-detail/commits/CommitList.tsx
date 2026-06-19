import { useCallback, useState } from "react";
import type { CycleCommit } from "@/types";
import { CommitRow } from "./CommitRow";

type Props = {
  commits: ReadonlyArray<CycleCommit>;
  /** When true, show attempt number in each row (task-wide panel). */
  showAttempt?: boolean;
};

export function CommitList({ commits, showAttempt = false }: Props) {
  const [expandedSha, setExpandedSha] = useState<string | null>(null);

  const handleToggle = useCallback((sha: string, nextOpen: boolean) => {
    setExpandedSha(nextOpen ? sha : null);
  }, []);

  return (
    <ul className="task-commits-list" data-testid="task-commits-list">
      {commits.map((commit) => (
        <CommitRow
          key={commit.sha}
          commit={commit}
          showAttempt={showAttempt}
          open={expandedSha === commit.sha}
          onToggle={handleToggle}
        />
      ))}
    </ul>
  );
}
