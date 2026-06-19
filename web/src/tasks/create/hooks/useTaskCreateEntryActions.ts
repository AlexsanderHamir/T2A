import type { QueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import type { AppSettings } from "@/api/settings";
import { errorMessage } from "@/lib/errorMessage";
import { settingsQueryKeys } from "../../task-query";
import { applyResumedDraftToForm } from "../draftPayload";
import { decideCreateEntry } from "../decideCreateEntry";
import type { TaskDraftsQuery } from "../types";
import type { useTaskCreateFormState } from "./useTaskCreateFormState";
import type { useTaskCreateModalState } from "./useTaskCreateModalState";
import type { useTaskCreateMutations } from "./useTaskCreateMutations";

export function useTaskCreateEntryActions(input: {
  form: ReturnType<typeof useTaskCreateFormState>;
  modal: ReturnType<typeof useTaskCreateModalState>;
  mutations: ReturnType<typeof useTaskCreateMutations>;
  draftsQuery: TaskDraftsQuery;
  queryClient: QueryClient;
}) {
  const openCreateModal = useCallback(
    (prefill?: { projectID: string; lockProjectAssignment?: boolean }) => {
      input.modal.setCreateEntryDraftErrorHint(null);
      const projectID = prefill?.projectID?.trim();
      input.modal.createModalPrefillRef.current = projectID
        ? {
            projectID,
            lockProjectAssignment: prefill?.lockProjectAssignment === true,
          }
        : null;
      const decision = decideCreateEntry({
        isPending: input.draftsQuery.isPending,
        isError: input.draftsQuery.isError,
        errorMessage: input.draftsQuery.isError
          ? errorMessage(input.draftsQuery.error)
          : null,
        draftCount: input.draftsQuery.data?.length ?? 0,
      });
      if (decision.kind === "showPicker") {
        input.modal.setDraftPickerOpen(true);
        return;
      }
      input.modal.setCreateEntryDraftErrorHint(decision.entryDraftErrorHint);
      input.modal.resetNewTaskForm();
      input.modal.applyCreateModalPrefill();
      input.modal.setCreateModalOpen(true);
    },
    [input],
  );

  const startFreshDraft = useCallback(async () => {
    input.modal.resetNewTaskForm();
    input.modal.applyCreateModalPrefill();
    input.modal.setDraftPickerOpen(false);
    input.modal.setCreateModalOpen(true);
  }, [input]);

  const resumeDraftByID = useCallback(
    async (id: string) => {
      input.modal.createModalPrefillRef.current = null;
      input.modal.setCreateModalAssignmentLocked(false);
      input.form.requestedResumeRef.current = id;
      const draft = await input.mutations.resumeDraftMutation.mutateAsync(id);
      // I4 — resume last-wins: ignore stale getTaskDraft responses.
      if (input.form.requestedResumeRef.current !== id) {
        return;
      }
      applyResumedDraftToForm({
        draft,
        settings: input.queryClient.getQueryData<AppSettings>(settingsQueryKeys.app()),
        setNewTaskRunner: input.form.setNewTaskRunner,
        setNewTaskCursorModel: input.form.setNewTaskCursorModel,
        setNewSchedule: input.form.setNewSchedule,
        setNewAutonomyEnabled: input.form.setNewAutonomyEnabled,
        setNewDraftID: input.form.setNewDraftID,
        setNewTitle: input.form.setNewTitle,
        setNewPrompt: input.form.setNewPrompt,
        setNewPriority: input.form.setNewPriority,
        setNewChecklistItems: input.form.setNewChecklistItems,
        setLatestDraftEvaluation: input.form.setLatestDraftEvaluation,
        setNewProjectID: input.form.setNewProjectID,
        setNewProjectContextItemIDs: input.form.setNewProjectContextItemIDs,
        setNewAutomationSelections: input.form.setNewAutomationSelections,
        setDraftAutosaveBaseline: input.form.setDraftAutosaveBaseline,
        setDraftAutosaveBaselineID: input.form.setDraftAutosaveBaselineID,
      });
      input.modal.setDraftPickerOpen(false);
      input.modal.setCreateModalOpen(true);
    },
    [input],
  );

  const deleteDraftByID = useCallback(
    async (id: string) => {
      await input.mutations.deleteDraftMutation.mutateAsync(id);
    },
    [input.mutations.deleteDraftMutation],
  );

  const retryDraftList = useCallback(async () => {
    await input.draftsQuery.refetch();
  }, [input.draftsQuery]);

  const retryCreateEntryDraftLoad = useCallback(async () => {
    const refreshed = await input.draftsQuery.refetch();
    if (refreshed.isError) {
      input.modal.setCreateEntryDraftErrorHint(errorMessage(refreshed.error));
      return;
    }
    input.modal.setCreateEntryDraftErrorHint(null);
    const drafts = refreshed.data ?? [];
    if (drafts.length > 0) {
      input.modal.setCreateModalOpen(false);
      input.modal.setDraftPickerOpen(true);
    }
  }, [input.draftsQuery, input.modal]);

  return {
    openCreateModal,
    startFreshDraft,
    resumeDraftByID,
    deleteDraftByID,
    retryDraftList,
    retryCreateEntryDraftLoad,
  };
}
