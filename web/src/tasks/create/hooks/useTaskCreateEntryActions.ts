import type { QueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import type { AppSettings } from "@/api/settings";
import { errorMessage } from "@/lib/errorMessage";
import { settingsQueryKeys } from "../../task-query";
import { hydrateFormFromComposePayload } from "../composePayload";
import { applyResumedDraftToForm } from "../draftPayload";
import { decideCreateEntry } from "../decideCreateEntry";
import type { ComposeOperation, ComposeTarget, TaskDraftsQuery } from "../types";
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
  const openComposeModal = useCallback(
    (opts?: {
      projectID?: string;
      lockProjectAssignment?: boolean;
      target?: ComposeTarget;
      operation?: ComposeOperation;
      skipDraftPicker?: boolean;
    }) => {
      input.modal.setCreateEntryDraftErrorHint(null);
      const projectID = opts?.projectID?.trim();
      input.modal.createModalPrefillRef.current = projectID
        ? {
            projectID,
            lockProjectAssignment: opts?.lockProjectAssignment === true,
          }
        : null;
      const target = opts?.target ?? "task";
      const operation = opts?.operation ?? "create";
      if (target === "template" || opts?.skipDraftPicker) {
        input.modal.resetNewTaskForm();
        input.modal.setComposeTarget(target);
        input.modal.setComposeOperation(operation);
        input.modal.applyCreateModalPrefill();
        input.modal.setCreateModalOpen(true);
        return;
      }
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

  const openCreateModal = useCallback(
    (prefill?: { projectID: string; lockProjectAssignment?: boolean }) => {
      openComposeModal({
        projectID: prefill?.projectID,
        lockProjectAssignment: prefill?.lockProjectAssignment,
        target: "task",
        operation: "create",
      });
    },
    [openComposeModal],
  );

  const openTemplateCreateModal = useCallback(() => {
    openComposeModal({ target: "template", operation: "create", skipDraftPicker: true });
  }, [openComposeModal]);

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
        setNewProjectID: input.form.setNewProjectID,
        setNewProjectContextItemIDs: input.form.setNewProjectContextItemIDs,
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

  const editTemplateByID = useCallback(
    async (id: string) => {
      input.modal.createModalPrefillRef.current = null;
      input.modal.setCreateModalAssignmentLocked(false);
      const template = await input.mutations.loadTemplateMutation.mutateAsync(id);
      const settings = input.queryClient.getQueryData<AppSettings>(settingsQueryKeys.app());
      const hydrated = hydrateFormFromComposePayload(template.payload, settings);
      input.modal.resetNewTaskForm();
      input.modal.setComposeTarget("template");
      input.modal.setComposeOperation("edit");
      input.modal.setEditingTemplateId(template.id);
      input.form.setNewTitle(hydrated.title);
      input.form.setNewPrompt(hydrated.prompt);
      input.form.setNewPriority(hydrated.priority);
      input.form.setNewTaskRunner(hydrated.runner);
      input.form.setNewTaskCursorModel(hydrated.cursorModel);
      input.form.setNewProjectID(hydrated.projectID);
      input.form.setNewProjectContextItemIDs(hydrated.projectContextItemIDs);
      input.form.setNewSchedule(hydrated.schedule);
      input.form.setNewAutonomyEnabled(hydrated.autonomyEnabled);
      input.form.setNewTagsCsv(hydrated.tagsCsv);
      input.form.setNewMilestone(hydrated.milestone);
      input.form.setNewDependsOn(hydrated.dependsOn);
      input.form.setNewChecklistItems(hydrated.checklistItems);
      input.modal.setCreateModalOpen(true);
    },
    [input],
  );

  const deleteTemplateByID = useCallback(
    async (id: string) => {
      await input.mutations.deleteTemplateMutation.mutateAsync(id);
    },
    [input.mutations.deleteTemplateMutation],
  );

  const instantiateTemplates = useCallback(
    async (items: import("@/api").TaskTemplateInstantiateItem[]) =>
      input.mutations.instantiateTemplatesMutation.mutateAsync(items),
    [input.mutations.instantiateTemplatesMutation],
  );

  return {
    openCreateModal,
    openComposeModal,
    openTemplateCreateModal,
    startFreshDraft,
    resumeDraftByID,
    deleteDraftByID,
    editTemplateByID,
    deleteTemplateByID,
    instantiateTemplates,
    retryDraftList,
    retryCreateEntryDraftLoad,
  };
}
