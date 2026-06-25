import { errorMessage } from "@/lib/errorMessage";
import type { useTaskCreateDraftAutosave } from "./hooks/useTaskCreateDraftAutosave";
import type { useTaskCreateEntryActions } from "./hooks/useTaskCreateEntryActions";
import type { useTaskCreateFormState } from "./hooks/useTaskCreateFormState";
import type { useTaskCreateModalState } from "./hooks/useTaskCreateModalState";
import type { useTaskCreateMutations } from "./hooks/useTaskCreateMutations";
import type { TaskDraftsQuery } from "./types";

export function deriveCreateFlowError(
  mutations: ReturnType<typeof useTaskCreateMutations>,
): string | null {
  if (mutations.createMutation.isError) {
    return errorMessage(mutations.createMutation.error);
  }
  if (mutations.saveTemplateMutation.isError) {
    return errorMessage(mutations.saveTemplateMutation.error);
  }
  if (mutations.patchTemplateMutation.isError) {
    return errorMessage(mutations.patchTemplateMutation.error);
  }
  return null;
}

export function mapCreateFlowViewModel(input: {
  createFlowError: string | null;
  form: ReturnType<typeof useTaskCreateFormState>;
  modal: ReturnType<typeof useTaskCreateModalState>;
  mutations: ReturnType<typeof useTaskCreateMutations>;
  autosave: ReturnType<typeof useTaskCreateDraftAutosave>;
  actions: ReturnType<typeof useTaskCreateEntryActions> &
    Pick<
      ReturnType<typeof import("./hooks/useTaskCreateSubmitActions").useTaskCreateSubmitActions>,
      "submitCreate" | "submitTemplate"
    > &
    Pick<
      ReturnType<typeof import("./hooks/useTaskCreateChecklistActions").useTaskCreateChecklistActions>,
      "appendNewChecklistCriterion" | "updateNewChecklistRow" | "removeNewChecklistRow" | "applyTestScenario"
    >;
  draftsQuery: TaskDraftsQuery;
}) {
  return {
    createFlowError: input.createFlowError,
    draftSavePending: input.mutations.saveDraftMutation.isPending,
    draftSaveLabel: input.autosave.draftSaveLabel,
    draftSaveError: input.autosave.draftSaveError,
    createPending: input.mutations.createMutation.isPending,
    templateSavePending:
      input.mutations.saveTemplateMutation.isPending ||
      input.mutations.patchTemplateMutation.isPending,
    templateSaveError:
      input.mutations.saveTemplateMutation.error ??
      input.mutations.patchTemplateMutation.error ??
      null,
    createError: input.mutations.createMutation.error,
    createFormError: input.form.createFormError,
    draftPickerOpen: input.modal.draftPickerOpen,
    setDraftPickerOpen: input.modal.setDraftPickerOpen,
    taskDrafts: input.draftsQuery.data ?? [],
    draftListLoading: input.draftsQuery.isPending,
    draftListError: input.draftsQuery.isError
      ? errorMessage(input.draftsQuery.error)
      : null,
    createEntryDraftErrorHint: input.modal.createEntryDraftErrorHint,
    repositorySetupPromptOpen: input.modal.repositorySetupPromptOpen,
    setRepositorySetupPromptOpen: input.modal.setRepositorySetupPromptOpen,
    retryDraftList: input.actions.retryDraftList,
    retryCreateEntryDraftLoad: input.actions.retryCreateEntryDraftLoad,
    deleteDraftPending: input.mutations.deleteDraftMutation.isPending,
    deleteTemplatePending: input.mutations.deleteTemplateMutation.isPending,
    instantiateTemplatesPending: input.mutations.instantiateTemplatesMutation.isPending,
    loadTemplatePending: input.mutations.loadTemplateMutation.isPending,
    deleteDraftError: input.mutations.deleteDraftMutation.isError
      ? errorMessage(input.mutations.deleteDraftMutation.error)
      : null,
    resumeDraftPending: input.mutations.resumeDraftMutation.isPending,
    resumeDraftError: input.mutations.resumeDraftMutation.isError
      ? errorMessage(input.mutations.resumeDraftMutation.error)
      : null,
    clearResumeDraftError: input.mutations.resumeDraftMutation.reset,
    newTitle: input.form.newTitle,
    setNewTitle: input.form.setNewTitle,
    newPrompt: input.form.newPrompt,
    setNewPrompt: input.form.setNewPrompt,
    newPriority: input.form.newPriority,
    setNewPriority: input.form.setNewPriority,
    newTaskRunner: input.form.newTaskRunner,
    setNewTaskRunner: input.form.setNewTaskRunner,
    newTaskCursorModel: input.form.newTaskCursorModel,
    setNewTaskCursorModel: input.form.setNewTaskCursorModel,
    newProjectID: input.form.newProjectID,
    setNewProjectID: input.form.setNewProjectID,
    newProjectContextItemIDs: input.form.newProjectContextItemIDs,
    setNewProjectContextItemIDs: input.form.setNewProjectContextItemIDs,
    newWorktreeID: input.form.newWorktreeID,
    setNewWorktreeID: input.form.setNewWorktreeID,
    newBranchID: input.form.newBranchID,
    setNewBranchID: input.form.setNewBranchID,
    newSchedule: input.form.newSchedule,
    setNewSchedule: input.form.setNewSchedule,
    newAutonomyEnabled: input.form.newAutonomyEnabled,
    setNewAutonomyEnabled: input.form.setNewAutonomyEnabled,
    newTagsCsv: input.form.newTagsCsv,
    setNewTagsCsv: input.form.setNewTagsCsv,
    newMilestone: input.form.newMilestone,
    setNewMilestone: input.form.setNewMilestone,
    newDependsOn: input.form.newDependsOn,
    setNewDependsOn: input.form.setNewDependsOn,
    newChecklistItems: input.form.newChecklistItems,
    appendNewChecklistCriterion: input.actions.appendNewChecklistCriterion,
    updateNewChecklistRow: input.actions.updateNewChecklistRow,
    removeNewChecklistRow: input.actions.removeNewChecklistRow,
    submitCreate: input.actions.submitCreate,
    submitTemplate: input.actions.submitTemplate,
    startFreshDraft: input.actions.startFreshDraft,
    saveDraftNow: input.autosave.saveDraftNow,
    resumeDraftByID: input.actions.resumeDraftByID,
    deleteDraftByID: input.actions.deleteDraftByID,
    applyTestScenario: input.actions.applyTestScenario,
    createModalOpen: input.modal.createModalOpen,
    createModalAssignmentLocked: input.modal.createModalAssignmentLocked,
    openCreateModal: input.actions.openCreateModal,
    openTemplateCreateModal: input.actions.openTemplateCreateModal,
    closeCreateModal: input.modal.closeCreateModal,
    editingTaskId: input.modal.editingTaskId,
    editingTemplateId: input.modal.editingTemplateId,
    composeTarget: input.modal.composeTarget,
    composeOperation: input.modal.composeOperation,
    editTemplateByID: input.actions.editTemplateByID,
    deleteTemplateByID: input.actions.deleteTemplateByID,
    instantiateTemplates: input.actions.instantiateTemplates,
    editingTaskRunner: input.modal.editingTaskRunner,
    composeStatus: input.modal.composeStatus,
    setComposeStatus: input.modal.setComposeStatus,
    beginEditSession: input.modal.beginEditSession,
  };
}

export type UseTaskCreateFlowReturn = ReturnType<typeof mapCreateFlowViewModel>;
