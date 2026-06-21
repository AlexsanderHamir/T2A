import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, type MutableRefObject } from "react";
import {
  createTask as apiCreate,
  deleteTaskDraft as apiDeleteDraft,
  deleteTaskTemplate as apiDeleteTemplate,
  getTaskDraft as apiGetDraft,
  getTaskTemplate as apiGetTemplate,
  instantiateTaskTemplates as apiInstantiateTemplates,
  patchTaskTemplate as apiPatchTemplate,
  saveTaskDraft as apiSaveDraft,
  saveTaskTemplate as apiSaveTemplate,
} from "@/api";
import type { PriorityChoice } from "@/types";
import { normalizeChecklistItems } from "../../task-compose/checklistRequirement";
import { taskQueryKeys } from "../../task-query";
import { buildComposePayloadFromForm } from "../composePayload";
import type { CreateTaskMutationInput, TaskCreateFormFields } from "../types";

export function useTaskCreateMutations(input: {
  queryClient: ReturnType<typeof useQueryClient>;
  newDraftIDRef: MutableRefObject<string>;
  newDraftID: string;
  closeCreateModal: () => void;
  setNewDraftID: (id: string) => void;
  setDraftAutosaveBaseline: (baseline: string) => void;
  setDraftAutosaveBaselineID: (id: string) => void;
  setLastDraftSavedAt: (timestamp: number | null) => void;
  createModalOpen: boolean;
  editingTemplateId: string | null;
}) {
  const createMutation = useMutation({
    mutationFn: async (mutationInput: CreateTaskMutationInput) => {
      const task = await apiCreate({
        title: mutationInput.title,
        initial_prompt: mutationInput.initial_prompt,
        status: mutationInput.status,
        priority: mutationInput.priority,
        draft_id: mutationInput.draft_id,
        runner: mutationInput.runner,
        cursor_model: mutationInput.cursor_model,
        ...(mutationInput.project_id ? { project_id: mutationInput.project_id } : {}),
        ...(mutationInput.project_context_item_ids.length > 0
          ? { project_context_item_ids: mutationInput.project_context_item_ids }
          : {}),
        ...(mutationInput.pickup_not_before !== null
          ? { pickup_not_before: mutationInput.pickup_not_before }
          : {}),
        ...(mutationInput.tags.length > 0 ? { tags: mutationInput.tags } : {}),
        ...(mutationInput.milestone ? { milestone: mutationInput.milestone } : {}),
        ...(mutationInput.depends_on.length > 0
          ? { depends_on: mutationInput.depends_on }
          : {}),
        checklist_items: normalizeChecklistItems(mutationInput.checklistItems),
      });
      return { task, input: mutationInput };
    },
    onSuccess: (_result, variables) => {
      // I3 — close modal only when create succeeded for the active draft.
      if (input.newDraftIDRef.current === variables.draft_id) {
        input.closeCreateModal();
      }
      void input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      void input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
      void input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.drafts() });
    },
  });

  const saveDraftMutation = useMutation({
    mutationFn: (mutationInput: {
      id: string;
      name: string;
      payload: {
        title: string;
        initial_prompt: string;
        priority: PriorityChoice;
        runner: string;
        cursor_model: string;
        project_id: string;
        project_context_item_ids: string[];
        checklist_items: import("@/types").TaskDraftChecklistItem[];
      };
      signature: string;
    }) => apiSaveDraft(mutationInput),
    onSuccess: async (saved, variables) => {
      // I2 — baseline stamp only when save response matches active draft ref.
      if (input.newDraftIDRef.current !== saved.id) {
        await input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.drafts() });
        return;
      }
      if (saved.id !== input.newDraftID) {
        input.setNewDraftID(saved.id);
      }
      input.setDraftAutosaveBaseline(variables.signature);
      input.setDraftAutosaveBaselineID(saved.id);
      input.setLastDraftSavedAt(Date.now());
      await input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.drafts() });
    },
  });

  const deleteDraftMutation = useMutation({
    mutationFn: (id: string) => apiDeleteDraft(id),
    onSuccess: async () => {
      await input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.drafts() });
    },
  });

  const resumeDraftMutation = useMutation({
    mutationFn: (id: string) => apiGetDraft(id),
  });

  const saveTemplateMutation = useMutation({
    mutationFn: (mutationInput: {
      id?: string;
      name: string;
      fields: TaskCreateFormFields;
    }) =>
      apiSaveTemplate({
        ...(mutationInput.id ? { id: mutationInput.id } : {}),
        name: mutationInput.name,
        payload: buildComposePayloadFromForm(mutationInput.fields),
      }),
    onSuccess: async () => {
      input.closeCreateModal();
      await input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.templates() });
    },
  });

  const patchTemplateMutation = useMutation({
    mutationFn: (mutationInput: { id: string; fields: TaskCreateFormFields; name: string }) =>
      apiPatchTemplate(mutationInput.id, {
        name: mutationInput.name,
        payload: buildComposePayloadFromForm(mutationInput.fields),
      }),
    onSuccess: async (_result, variables) => {
      if (input.editingTemplateId === variables.id) {
        input.closeCreateModal();
      }
      await input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.templates() });
    },
  });

  const loadTemplateMutation = useMutation({
    mutationFn: (id: string) => apiGetTemplate(id),
  });

  const deleteTemplateMutation = useMutation({
    mutationFn: (id: string) => apiDeleteTemplate(id),
    onSuccess: async () => {
      await input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.templates() });
    },
  });

  const instantiateTemplatesMutation = useMutation({
    mutationFn: (items: import("@/api").TaskTemplateInstantiateItem[]) =>
      apiInstantiateTemplates(items),
    onSuccess: (result) => {
      if (result.tasks.length > 0) {
        void input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
        void input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
      }
    },
  });

  useEffect(() => {
    if (!input.createModalOpen && !saveDraftMutation.isIdle) {
      saveDraftMutation.reset();
    }
  }, [input.createModalOpen, saveDraftMutation]);

  useEffect(() => {
    if (!input.createModalOpen && !createMutation.isIdle) {
      createMutation.reset();
    }
  }, [input.createModalOpen, createMutation]);

  useEffect(() => {
    if (!input.createModalOpen && !saveTemplateMutation.isIdle) {
      saveTemplateMutation.reset();
    }
  }, [input.createModalOpen, saveTemplateMutation]);

  useEffect(() => {
    if (!input.createModalOpen && !patchTemplateMutation.isIdle) {
      patchTemplateMutation.reset();
    }
  }, [input.createModalOpen, patchTemplateMutation]);

  return {
    createMutation,
    saveDraftMutation,
    deleteDraftMutation,
    resumeDraftMutation,
    saveTemplateMutation,
    patchTemplateMutation,
    loadTemplateMutation,
    deleteTemplateMutation,
    instantiateTemplatesMutation,
  };
}
