export type ProjectStatus = "active" | "archived";

export const DEFAULT_PROJECT_ID = "00000000-0000-4000-8000-000000000001";

export type ProjectContextKind = string;
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

export type ProjectStepGateStatus =
  | "locked"
  | "active"
  | "pending_release"
  | "released";

export type ProjectStepCriterion = {
  id: string;
  text: string;
  done: boolean;
  sort_order: number;
};

export type ProjectGoalCriterion = ProjectStepCriterion;

export type ProjectGoal = {
  id: string;
  project_id: string;
  title: string;
  description: string;
  depends_on_goal_ids: string[];
  gate_status: ProjectStepGateStatus;
  gate_hold: boolean;
  pending_release_deadline?: string;
  criteria: ProjectGoalCriterion[];
  created_at: string;
  updated_at: string;
};

export type ProjectGoalsListResponse = {
  goals: ProjectGoal[];
};

export type ProjectStep = {
  id: string;
  project_id: string;
  /** Present on steps created after the goals layer shipped. */
  goal_id?: string;
  title: string;
  description: string;
  sort_order: number;
  gate_status: ProjectStepGateStatus;
  gate_hold: boolean;
  pending_release_deadline?: string;
  criteria: ProjectStepCriterion[];
  created_at: string;
  updated_at: string;
};

export type ProjectStepsListResponse = {
  steps: ProjectStep[];
};

export type ProjectContextListResponse = {
  items: ProjectContextItem[];
  edges: ProjectContextEdge[];
  limit: number;
};

export const PROJECT_STATUSES: ProjectStatus[] = ["active", "archived"];

export const PROJECT_CONTEXT_KIND_SUGGESTIONS: ProjectContextKind[] = [
  "note",
  "decision",
  "constraint",
];

export const PROJECT_CONTEXT_RELATIONS: ProjectContextRelation[] = [
  "supports",
  "blocks",
  "refines",
  "depends_on",
  "related",
];
