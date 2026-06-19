import { TASK_TIMINGS } from "@/constants/tasks";
import { useCallback, useEffect, useMemo, type MutableRefObject } from "react";
import { buildDraftSavePayload, computeDraftAutosaveSignature } from "../draftPayload";
import type { DraftEvaluationSnapshot, TaskCreateFormFields } from "../types";
import type { useTaskCreateMutations } from "./useTaskCreateMutations";

const DRAFT_AUTOSAVE_DEBOUNCE_MS = TASK_TIMINGS.draftAutosaveDebounceMs;

export function useTaskCreateDraftAutosave(input: {
  formFields: TaskCreateFormFields;
  latestDraftEvaluation: DraftEvaluationSnapshot | null;
  draftAutosaveBaseline: string;
  draftAutosaveBaselineID: string;
  editingTaskId: string | null;
  createModalOpen: boolean;
  autosaveTimerRef: MutableRefObject<ReturnType<typeof setTimeout> | null>;
  saveDraftMutation: ReturnType<typeof useTaskCreateMutations>["saveDraftMutation"];
  lastDraftSavedAt: number | null;
}) {
  const currentDraftAutosaveSignature = useMemo(
    () => computeDraftAutosaveSignature(input.formFields, input.latestDraftEvaluation),
    [input.formFields, input.latestDraftEvaluation],
  );

  const buildDraftSaveInput = useCallback(
    () => buildDraftSavePayload(input.formFields, input.latestDraftEvaluation),
    [input.formFields, input.latestDraftEvaluation],
  );

  const saveDraftNow = useCallback(() => {
    // I1 — no autosave while editing an existing task.
    if (input.editingTaskId || !input.createModalOpen || !input.formFields.newDraftID) return;
    if (input.draftAutosaveBaselineID !== input.formFields.newDraftID) return;
    if (currentDraftAutosaveSignature === input.draftAutosaveBaseline) return;
    if (input.autosaveTimerRef.current) {
      clearTimeout(input.autosaveTimerRef.current);
      input.autosaveTimerRef.current = null;
    }
    input.saveDraftMutation.mutate({
      ...buildDraftSaveInput(),
      signature: currentDraftAutosaveSignature,
    });
  }, [
    buildDraftSaveInput,
    currentDraftAutosaveSignature,
    input,
  ]);

  useEffect(() => {
    // I1 — no autosave while editing an existing task.
    if (input.editingTaskId || !input.createModalOpen || !input.formFields.newDraftID) return;
    if (input.draftAutosaveBaselineID !== input.formFields.newDraftID) return;
    if (currentDraftAutosaveSignature === input.draftAutosaveBaseline) return;
    const signatureAtSchedule = currentDraftAutosaveSignature;
    input.autosaveTimerRef.current = setTimeout(() => {
      input.saveDraftMutation.mutate({
        ...buildDraftSaveInput(),
        signature: signatureAtSchedule,
      });
      input.autosaveTimerRef.current = null;
    }, DRAFT_AUTOSAVE_DEBOUNCE_MS);
    return () => {
      if (input.autosaveTimerRef.current) {
        clearTimeout(input.autosaveTimerRef.current);
        input.autosaveTimerRef.current = null;
      }
    };
  }, [
    buildDraftSaveInput,
    currentDraftAutosaveSignature,
    input,
  ]);

  const draftSaveLabel = useMemo(() => {
    if (input.editingTaskId || !input.createModalOpen) return null;
    if (input.saveDraftMutation.isPending) return "Saving draft…";
    if (input.saveDraftMutation.isError) {
      return "Draft autosave failed. You can still create the task.";
    }
    if (input.lastDraftSavedAt == null) return null;
    return "Draft saved";
  }, [
    input.createModalOpen,
    input.editingTaskId,
    input.lastDraftSavedAt,
    input.saveDraftMutation.isError,
    input.saveDraftMutation.isPending,
  ]);

  return {
    saveDraftNow,
    draftSaveLabel,
    draftSaveError: input.createModalOpen && input.saveDraftMutation.isError,
  };
}
