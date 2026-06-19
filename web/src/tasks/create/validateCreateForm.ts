import type { ChecklistItemDraft, PriorityChoice } from "@/types";
import {
  CREATE_CHECKLIST_REQUIRED_MSG,
  nonEmptyChecklistCount,
} from "../task-compose/checklistRequirement";

export function validateCreateFormChecklist(
  title: string,
  priority: PriorityChoice,
  checklistItems: ChecklistItemDraft[],
): string | null {
  if (!title.trim() || !priority) return null;
  if (nonEmptyChecklistCount(checklistItems) < 1) {
    return CREATE_CHECKLIST_REQUIRED_MSG;
  }
  return null;
}
