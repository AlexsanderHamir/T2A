import { useCallback, type FormEvent } from "react";
import { DEFAULT_NEW_TASK_STATUS, type Priority } from "@/types";
import { buildCreateTaskMutationInput } from "../buildCreateMutationInput";
import { validateCreateFormChecklist } from "../validateCreateForm";
import type { useTaskCreateFormState } from "./useTaskCreateFormState";
import type { useTaskCreateMutations } from "./useTaskCreateMutations";

export function useTaskCreateSubmitActions(input: {
  form: ReturnType<typeof useTaskCreateFormState>;
  mutations: ReturnType<typeof useTaskCreateMutations>;
}) {
  const evaluateDraftBeforeCreate = useCallback(() => {
    const validationError = validateCreateFormChecklist(
      input.form.newTitle,
      input.form.newPriority,
      input.form.newChecklistItems,
    );
    if (!input.form.newTitle.trim() || !input.form.newPriority) return;
    if (validationError) {
      input.form.setCreateFormError(validationError);
      return;
    }
    input.form.setCreateFormError(null);
    input.mutations.evaluateDraftMutation.mutate({
      id: input.form.newDraftID,
      title: input.form.newTitle.trim(),
      initial_prompt: input.form.newPrompt,
      status: DEFAULT_NEW_TASK_STATUS,
      priority: input.form.newPriority as Priority,
      checklistItems: input.form.newChecklistItems,
    });
  }, [input]);

  const submitCreate = useCallback(async (event: FormEvent) => {
    event.preventDefault();
    const validationError = validateCreateFormChecklist(
      input.form.newTitle,
      input.form.newPriority,
      input.form.newChecklistItems,
    );
    if (!input.form.newTitle.trim() || !input.form.newPriority) return;
    if (validationError) {
      input.form.setCreateFormError(validationError);
      return;
    }
    input.form.setCreateFormError(null);
    input.mutations.createMutation.mutate(buildCreateTaskMutationInput(input.form.formFields));
  }, [input]);

  return {
    evaluateDraftBeforeCreate,
    submitCreate,
  };
}
