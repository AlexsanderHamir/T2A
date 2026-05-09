import type {
  Project,
  ProjectContextEdge,
  ProjectContextItem,
  ProjectContextKind,
  ProjectContextListResponse,
  ProjectContextRelation,
  ProjectGoal,
  ProjectGoalsListResponse,
  ProjectListResponse,
  ProjectStatus,
  ProjectStep,
  ProjectStepCriterion,
  ProjectStepGateStatus,
  ProjectStepsListResponse,
} from "@/types";
import { fetchWithTimeout, jsonHeaders, readError } from "./shared";
import {
  isRecord,
  parseActor,
  parseBooleanField,
  parseFiniteNumber,
  parseISO8601Required,
  parseNonEmptyString,
  parseOptionalNonEmptyId,
  parseString,
} from "./parseTaskApiCore";
import { assertListIntQuery, assertTaskPathId } from "./taskRequestBounds";

const PROJECT_STATUSES = ["active", "archived"] as const;

const PROJECT_STEP_GATE_STATUSES = [
  "locked",
  "active",
  "pending_release",
  "released",
] as const;

function parseProjectStepGateStatus(value: unknown): ProjectStepGateStatus {
  if (
    typeof value !== "string" ||
    !(PROJECT_STEP_GATE_STATUSES as readonly string[]).includes(value)
  ) {
    throw new Error("Invalid API response: project step gate_status is unknown");
  }
  return value as ProjectStepGateStatus;
}

