export type CreateEntryDecision =
  | { kind: "showPicker" }
  | { kind: "openFreshForm"; entryDraftErrorHint: string | null };

export function decideCreateEntry(input: {
  isPending: boolean;
  isError: boolean;
  errorMessage: string | null;
  draftCount: number;
}): CreateEntryDecision {
  if (input.isPending) {
    return { kind: "showPicker" };
  }
  if (input.isError) {
    return { kind: "openFreshForm", entryDraftErrorHint: input.errorMessage };
  }
  if (input.draftCount > 0) {
    return { kind: "showPicker" };
  }
  return { kind: "openFreshForm", entryDraftErrorHint: null };
}
