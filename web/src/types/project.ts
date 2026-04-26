export type ProjectStatus = "active" | "archived";

export const DEFAULT_PROJECT_ID = "00000000-0000-4000-8000-000000000001";

export type ProjectContextKind = "note" | "decision" | "constraint" | "handoff";
export type ProjectContextRelation =
  | "supports"
  | "blocks"
  | "refines"
  | "depends_on"
  | "related";

export type Project = {
  id: string;
  name: string;
  description: string;
  status: ProjectStatus;
  context_summary: string;
  created_at: string;
  updated_at: string;
};

export type ProjectContextItem = {
  id: string;
  project_id: string;
  kind: ProjectContextKind;
  title: string;
  body: string;
  source_task_id?: string;
  source_cycle_id?: string;
  created_by: "user" | "agent";
  pinned: boolean;
  created_at: string;
  updated_at: string;
};

export type ProjectContextEdge = {
  id: string;
  project_id: string;
  source_context_id: string;
  target_context_id: string;
  relation: ProjectContextRelation;
  strength: number;
  note: string;
  created_at: string;
  updated_at: string;
};

export type ProjectListResponse = {
  projects: Project[];
  limit: number;
};

export type ProjectContextListResponse = {
  items: ProjectContextItem[];
  edges: ProjectContextEdge[];
  limit: number;
};

export const PROJECT_STATUSES: ProjectStatus[] = ["active", "archived"];

export const PROJECT_CONTEXT_KINDS: ProjectContextKind[] = [
  "note",
  "decision",
  "constraint",
  "handoff",
];

export const PROJECT_CONTEXT_RELATIONS: ProjectContextRelation[] = [
  "supports",
  "blocks",
  "refines",
  "depends_on",
  "related",
];