function parseProjectStepCriterion(value: unknown, index: number): ProjectStepCriterion {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: criteria[${index}] must be an object`);
  }
  return {
    id: parseNonEmptyString(value.id, "id"),
    text: parseString(value.text, "text"),
    done: parseBooleanField(value.done, "done"),
    sort_order: parseFiniteNumber(value.sort_order, "sort_order"),
  };
}

export function parseProjectStep(value: unknown): ProjectStep {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: project step must be an object");
  }
  const deadlineRaw = value.pending_release_deadline;
  let pending_release_deadline: string | undefined;
  if (deadlineRaw !== undefined && deadlineRaw !== null) {
    pending_release_deadline = parseISO8601Required(
      deadlineRaw,
      "pending_release_deadline",
    );
  }
  const critRaw = value.criteria;
  let criteria: ProjectStepCriterion[];
  if (critRaw === undefined || critRaw === null) {
    criteria = [];
  } else if (!Array.isArray(critRaw)) {
    throw new Error("Invalid API response: criteria must be an array");
  } else {
    criteria = critRaw.map((row, i) => parseProjectStepCriterion(row, i));
  }
  const goalId = parseOptionalNonEmptyId(value.goal_id, "goal_id");
  return {
    id: parseNonEmptyString(value.id, "id"),
    project_id: parseNonEmptyString(value.project_id, "project_id"),
    ...(goalId !== undefined ? { goal_id: goalId } : {}),
    title: parseString(value.title, "title"),
    description: parseString(value.description, "description"),
    sort_order: parseFiniteNumber(value.sort_order, "sort_order"),
    gate_status: parseProjectStepGateStatus(value.gate_status),
    gate_hold: parseBooleanField(value.gate_hold, "gate_hold"),
    criteria,
    ...(pending_release_deadline !== undefined
      ? { pending_release_deadline }
      : {}),
    created_at: parseISO8601Required(value.created_at, "created_at"),
    updated_at: parseISO8601Required(value.updated_at, "updated_at"),
  };
}

export function parseProjectStepsListResponse(value: unknown): ProjectStepsListResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: project steps list must be an object");
  }
  const raw = value.steps;
  if (!Array.isArray(raw)) {
    throw new Error("Invalid API response: steps must be an array");
  }
  const steps: ProjectStep[] = raw.map((row, i) => {
    try {
      return parseProjectStep(row);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`Invalid API response: steps[${i}]: ${msg}`);
    }
  });
  return { steps };
}

export function parseProjectGoal(value: unknown): ProjectGoal {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: project goal must be an object");
  }
  const deadlineRaw = value.pending_release_deadline;
  let pending_release_deadline: string | undefined;
  if (deadlineRaw !== undefined && deadlineRaw !== null) {
    pending_release_deadline = parseISO8601Required(
      deadlineRaw,
      "pending_release_deadline",
    );
  }
  const critRaw = value.criteria;
  let criteria: ProjectGoal["criteria"];
  if (critRaw === undefined || critRaw === null) {
    criteria = [];
  } else if (!Array.isArray(critRaw)) {
    throw new Error("Invalid API response: criteria must be an array");
  } else {
    criteria = critRaw.map((row, i) => parseProjectStepCriterion(row, i));
  }
  const depRaw = value.depends_on_goal_ids;
  let depends_on_goal_ids: string[];
  if (depRaw === undefined || depRaw === null) {
    depends_on_goal_ids = [];
  } else if (!Array.isArray(depRaw)) {
    throw new Error("Invalid API response: depends_on_goal_ids must be an array");
  } else {
    depends_on_goal_ids = depRaw.map((x, i) => {
      if (typeof x !== "string") {
        throw new Error(`Invalid API response: depends_on_goal_ids[${i}] must be a string`);
      }
      return parseNonEmptyString(x, `depends_on_goal_ids[${i}]`);
    });
  }
  return {
    id: parseNonEmptyString(value.id, "id"),
    project_id: parseNonEmptyString(value.project_id, "project_id"),
    title: parseString(value.title, "title"),
    description: parseString(value.description, "description"),
    depends_on_goal_ids,
    gate_status: parseProjectStepGateStatus(value.gate_status),
    gate_hold: parseBooleanField(value.gate_hold, "gate_hold"),
    criteria,
    ...(pending_release_deadline !== undefined
      ? { pending_release_deadline }
      : {}),
    created_at: parseISO8601Required(value.created_at, "created_at"),
    updated_at: parseISO8601Required(value.updated_at, "updated_at"),
  };
}

export function parseProjectGoalsListResponse(value: unknown): ProjectGoalsListResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: project goals list must be an object");
  }
  const raw = value.goals;
  if (!Array.isArray(raw)) {
    throw new Error("Invalid API response: goals must be an array");
  }
  const goals: ProjectGoal[] = raw.map((row, i) => {
    try {
      return parseProjectGoal(row);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`Invalid API response: goals[${i}]: ${msg}`);
    }
  });
  return { goals };
}

const PROJECT_CONTEXT_RELATIONS = [
  "supports",
  "blocks",
  "refines",
  "depends_on",
  "related",
] as const;

function parseProjectStatus(value: unknown): ProjectStatus {
  if (
    typeof value !== "string" ||
    !(PROJECT_STATUSES as readonly string[]).includes(value)
  ) {
    throw new Error("Invalid API response: project status must be active or archived");
  }
  return value as ProjectStatus;
}

function parseProjectContextKind(value: unknown): ProjectContextKind {
  if (typeof value !== "string" || !value.trim()) {
    throw new Error("Invalid API response: context kind must be a string");
  }
  return value;
}

function parseProjectContextRelation(value: unknown): ProjectContextRelation {
  if (
    typeof value !== "string" ||
    !(PROJECT_CONTEXT_RELATIONS as readonly string[]).includes(value)
  ) {
    throw new Error("Invalid API response: context relation is unknown");
  }
  return value as ProjectContextRelation;
}

export function parseProject(value: unknown): Project {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: project must be an object");
  }
  return {
    id: parseNonEmptyString(value.id, "id"),
    name: parseString(value.name, "name"),
    description: parseString(value.description, "description"),
    status: parseProjectStatus(value.status),
    context_summary: parseString(value.context_summary, "context_summary"),
    created_at: parseISO8601Required(value.created_at, "created_at"),
    updated_at: parseISO8601Required(value.updated_at, "updated_at"),
  };
}

export function parseProjectListResponse(value: unknown): ProjectListResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: project list must be an object");
  }
  const raw = value.projects;
  if (!Array.isArray(raw)) {
    throw new Error("Invalid API response: projects must be an array");
  }
  return {
    projects: raw.map(parseProject),
    limit: parseFiniteNumber(value.limit, "limit"),
  };
}

export function parseProjectContextItem(value: unknown): ProjectContextItem {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: project context item must be an object");
  }
  const item: ProjectContextItem = {
    id: parseNonEmptyString(value.id, "id"),
    project_id: parseNonEmptyString(value.project_id, "project_id"),
    kind: parseProjectContextKind(value.kind),
    title: parseString(value.title, "title"),
    body: parseString(value.body, "body"),
    created_by: parseActor(value.created_by),
    pinned: parseBooleanField(value.pinned, "pinned"),
    created_at: parseISO8601Required(value.created_at, "created_at"),
    updated_at: parseISO8601Required(value.updated_at, "updated_at"),
  };
  const sourceTaskID = parseOptionalNonEmptyId(value.source_task_id, "source_task_id");
  if (sourceTaskID !== undefined) item.source_task_id = sourceTaskID;
  const sourceCycleID = parseOptionalNonEmptyId(
    value.source_cycle_id,
    "source_cycle_id",
  );
  if (sourceCycleID !== undefined) item.source_cycle_id = sourceCycleID;
  return item;
}

export function parseProjectContextEdge(value: unknown): ProjectContextEdge {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: project context edge must be an object");
  }
  const strength = parseFiniteNumber(value.strength, "strength");
  if (!Number.isInteger(strength) || strength < 1 || strength > 5) {
    throw new Error("Invalid API response: context edge strength must be 1..5");
  }
  return {
    id: parseNonEmptyString(value.id, "id"),
    project_id: parseNonEmptyString(value.project_id, "project_id"),
    source_context_id: parseNonEmptyString(
      value.source_context_id,
      "source_context_id",
    ),
    target_context_id: parseNonEmptyString(
      value.target_context_id,
      "target_context_id",
    ),
    relation: parseProjectContextRelation(value.relation),
    strength,
    note: parseString(value.note, "note"),
    created_at: parseISO8601Required(value.created_at, "created_at"),
    updated_at: parseISO8601Required(value.updated_at, "updated_at"),
  };
}

export function parseProjectContextListResponse(
  value: unknown,
): ProjectContextListResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: project context list must be an object");
  }
  const raw = value.items;
  if (!Array.isArray(raw)) {
    throw new Error("Invalid API response: items must be an array");
  }
  const rawEdges = value.edges ?? [];
  if (!Array.isArray(rawEdges)) {
    throw new Error("Invalid API response: edges must be an array");
  }
  return {
    items: raw.map(parseProjectContextItem),
    edges: rawEdges.map(parseProjectContextEdge),
    limit: parseFiniteNumber(value.limit, "limit"),
  };
}

export async function listProjects(options?: {
  signal?: AbortSignal;
  limit?: number;
  includeArchived?: boolean;
}): Promise<ProjectListResponse> {
  const q = new URLSearchParams({
    limit:
      options?.limit === undefined
        ? "50"
        : assertListIntQuery("limit", options.limit, 0, 100),
  });
  if (options?.includeArchived) q.set("include_archived", "true");
  const res = await fetchWithTimeout(`/projects?${q}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw new Error(await readError(res));
  return parseProjectListResponse((await res.json()) as unknown);
}

