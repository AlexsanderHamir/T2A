export const CREATE_CHECKLIST_REQUIRED_MSG = "Add at least one done criterion.";

export function nonEmptyChecklistCount(items: string[]): number {
  let n = 0;
  for (const raw of items) {
    if (raw.trim() !== "") {
      n++;
    }
  }
  return n;
}

export function normalizeChecklistItems(items: string[]): Array<{ text: string }> {
  return items
    .map((text) => text.trim())
    .filter(Boolean)
    .map((text) => ({ text }));
}
