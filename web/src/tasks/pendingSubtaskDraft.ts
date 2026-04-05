import type { Priority } from "@/types";

/** Child task queued while creating a parent on the home page (POST after parent exists). */
export type PendingSubtaskDraft = {
  title: string;
  initial_prompt: string;
  priority: Priority;
  checklistItems: string[];
  checklist_inherit: boolean;
};
