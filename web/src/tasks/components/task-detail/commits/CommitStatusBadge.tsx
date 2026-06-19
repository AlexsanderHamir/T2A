import type { CommitStatus } from "@/types";
import { commitStatusLabel, commitStatusPillClass } from "./commitDisplay";

export function CommitStatusBadge({
  status,
  gateReason,
}: {
  status: CommitStatus;
  gateReason?: string;
}) {
  const title = gateReason?.trim() ? gateReason : undefined;
  return (
    <span
      className={commitStatusPillClass(status)}
      title={title}
      data-testid="task-commit-status"
    >
      {commitStatusLabel(status)}
    </span>
  );
}
