/**
 * advancedSummaryLine builds the one-line caption shown next to the
 * collapsed "More options" disclosure on the new-task modal. It lets the
 * operator see the effective values for the secondary fields (agent,
 * schedule, tags, milestone, dependencies) without expanding the panel.
 *
 * Returns a stable fallback ("Agent, schedule, tags, dependencies") when
 * every input is at its default — that copy doubles as the affordance
 * description so the disclosure never reads as empty chrome.
 */
export function advancedSummaryLine(input: {
  runner: string;
  cursorModel: string;
  schedule: string | null;
  tagsCsv: string;
  milestone: string;
  dependsOn: string[];
}): string {
  const parts: string[] = [];

  const runnerLabel = runnerDisplayLabel(input.runner);
  const modelLabel = input.cursorModel.trim();
  if (modelLabel) {
    parts.push(`${runnerLabel} · ${modelLabel}`);
  }

  if (input.schedule) {
    parts.push("Scheduled");
  }

  const tagCount = countCsv(input.tagsCsv);
  if (tagCount > 0) {
    parts.push(`${tagCount} ${tagCount === 1 ? "tag" : "tags"}`);
  }

  if (input.milestone.trim()) {
    parts.push("Milestone");
  }

  const depCount = input.dependsOn.length;
  if (depCount > 0) {
    parts.push(`${depCount} ${depCount === 1 ? "dep" : "deps"}`);
  }

  if (parts.length === 0) {
    return "Agent, schedule, tags, dependencies";
  }

  return parts.join(" · ");
}

function runnerDisplayLabel(id: string): string {
  if (id === "cursor") return "Cursor CLI";
  return id || "Cursor CLI";
}

function countCsv(csv: string): number {
  if (!csv) return 0;
  return csv
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean).length;
}
