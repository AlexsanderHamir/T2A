import type {
  Project,
  ProjectContextItem,
  ProjectContextKind,
  ProjectContextListResponse,
  ProjectListResponse,
  ProjectStatus,
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
const PROJECT_CONTEXT_KINDS = ["note", "decision", "constraint", "handoff"] as const;

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
  if (
    typeof value !== "string" ||
    !(PROJECT_CONTEXT_KINDS as readonly string[]).includes(value)
  ) {
    throw new Error("Invalid API response: context kind is unknown");
  }
  return value as ProjectContextKind;
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
  return {
    items: raw.map(parseProjectContextItem),
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
