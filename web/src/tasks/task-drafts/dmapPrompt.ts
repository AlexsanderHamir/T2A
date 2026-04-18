/**
 * DMAP draft prompt helpers.
 *
 * "DMAP" is a `task_type` that posts to `/tasks` with `task_type: "general"` and
 * an `initial_prompt` whose first lines describe the DMAP session contract
 * (commit limit, domain, optional direction). Keeping the formatter and
 * commit-limit normalizer here lets consumers test wire shape independent of
 * the create/edit modal hook.
 */

/**
 * Coerce the raw commit-limit string from the form into a positive integer.
 * Falls back to 1 for empty / malformed input so the prompt always has a
 * concrete cap (the server defaults the same way; mirrored client-side so the
 * preview text matches what the agent will see).
 */
export function normalizeDmapCommitLimit(value: string): number {
  const parsed = Number.parseInt(value, 10);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : 1;
}

export type DmapPromptInput = {
  commitLimit: string;
  domain: string;
  description: string;
};

/**
 * Render the leading prompt block for a DMAP session.
 * Domain is required upstream; an empty domain still renders "unspecified" so
 * the agent gets a visible placeholder rather than a missing field.
 */
export function buildDmapPrompt(input: DmapPromptInput): string {
  const lines = [
    "DMAP session setup",
    "",
    `- Commits until stoppage: ${normalizeDmapCommitLimit(input.commitLimit)}`,
    `- Domain: ${input.domain.trim() || "unspecified"}`,
  ];
  if (input.description.trim()) {
    lines.push(`- Direction: ${input.description.trim()}`);
  }
  return lines.join("\n");
}