export async function getProject(
  id: string,
  options?: { signal?: AbortSignal },
): Promise<Project> {
  const projectID = assertTaskPathId(id, "project id");
  const res = await fetchWithTimeout(`/projects/${encodeURIComponent(projectID)}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw new Error(await readError(res));
  return parseProject((await res.json()) as unknown);
}

export async function createProject(input: {
  name: string;
  id?: string;
  description?: string;
  context_summary?: string;
}): Promise<Project> {
  const res = await fetchWithTimeout("/projects", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
  if (!res.ok) throw new Error(await readError(res));
  return parseProject((await res.json()) as unknown);
}

export async function patchProject(
  id: string,
  input: {
    name?: string;
    description?: string;
    status?: ProjectStatus;
    context_summary?: string;
  },
): Promise<Project> {
  const projectID = assertTaskPathId(id, "project id");
  const res = await fetchWithTimeout(`/projects/${encodeURIComponent(projectID)}`, {
    method: "PATCH",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
  if (!res.ok) throw new Error(await readError(res));
  return parseProject((await res.json()) as unknown);
}

export async function deleteProject(id: string): Promise<void> {
  const projectID = assertTaskPathId(id, "project id");
  const res = await fetchWithTimeout(`/projects/${encodeURIComponent(projectID)}`, {
    method: "DELETE",
    headers: { Accept: "application/json" },
  });
  if (!res.ok) throw new Error(await readError(res));
}

export async function listProjectContext(
  projectId: string,
  options?: { signal?: AbortSignal; limit?: number; pinnedOnly?: boolean },
): Promise<ProjectContextListResponse> {
  const projectID = assertTaskPathId(projectId, "project id");
  const q = new URLSearchParams({
    limit:
      options?.limit === undefined
        ? "50"
        : assertListIntQuery("limit", options.limit, 0, 100),
  });
  if (options?.pinnedOnly) q.set("pinned_only", "true");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/context?${q}`,
    {
      headers: { Accept: "application/json" },
      signal: options?.signal,
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  return parseProjectContextListResponse((await res.json()) as unknown);
}

export async function createProjectContext(
  projectId: string,
  input: {
    id?: string;
    kind?: ProjectContextKind;
    title: string;
    body: string;
    source_task_id?: string;
    source_cycle_id?: string;
    pinned?: boolean;
  },
): Promise<ProjectContextItem> {
  const projectID = assertTaskPathId(projectId, "project id");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/context`,
    {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify(input),
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  return parseProjectContextItem((await res.json()) as unknown);
}

export async function createProjectContextEdge(
  projectId: string,
  input: {
    id?: string;
    source_context_id: string;
    target_context_id: string;
    relation?: ProjectContextRelation;
    strength?: number;
    note?: string;
  },
): Promise<ProjectContextEdge> {
  const projectID = assertTaskPathId(projectId, "project id");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/context/edges`,
    {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify(input),
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  return parseProjectContextEdge((await res.json()) as unknown);
}

export async function patchProjectContextEdge(
  projectId: string,
  edgeId: string,
  input: {
    relation?: ProjectContextRelation;
    strength?: number;
    note?: string;
  },
): Promise<ProjectContextEdge> {
  const projectID = assertTaskPathId(projectId, "project id");
  const edgeID = assertTaskPathId(edgeId, "context edge id");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/context/edges/${encodeURIComponent(edgeID)}`,
    {
      method: "PATCH",
      headers: jsonHeaders,
      body: JSON.stringify(input),
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  return parseProjectContextEdge((await res.json()) as unknown);
}

export async function deleteProjectContextEdge(
  projectId: string,
  edgeId: string,
): Promise<void> {
  const projectID = assertTaskPathId(projectId, "project id");
  const edgeID = assertTaskPathId(edgeId, "context edge id");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/context/edges/${encodeURIComponent(edgeID)}`,
    {
      method: "DELETE",
      headers: { Accept: "application/json" },
    },
  );
  if (!res.ok) throw new Error(await readError(res));
}

export async function patchProjectContext(
  projectId: string,
  contextId: string,
  input: {
    kind?: ProjectContextKind;
    title?: string;
    body?: string;
    pinned?: boolean;
  },
): Promise<ProjectContextItem> {
  const projectID = assertTaskPathId(projectId, "project id");
  const itemID = assertTaskPathId(contextId, "context id");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/context/${encodeURIComponent(itemID)}`,
    {
      method: "PATCH",
      headers: jsonHeaders,
      body: JSON.stringify(input),
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  return parseProjectContextItem((await res.json()) as unknown);
}

export async function deleteProjectContext(
  projectId: string,
  contextId: string,
): Promise<void> {
  const projectID = assertTaskPathId(projectId, "project id");
  const itemID = assertTaskPathId(contextId, "context id");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/context/${encodeURIComponent(itemID)}`,
    {
      method: "DELETE",
      headers: { Accept: "application/json" },
    },
  );
  if (!res.ok) throw new Error(await readError(res));
}

export async function listProjectSteps(
  projectId: string,
  opts?: { signal?: AbortSignal; goalId?: string },
): Promise<ProjectStepsListResponse> {
  const projectID = assertTaskPathId(projectId, "project id");
  const gid = (opts?.goalId ?? "").trim();
  const q = gid !== "" ? `?goal_id=${encodeURIComponent(gid)}` : "";
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/steps${q}`,
    { method: "GET", headers: { Accept: "application/json" }, signal: opts?.signal },
  );
  if (!res.ok) throw new Error(await readError(res));
  return parseProjectStepsListResponse((await res.json()) as unknown);
}

export async function listProjectGoals(
  projectId: string,
  opts?: { signal?: AbortSignal },
): Promise<ProjectGoalsListResponse> {
  const projectID = assertTaskPathId(projectId, "project id");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/goals`,
    { method: "GET", headers: { Accept: "application/json" }, signal: opts?.signal },
  );
  if (!res.ok) throw new Error(await readError(res));
  return parseProjectGoalsListResponse((await res.json()) as unknown);
}

