import { useCallback, type FormEvent } from "react";
import { buildCreateTaskMutationInput } from "../buildCreateMutationInput";
import { validateCreateFormChecklist } from "../validateCreateForm";
import type { useTaskCreateFormState } from "./useTaskCreateFormState";
import type { useTaskCreateMutations } from "./useTaskCreateMutations";

export function useTaskCreateSubmitActions(input: {
  form: ReturnType<typeof useTaskCreateFormState>;
  mutations: ReturnType<typeof useTaskCreateMutations>;
}) {
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
    submitCreate,
  };
}
