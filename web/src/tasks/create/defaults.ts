import type { AppSettings } from "@/api/settings";
import { TASK_DRAFTS } from "@/constants/tasks";
import { DEFAULT_NEW_TASK_STATUS, DEFAULT_PROJECT_ID, type Status } from "@/types";
import { draftAutosaveSignature } from "../task-drafts";

export function generateTaskDraftID(): string {
  return typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
    ? crypto.randomUUID()
    : `draft-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

export function defaultRunnerFromSettings(settings: AppSettings | undefined): string {
  return (settings?.runner ?? "cursor").trim() || "cursor";
}

export function defaultCursorModelFromSettings(settings: AppSettings | undefined): string {
  return settings?.cursor_model ?? "";
}

export function createSubmitStatusForAutonomy(autonomyEnabled: boolean): Status {
  return autonomyEnabled ? DEFAULT_NEW_TASK_STATUS : "on_hold";
}

export function buildFreshDraftAutosaveBaseline(
  settings: AppSettings | undefined,
  generatedID: string,
): string {
  return draftAutosaveSignature({
    id: generatedID,
    name: TASK_DRAFTS.untitledDraftName,
    title: "",
    prompt: "",
    priority: "",
    runner: defaultRunnerFromSettings(settings),
    cursorModel: defaultCursorModelFromSettings(settings),
    projectId: DEFAULT_PROJECT_ID,
    projectContextItemIds: [],
    checklistItems: [],
    latestEvaluation: null,
  });
}
