import {
  type DraftTaskEvaluation,
  type TaskDraftDetail,
  type TaskDraftPayload,
  type TaskDraftSummary,
} from "@/types";
import {
  isRecord,
  parseFiniteNumber,
  parseNonEmptyString,
  parsePriority,
  parsePriorityChoice,
  parseString,
  parseTaskType,
} from "./parseTaskApiCore";

/** Validates POST /tasks/evaluate JSON. */
export function parseDraftTaskEvaluation(value: unknown): DraftTaskEvaluation {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: draft evaluation must be an object");
  }
  const sectionsRaw = value.sections;
  if (!Array.isArray(sectionsRaw)) {
    throw new Error("Invalid API response: sections must be an array");
  }
  const sections = sectionsRaw.map((row, i) => {
    if (!isRecord(row)) {
      throw new Error(`Invalid API response: sections[${i}] must be an object`);
    }
    const suggestionsRaw = row.suggestions;
    if (!Array.isArray(suggestionsRaw)) {
      throw new Error(
        `Invalid API response: sections[${i}].suggestions must be an array`,
      );
    }
    return {
      key: parseNonEmptyString(row.key, `sections[${i}].key`),
      label: parseString(row.label, `sections[${i}].label`),
      score: parseFiniteNumber(row.score, `sections[${i}].score`),
      summary: parseString(row.summary, `sections[${i}].summary`),
      suggestions: suggestionsRaw.map((s, j) =>
        parseString(s, `sections[${i}].suggestions[${j}]`),
      ),
    };
  });
  const cohesionSuggestionsRaw = value.cohesion_suggestions;
  if (!Array.isArray(cohesionSuggestionsRaw)) {
    throw new Error(
      "Invalid API response: cohesion_suggestions must be an array",
    );
  }
  const createdAt = parseString(value.created_at, "created_at");
  if (Number.isNaN(Date.parse(createdAt))) {
    throw new Error("Invalid API response: created_at must be a parseable date");
  }
  return {
    evaluation_id: parseNonEmptyString(value.evaluation_id, "evaluation_id"),
    created_at: createdAt,
    overall_score: parseFiniteNumber(value.overall_score, "overall_score"),
    overall_summary: parseString(value.overall_summary, "overall_summary"),
    sections,
    cohesion_score: parseFiniteNumber(value.cohesion_score, "cohesion_score"),
    cohesion_summary: parseString(value.cohesion_summary, "cohesion_summary"),
    cohesion_suggestions: cohesionSuggestionsRaw.map((s, i) =>
      parseString(s, `cohesion_suggestions[${i}]`),
    ),
  };
}

function parseDraftPayload(value: unknown): TaskDraftPayload {
  if (!isRecord(value)) throw new Error("Invalid API response: payload must be object");
  const checklistRaw = value.checklist_items;
  if (!Array.isArray(checklistRaw)) {
    throw new Error("Invalid API response: payload.checklist_items must be array");
  }
  const subtasksRaw = value.pending_subtasks;
  if (!Array.isArray(subtasksRaw)) {
    throw new Error("Invalid API response: payload.pending_subtasks must be array");
  }
  return {
    title: parseString(value.title, "payload.title"),
    initial_prompt: parseString(value.initial_prompt, "payload.initial_prompt"),
    priority: parsePriorityChoice(value.priority),
    task_type: parseTaskType(value.task_type ?? "general"),
    parent_id: parseString(value.parent_id ?? "", "payload.parent_id"),
    checklist_inherit: value.checklist_inherit === true,
    checklist_items: checklistRaw.map((s, i) => parseString(s, `payload.checklist_items[${i}]`)),
    pending_subtasks: subtasksRaw.map((s, i) => {
      if (!isRecord(s)) throw new Error(`Invalid API response: payload.pending_subtasks[${i}] must be object`);
      const sChecklist = s.checklist_items;
      if (!Array.isArray(sChecklist)) throw new Error(`Invalid API response: payload.pending_subtasks[${i}].checklist_items must be array`);
      return {
        title: parseString(s.title, `payload.pending_subtasks[${i}].title`),
        initial_prompt: parseString(
          s.initial_prompt,
          `payload.pending_subtasks[${i}].initial_prompt`,
        ),
        priority: parsePriority(s.priority),
        task_type: parseTaskType(s.task_type ?? "general"),
        checklist_items: sChecklist.map((x, j) =>
          parseString(x, `payload.pending_subtasks[${i}].checklist_items[${j}]`),
        ),
        checklist_inherit: s.checklist_inherit === true,
      };
    }),
    ...(isRecord(value.latest_evaluation)
      ? {
          latest_evaluation: {
            overall_score: parseFiniteNumber(
              value.latest_evaluation.overall_score,
              "payload.latest_evaluation.overall_score",
            ),
            overall_summary: parseString(
              value.latest_evaluation.overall_summary,
              "payload.latest_evaluation.overall_summary",
            ),
            sections: Array.isArray(value.latest_evaluation.sections)
              ? value.latest_evaluation.sections
                  .filter((s): s is Record<string, unknown> => isRecord(s))
                  .map((s) => ({
                    key: parseString(s.key, "payload.latest_evaluation.sections[].key"),
                    score: parseFiniteNumber(
                      s.score,
                      "payload.latest_evaluation.sections[].score",
                    ),
                  }))
              : [],
          },
        }
      : {}),
    ...(isRecord(value.dmap_config)
      ? {
          dmap_config: {
            commit_limit: parseFiniteNumber(
              value.dmap_config.commit_limit,
              "payload.dmap_config.commit_limit",
            ),
            domain: parseString(value.dmap_config.domain, "payload.dmap_config.domain"),
            description: parseString(
              value.dmap_config.description,
              "payload.dmap_config.description",
            ),
          },
        }
      : {}),
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
