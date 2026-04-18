import type { PriorityChoice, TaskType } from "@/types";
import type { PendingSubtaskDraft } from "../task-tree";

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
  taskType: TaskType;
  parentId: string;
  checklistInherit: boolean;
  checklistItems: string[];
  pendingSubtasks: PendingSubtaskDraft[];
  latestEvaluation: {
    overallScore: number;
    overallSummary: string;
    sections: Array<{ key: string; score: number }>;
  } | null;
  dmapConfig: {
    commitLimit: string;
    domain: string;
    description: string;
  };
};

/**
 * Stable, JSON-serialized fingerprint of a create-task draft.
 * Used by the autosave loop to short-circuit no-op writes: when the current
 * signature equals the last-saved baseline, the debounce timer skips POST.
 *
 * The shape mirrors `TaskDraftPayload` so the baseline reflects exactly what
 * the server would persist (modulo `dmap_config`, which the hook adds when
 * `task_type === "dmap"` because the wire field is conditional).
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
      task_type: input.taskType,
      parent_id: input.parentId,
      checklist_inherit: input.checklistInherit,
      checklist_items: input.checklistItems,
      pending_subtasks: input.pendingSubtasks.map((st) => ({
        title: st.title,
        initial_prompt: st.initial_prompt,
        priority: st.priority,
        task_type: st.task_type,
        checklist_items: st.checklistItems,
        checklist_inherit: st.checklist_inherit,
      })),
      latest_evaluation: input.latestEvaluation,
      dmap_config: input.dmapConfig,
    },
  });
}
