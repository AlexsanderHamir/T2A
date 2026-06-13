import type { PriorityChoice } from "@/types";

/**
 * Treat editor-empty TipTap markup as the empty string when computing the
 * autosave signature. Without this, opening the create modal and tabbing
 * through fields would produce `<p></p>` / `<p><br></p>` / NBSP-only
 * paragraphs that look identical to the user but flip the dirty bit and
 * trigger pointless POST /tasks/drafts writes.
 */
export function normalizeDraftPromptForDirty(prompt: string): string {
  const compact = prompt.replace(/[\s\u200B\uFEFF]/g, "").toLowerCase();
  if (
    compact === "" ||
    compact === "<p></p>" ||
    compact === "<p><br></p>" ||
    /^<p>(<br\/?>|&nbsp;|&#160;)*<\/p>$/.test(compact)
  ) {
    return "";
  }
  return prompt;
}

export type DraftAutosaveSignatureInput = {
  id: string;
  name: string;
  title: string;
  prompt: string;
  priority: PriorityChoice;
  runner: string;
  cursorModel: string;
  /**
   * Project the operator is composing against. Empty string means "no
   * project bound". Folded into the autosave signature so changing the
   * project (or its selected context items) flips the dirty bit and
   * triggers an autosave.
   */
  projectId: string;
  /** User-selected project context item ids. Order matters and is preserved. */
  projectContextItemIds: string[];
  checklistItems: string[];
  latestEvaluation: {
    overallScore: number;
    overallSummary: string;
    sections: Array<{ key: string; score: number }>;
  } | null;
};

/**
 * Stable, JSON-serialized fingerprint of a create-task draft.
 * Used by the autosave loop to short-circuit no-op writes: when the current
 * signature equals the last-saved baseline, the debounce timer skips POST.
 *
 * The shape mirrors `TaskDraftPayload` so the baseline reflects exactly what
 * the server would persist.
 */
export function draftAutosaveSignature(
  input: DraftAutosaveSignatureInput,
): string {
  return JSON.stringify({
    id: input.id,
    name: input.name,
    payload: {
      title: input.title,
      initial_prompt: normalizeDraftPromptForDirty(input.prompt),
      priority: input.priority,
      runner: input.runner,
      cursor_model: input.cursorModel,
      project_id: input.projectId,
      project_context_item_ids: input.projectContextItemIds,
      checklist_items: input.checklistItems,
      latest_evaluation: input.latestEvaluation,
    },
  });
}