export async function createProjectGoal(
  projectId: string,
  input: {
    id?: string;
    title: string;
    description?: string;
    depends_on_goal_ids?: string[];
    criteria?: { id?: string; text: string; done?: boolean; sort_order?: number }[];
  },
  opts?: { signal?: AbortSignal },
): Promise<ProjectGoal> {
  const projectID = assertTaskPathId(projectId, "project id");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/goals`,
    {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify(input),
      signal: opts?.signal,
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  return parseProjectGoal((await res.json()) as unknown);
}

export async function patchProjectGoal(
  projectId: string,
  goalId: string,
  input: {
    title?: string;
    description?: string;
    depends_on_goal_ids?: string[];
    gate_action?: "release" | "hold" | "clear_hold";
    criteria?: { id: string; text: string; done: boolean; sort_order: number }[];
  },
  opts?: { signal?: AbortSignal },
): Promise<ProjectGoal> {
  const projectID = assertTaskPathId(projectId, "project id");
  const gid = assertTaskPathId(goalId, "goal id");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/goals/${encodeURIComponent(gid)}`,
    {
      method: "PATCH",
      headers: jsonHeaders,
      body: JSON.stringify(input),
      signal: opts?.signal,
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  return parseProjectGoal((await res.json()) as unknown);
}

export async function deleteProjectGoal(
  projectId: string,
  goalId: string,
  opts?: { signal?: AbortSignal },
): Promise<void> {
  const projectID = assertTaskPathId(projectId, "project id");
  const gid = assertTaskPathId(goalId, "goal id");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/goals/${encodeURIComponent(gid)}`,
    {
      method: "DELETE",
      headers: { Accept: "application/json" },
      signal: opts?.signal,
    },
  );
  if (!res.ok) throw new Error(await readError(res));
}

export async function createProjectStep(
  projectId: string,
  input: {
    id?: string;
    goal_id: string;
    title: string;
    description?: string;
    sort_order?: number;
    criteria?: { id?: string; text: string; done?: boolean; sort_order?: number }[];
  },
  opts?: { signal?: AbortSignal },
): Promise<ProjectStep> {
  const projectID = assertTaskPathId(projectId, "project id");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/steps`,
    {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify(input),
      signal: opts?.signal,
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  return parseProjectStep((await res.json()) as unknown);
}

export async function patchProjectStep(
  projectId: string,
  stepId: string,
  input: {
    title?: string;
    description?: string;
    sort_order?: number;
    gate_action?: "release" | "hold" | "clear_hold";
    criteria?: { id: string; text: string; done: boolean; sort_order: number }[];
  },
  opts?: { signal?: AbortSignal },
): Promise<ProjectStep> {
  const projectID = assertTaskPathId(projectId, "project id");
  const sid = assertTaskPathId(stepId, "step id");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/steps/${encodeURIComponent(sid)}`,
    {
      method: "PATCH",
      headers: jsonHeaders,
      body: JSON.stringify(input),
      signal: opts?.signal,
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  return parseProjectStep((await res.json()) as unknown);
}

export async function deleteProjectStep(
  projectId: string,
  stepId: string,
  opts?: { signal?: AbortSignal },
): Promise<void> {
  const projectID = assertTaskPathId(projectId, "project id");
  const sid = assertTaskPathId(stepId, "step id");
  const res = await fetchWithTimeout(
    `/projects/${encodeURIComponent(projectID)}/steps/${encodeURIComponent(sid)}`,
    {
      method: "DELETE",
      headers: { Accept: "application/json" },
      signal: opts?.signal,
    },
  );
  if (!res.ok) throw new Error(await readError(res));
}
