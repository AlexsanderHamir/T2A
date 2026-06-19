import { useCallback } from "react";
import type { ChecklistItemDraft } from "@/types";
import { nonEmptyChecklistCount, normalizeVerifyCommands } from "../../task-compose/checklistRequirement";
import { plainTextToInitialHtml } from "../../task-prompt";
import type { useTaskCreateFormState } from "./useTaskCreateFormState";

export function useTaskCreateChecklistActions(input: {
  form: ReturnType<typeof useTaskCreateFormState>;
}) {
  const applyTestScenario = useCallback(
    (scenario: import("../../test-scenarios").TestScenario) => {
      input.form.setNewTitle(scenario.title);
      input.form.setNewPrompt(plainTextToInitialHtml(scenario.prompt));
      input.form.setNewPriority(scenario.priority);
      input.form.setNewChecklistItems(
        scenario.criteria
          .map((item) => {
            const text = item.text.trim();
            if (!text) return null;
            const verify_commands = normalizeVerifyCommands(item.verify_commands ?? []);
            return {
              text,
              ...(verify_commands.length > 0 ? { verify_commands } : {}),
            };
          })
          .filter((item): item is ChecklistItemDraft => item !== null),
      );
    },
    [input.form],
  );

  const appendNewChecklistCriterion = useCallback((raw: ChecklistItemDraft | string) => {
    const item = typeof raw === "string" ? { text: raw } : raw;
    const text = item.text.trim();
    if (!text) return;
    input.form.setNewChecklistItems((prev) => {
      const next = [...prev, { text, verify_commands: item.verify_commands }];
      if (nonEmptyChecklistCount(next) >= 1) {
        input.form.setCreateFormError(null);
      }
      return next;
    });
  }, [input.form]);

  const removeNewChecklistRow = useCallback((index: number) => {
    input.form.setNewChecklistItems((prev) => prev.filter((_, rowIndex) => rowIndex !== index));
  }, [input.form]);

  const updateNewChecklistRow = useCallback((index: number, item: ChecklistItemDraft) => {
    const text = item.text.trim();
    if (!text) return;
    input.form.setNewChecklistItems((prev) =>
      prev.map((row, rowIndex) =>
        rowIndex === index ? { text, verify_commands: item.verify_commands } : row,
      ),
    );
  }, [input.form]);

  return {
    applyTestScenario,
    appendNewChecklistCriterion,
    removeNewChecklistRow,
    updateNewChecklistRow,
  };
}
