import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo } from "react";
import { listTaskDrafts as apiListDrafts } from "@/api";
import { TASK_DRAFTS } from "@/constants/tasks";
import { taskQueryKeys } from "../../task-query";
import { deriveCreateFlowError, mapCreateFlowViewModel } from "../mapCreateFlowViewModel";
import { useTaskCreateChecklistActions } from "./useTaskCreateChecklistActions";
import { useTaskCreateDraftAutosave } from "./useTaskCreateDraftAutosave";
import { useTaskCreateEntryActions } from "./useTaskCreateEntryActions";
import { useTaskCreateFormState } from "./useTaskCreateFormState";
import { useTaskCreateModalState } from "./useTaskCreateModalState";
import { useTaskCreateMutations } from "./useTaskCreateMutations";
import { useTaskCreateSubmitActions } from "./useTaskCreateSubmitActions";

/**
 * Create-task modal, draft autosave, draft picker, and related mutations.
 * Composed by `useTasksApp`.
 */
export function useTaskCreateFlow() {
  const queryClient = useQueryClient();
  const form = useTaskCreateFormState(queryClient);
  const modal = useTaskCreateModalState(
    form.resetFormFields,
    form.populateFromTask,
    form.setNewChecklistItems,
    form.setNewProjectID,
  );
  const draftsQuery = useQuery({
    queryKey: taskQueryKeys.drafts(),
    queryFn: ({ signal }) =>
      apiListDrafts(TASK_DRAFTS.createModalDraftListLimit, { signal }),
  });
  const mutations = useTaskCreateMutations({
    queryClient,
    newDraftIDRef: form.newDraftIDRef,
    newDraftID: form.newDraftID,
    closeCreateModal: modal.closeCreateModal,
    setNewDraftID: form.setNewDraftID,
    setDraftAutosaveBaseline: form.setDraftAutosaveBaseline,
    setDraftAutosaveBaselineID: form.setDraftAutosaveBaselineID,
    setLastDraftSavedAt: form.setLastDraftSavedAt,
    createModalOpen: modal.createModalOpen,
  });
  const autosave = useTaskCreateDraftAutosave({
    formFields: form.formFields,
    draftAutosaveBaseline: form.draftAutosaveBaseline,
    draftAutosaveBaselineID: form.draftAutosaveBaselineID,
    editingTaskId: modal.editingTaskId,
    createModalOpen: modal.createModalOpen,
    autosaveTimerRef: form.autosaveTimerRef,
    saveDraftMutation: mutations.saveDraftMutation,
    lastDraftSavedAt: form.lastDraftSavedAt,
  });
  const entryActions = useTaskCreateEntryActions({
    form,
    modal,
    mutations,
    draftsQuery,
    queryClient,
  });
  const submitActions = useTaskCreateSubmitActions({ form, mutations });
  const checklistActions = useTaskCreateChecklistActions({ form });
  const actions = { ...entryActions, ...submitActions, ...checklistActions };
  const { createMutation } = mutations;
  const createFlowError = useMemo(
    () => deriveCreateFlowError(mutations),
    [createMutation.error, createMutation.isError],
  );

  return mapCreateFlowViewModel({
    createFlowError,
    form,
    modal,
    mutations,
    autosave,
    actions,
    draftsQuery,
  });
}
