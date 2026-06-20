import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, type MutableRefObject } from "react";
import {
  createTask as apiCreate,
  deleteTaskDraft as apiDeleteDraft,
  evaluateDraftTask as apiEvaluateDraft,
  getTaskDraft as apiGetDraft,
  saveTaskDraft as apiSaveDraft,
} from "@/api";
import type { ChecklistItemDraft, Priority, PriorityChoice } from "@/types";
import { normalizeChecklistItems } from "../../task-compose/checklistRequirement";
import { taskQueryKeys } from "../../task-query";
import type { CreateTaskMutationInput, DraftEvaluationSnapshot } from "../types";

export function useTaskCreateMutations(input: {
  queryClient: ReturnType<typeof useQueryClient>;
  newDraftIDRef: MutableRefObject<string>;
  newDraftID: string;
  closeCreateModal: () => void;
  setNewDraftID: (id: string) => void;
  setLatestDraftEvaluation: (evaluation: DraftEvaluationSnapshot | null) => void;
  setDraftAutosaveBaseline: (baseline: string) => void;
  setDraftAutosaveBaselineID: (id: string) => void;
  setLastDraftSavedAt: (timestamp: number | null) => void;
  createModalOpen: boolean;
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

  const evaluateDraftMutation = useMutation({
    mutationFn: async (mutationInput: {
      id: string;
      title: string;
      initial_prompt: string;
      status: import("@/types").Status;
      priority: Priority;
      checklistItems: ChecklistItemDraft[];
    }) => {
      return apiEvaluateDraft({
        id: mutationInput.id,
        title: mutationInput.title,
        initial_prompt: mutationInput.initial_prompt,
        status: mutationInput.status,
        priority: mutationInput.priority,
        checklist_items: normalizeChecklistItems(mutationInput.checklistItems),
      });
    },
    onSuccess: (evaluation, variables) => {
      // I5 — evaluation snapshot applies only to the active draft.
      if (input.newDraftIDRef.current !== variables.id) return;
      input.setLatestDraftEvaluation({
        overallScore: evaluation.overall_score,
        overallSummary: evaluation.overall_summary,
        sections: evaluation.sections.map((section) => ({
          key: section.key,
          score: section.score,
        })),
      });
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
        latest_evaluation?: {
          overall_score: number;
          overall_summary: string;
          sections: Array<{ key: string; score: number }>;
        };
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

  useEffect(() => {
    if (!input.createModalOpen && !saveDraftMutation.isIdle) {
      saveDraftMutation.reset();
    }
  }, [input.createModalOpen, saveDraftMutation]);

  useEffect(() => {
    if (!input.createModalOpen) {
      if (!createMutation.isIdle) createMutation.reset();
      if (!evaluateDraftMutation.isIdle) evaluateDraftMutation.reset();
    }
  }, [input.createModalOpen, createMutation, evaluateDraftMutation]);

  return {
    createMutation,
    evaluateDraftMutation,
    saveDraftMutation,
    deleteDraftMutation,
    resumeDraftMutation,
  };
}
