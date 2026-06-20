import {
  type TaskDraftChecklistItem,
  type TaskDraftDetail,
  type TaskDraftPayload,
  type TaskDraftSummary,
} from "@/types";
import { parseChecklistVerifyCommand } from "./parseTaskApiTasks";
import {
  isRecord,
  parseNonEmptyString,
  parsePriorityChoice,
  parseString,
} from "./parseTaskApiCore";

function parseDraftChecklistItem(
  value: unknown,
  path: string,
): TaskDraftChecklistItem {
  if (typeof value === "string") {
    return { text: parseString(value, path) };
  }
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${path} must be string or object`);
  }
  let verify_commands: TaskDraftChecklistItem["verify_commands"];
  if (value.verify_commands !== undefined && value.verify_commands !== null) {
    if (!Array.isArray(value.verify_commands)) {
      throw new Error(`Invalid API response: ${path}.verify_commands must be an array`);
    }
    verify_commands = value.verify_commands.map((cmd, j) =>
      parseChecklistVerifyCommand(cmd, `${path}.verify_commands[${j}]`),
    );
  }
  return {
    text: parseString(value.text, `${path}.text`),
    ...(verify_commands !== undefined && verify_commands.length > 0
      ? { verify_commands }
      : {}),
  };
}

function parseDraftPayload(value: unknown): TaskDraftPayload {
  if (!isRecord(value)) throw new Error("Invalid API response: payload must be object");
  const checklistRaw = value.checklist_items;
  if (!Array.isArray(checklistRaw)) {
    throw new Error("Invalid API response: payload.checklist_items must be array");
  }
  return {
    title: parseString(value.title, "payload.title"),
    initial_prompt: parseString(value.initial_prompt, "payload.initial_prompt"),
    priority: parsePriorityChoice(value.priority),
    checklist_items: checklistRaw.map((row, i) =>
      parseDraftChecklistItem(row, `payload.checklist_items[${i}]`),
    ),
    ...(typeof value.runner === "string"
      ? { runner: parseString(value.runner, "payload.runner") }
      : {}),
    ...(typeof value.cursor_model === "string"
      ? {
          cursor_model: parseString(
            value.cursor_model,
            "payload.cursor_model",
          ),
        }
      : {}),
    ...(typeof value.project_id === "string"
      ? {
          project_id: parseString(value.project_id, "payload.project_id"),
        }
      : {}),
    ...(Array.isArray(value.project_context_item_ids)
      ? {
          project_context_item_ids: value.project_context_item_ids.map((id, i) =>
            parseString(id, `payload.project_context_item_ids[${i}]`),
          ),
        }
      : {}),
  };
}

/** Validates GET /task-drafts list JSON (`drafts` array). */
export function parseTaskDraftSummaryList(value: unknown): TaskDraftSummary[] {
  if (!isRecord(value)) throw new Error("Invalid API response: draft list must be object");
  const raw = value.drafts;
  if (!Array.isArray(raw)) throw new Error("Invalid API response: drafts must be array");
  return raw.map((item, i) => {
    if (!isRecord(item)) throw new Error(`Invalid API response: drafts[${i}] must be object`);
    const created = parseString(item.created_at, `drafts[${i}].created_at`);
    const updated = parseString(item.updated_at, `drafts[${i}].updated_at`);
    return {
      id: parseNonEmptyString(item.id, `drafts[${i}].id`),
      name: parseString(item.name, `drafts[${i}].name`),
      created_at: created,
      updated_at: updated,
    };
  });
}

/** Validates GET /task-drafts/{id} JSON. */
export function parseTaskDraftDetail(value: unknown): TaskDraftDetail {
  if (!isRecord(value)) throw new Error("Invalid API response: draft detail must be object");
  return {
    id: parseNonEmptyString(value.id, "id"),
    name: parseString(value.name, "name"),
    created_at: parseString(value.created_at, "created_at"),
    updated_at: parseString(value.updated_at, "updated_at"),
    payload: parseDraftPayload(value.payload),
  };
}
