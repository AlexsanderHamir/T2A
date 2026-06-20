import {
  type Task,
  type TaskComposePayload,
  type TaskTemplateDetail,
  type TaskTemplateSummary,
} from "@/types";
import { parseChecklistVerifyCommand, parseTask } from "./parseTaskApiTasks";
import {
  isRecord,
  parseNonEmptyString,
  parsePriorityChoice,
  parseStatus,
  parseString,
} from "./parseTaskApiCore";

function parseComposeChecklistItem(
  value: unknown,
  path: string,
): TaskComposePayload["checklist_items"][number] {
  if (typeof value === "string") {
    return { text: parseString(value, path) };
  }
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${path} must be string or object`);
  }
  let verify_commands: TaskComposePayload["checklist_items"][number]["verify_commands"];
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

function parseDependsOnWire(value: unknown): TaskComposePayload["depends_on"] {
  if (value === undefined || value === null) return [];
  if (!Array.isArray(value)) {
    throw new Error("Invalid API response: depends_on must be array");
  }
  return value.map((edge, i) => {
    if (typeof edge === "string") {
      return { task_id: parseString(edge, `depends_on[${i}]`), satisfies: "done" as const };
    }
    if (!isRecord(edge)) {
      throw new Error(`Invalid API response: depends_on[${i}] must be object or string`);
    }
    return {
      task_id: parseString(edge.task_id, `depends_on[${i}].task_id`),
      satisfies: "done" as const,
    };
  });
}

export function parseTaskComposePayload(value: unknown): TaskComposePayload {
  if (!isRecord(value)) throw new Error("Invalid API response: payload must be object");
  const checklistRaw = value.checklist_items;
  if (!Array.isArray(checklistRaw)) {
    throw new Error("Invalid API response: payload.checklist_items must be array");
  }
  return {
    title: parseString(value.title, "payload.title"),
    initial_prompt: parseString(value.initial_prompt, "payload.initial_prompt"),
    status: parseStatus(value.status),
    priority: parsePriorityChoice(value.priority) as TaskComposePayload["priority"],
    checklist_items: checklistRaw.map((row, i) =>
      parseComposeChecklistItem(row, `payload.checklist_items[${i}]`),
    ),
    ...(typeof value.runner === "string"
      ? { runner: parseString(value.runner, "payload.runner") }
      : {}),
    ...(typeof value.cursor_model === "string"
      ? { cursor_model: parseString(value.cursor_model, "payload.cursor_model") }
      : {}),
    ...(typeof value.project_id === "string"
      ? { project_id: parseString(value.project_id, "payload.project_id") }
      : {}),
    ...(Array.isArray(value.project_context_item_ids)
      ? {
          project_context_item_ids: value.project_context_item_ids.map((id, i) =>
            parseString(id, `payload.project_context_item_ids[${i}]`),
          ),
        }
      : {}),
    ...(typeof value.pickup_not_before === "string"
      ? {
          pickup_not_before: parseString(
            value.pickup_not_before,
            "payload.pickup_not_before",
          ),
        }
      : {}),
    ...(Array.isArray(value.tags)
      ? {
          tags: value.tags.map((tag, i) => parseString(tag, `payload.tags[${i}]`)),
        }
      : {}),
    ...(typeof value.milestone === "string"
      ? { milestone: parseString(value.milestone, "payload.milestone") }
      : {}),
    depends_on: parseDependsOnWire(value.depends_on),
  };
}

export function parseTaskTemplateSummaryList(value: unknown): TaskTemplateSummary[] {
  if (!isRecord(value)) throw new Error("Invalid API response: template list must be object");
  const raw = value.templates;
  if (!Array.isArray(raw)) throw new Error("Invalid API response: templates must be array");
  return raw.map((item, i) => {
    if (!isRecord(item)) throw new Error(`Invalid API response: templates[${i}] must be object`);
    return {
      id: parseNonEmptyString(item.id, `templates[${i}].id`),
      name: parseString(item.name, `templates[${i}].name`),
      created_at: parseString(item.created_at, `templates[${i}].created_at`),
      updated_at: parseString(item.updated_at, `templates[${i}].updated_at`),
    };
  });
}

export function parseTaskTemplateDetail(value: unknown): TaskTemplateDetail {
  if (!isRecord(value)) throw new Error("Invalid API response: template detail must be object");
  return {
    id: parseNonEmptyString(value.id, "id"),
    name: parseString(value.name, "name"),
    created_at: parseString(value.created_at, "created_at"),
    updated_at: parseString(value.updated_at, "updated_at"),
    payload: parseTaskComposePayload(value.payload),
  };
}

export function parseTaskTemplateInstantiateResponse(value: unknown): {
  tasks: Task[];
  errors: { template_id: string; error: string }[];
} {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: instantiate response must be object");
  }
  const tasksRaw = value.tasks;
  const errorsRaw = value.errors;
  if (!Array.isArray(tasksRaw)) {
    throw new Error("Invalid API response: tasks must be array");
  }
  if (!Array.isArray(errorsRaw)) {
    throw new Error("Invalid API response: errors must be array");
  }
  return {
    tasks: tasksRaw.map((t) => parseTask(t)),
    errors: errorsRaw.map((row, i) => {
      if (!isRecord(row)) {
        throw new Error(`Invalid API response: errors[${i}] must be object`);
      }
      return {
        template_id: parseString(row.template_id, `errors[${i}].template_id`),
        error: parseString(row.error, `errors[${i}].error`),
      };
    }),
  };
}
