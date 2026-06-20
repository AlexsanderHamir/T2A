import { DEFAULT_PROJECT_ID } from "@/types";

export const DEMO_SECOND_PROJECT_ID = "22222222-2222-4222-8222-222222222222";
export const DEMO_THIRD_PROJECT_ID = "33333333-3333-4333-8333-333333333333";

const ISO = "2026-03-10T12:00:00Z";

const DEMO_PROJECT_IDS = new Set([
  DEFAULT_PROJECT_ID,
  DEMO_SECOND_PROJECT_ID,
  DEMO_THIRD_PROJECT_ID,
]);

export function isDemoProjectId(id: string): boolean {
  return DEMO_PROJECT_IDS.has(id);
}

function task(
  id: string,
  title: string,
  status: string,
  priority: string,
  opt: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    id,
    title,
    initial_prompt: `Operator context for “${title}”. Includes acceptance notes and links.`,
    status,
    priority,
    runner: "cursor",
    cursor_model: "",
    checklist_inherit: true,
    tags: [],
    depends_on: [],
    ...opt,
  };
}

const M_JWT = "JWT rollout";
const M_SESS = "Session hardening";
const M_DISC = "Discovery";
const M_TEST = "Load testing";
const M_REL = "Release";

const C1 = "c1111111-1111-4111-8111-111111111111";
const C2 = "c2222222-2222-4222-8222-222222222222";
const C3 = "c3333333-3333-4333-8333-333333333333";
const C4 = "c4444444-4444-4444-8444-444444444444";
const C5 = "c5555555-5555-4555-8555-555555555555";
const C6 = "c6666666-6666-4666-8666-666666666666";

const E1 = "e1111111-1111-4111-8111-111111111111";

const ALL_TASK_IDS: string[] = [];

function reg(id: string) {
  ALL_TASK_IDS.push(id);
  return id;
}

const ROOT_TASKS: Record<string, unknown>[] = [
  task(reg("f0000001-0000-4000-8000-000000000001"), "Auth refactor rollout", "running", "high", {
    project_id: DEFAULT_PROJECT_ID,
    milestone: M_JWT,
    tags: ["auth"],
  }),
  task(reg("f0000002-0000-4000-8000-000000000002"), "Session invalidation sweep", "ready", "medium", {
    project_id: DEFAULT_PROJECT_ID,
    milestone: M_SESS,
    tags: ["auth"],
  }),
  task(reg("f0000003-0000-4000-8000-000000000003"), "OAuth consent copy review", "blocked", "low", {
    project_id: DEFAULT_PROJECT_ID,
    milestone: M_DISC,
  }),
  task(reg("f0000004-0000-4000-8000-000000000004"), "Load-test harness for login", "review", "critical", {
    project_id: DEFAULT_PROJECT_ID,
    milestone: M_TEST,
  }),
  task(reg("f0000005-0000-4000-8000-000000000005"), "Release checklist: AuthV2", "done", "medium", {
    project_id: DEFAULT_PROJECT_ID,
    milestone: M_REL,
  }),
  task(reg("f0000006-0000-4000-8000-000000000006"), "Backfill audit logs", "ready", "medium", {
    project_id: DEFAULT_PROJECT_ID,
  }),
  task(reg("f0000007-0000-4000-8000-000000000007"), "Customer migration dry run", "failed", "high", {
    project_id: DEFAULT_PROJECT_ID,
    milestone: M_TEST,
  }),
  task(reg("f0000008-0000-4000-8000-000000000008"), "Billing webhook resilience", "running", "critical", {
    project_id: DEMO_SECOND_PROJECT_ID,
  }),
  task(reg("f0000009-0000-4000-8000-000000000009"), "Usage dashboard tiles", "ready", "medium", {
    project_id: DEMO_SECOND_PROJECT_ID,
  }),
  task(reg("f000000a-0000-4000-8000-00000000000a"), "Unassigned triage: docs site", "ready", "low", {}),
  task(
    reg("f000000b-0000-4000-8000-00000000000b"),
    "Parent: onboarding epic",
    "running",
    "medium",
    {
      project_id: DEFAULT_PROJECT_ID,
      milestone: M_DISC,
      children: [
        task(reg("f000000c-0000-4000-8000-00000000000c"), "Child: empty state illustrations", "done", "low", {
          parent_id: "f000000b-0000-4000-8000-00000000000b",
          project_id: DEFAULT_PROJECT_ID,
        }),
        task(reg("f000000d-0000-4000-8000-00000000000d"), "Child: analytics beacon", "ready", "medium", {
          parent_id: "f000000b-0000-4000-8000-00000000000b",
          project_id: DEFAULT_PROJECT_ID,
        }),
      ],
    },
  ),
];

const EXTRA_BACKLOG_IDS = [
  "fafaf001-fafa-4afa-bafa-000000000001",
  "fafaf002-fafa-4afa-bafa-000000000002",
  "fafaf003-fafa-4afa-bafa-000000000003",
  "fafaf004-fafa-4afa-bafa-000000000004",
  "fafaf005-fafa-4afa-bafa-000000000005",
  "fafaf006-fafa-4afa-bafa-000000000006",
  "fafaf007-fafa-4afa-bafa-000000000007",
  "fafaf008-fafa-4afa-bafa-000000000008",
  "fafaf009-fafa-4afa-bafa-000000000009",
  "fafaf00a-fafa-4afa-bafa-00000000000a",
];
EXTRA_BACKLOG_IDS.forEach((id, i) => {
  ROOT_TASKS.push(
    task(reg(id), `Synthetic backlog item ${i + 1}`, i % 5 === 0 ? "done" : i % 4 === 0 ? "blocked" : "ready", "medium", {
      project_id:
        i % 3 === 0 ? DEFAULT_PROJECT_ID : i % 3 === 1 ? DEMO_SECOND_PROJECT_ID : undefined,
      milestone: i % 2 === 0 ? M_JWT : undefined,
    }),
  );
});

const DEMO_TASK_BY_ID = new Map<string, Record<string, unknown>>();
for (const row of ROOT_TASKS) {
  DEMO_TASK_BY_ID.set(row.id as string, row);
  const ch = row.children as Record<string, unknown>[] | undefined;
  if (ch) {
    for (const c of ch) {
      DEMO_TASK_BY_ID.set(c.id as string, c);
    }
  }
}

export function demoProjectsListWire(): unknown {
  return {
    projects: [
      {
        id: DEFAULT_PROJECT_ID,
        name: "AuthV2",
        description: "JWT + session hardening across services.",
        status: "active",
        context_summary: "Primary operator sandbox project.",
        created_at: ISO,
        updated_at: ISO,
      },
      {
        id: DEMO_SECOND_PROJECT_ID,
        name: "Billing insights",
        description: "Usage metering, exports, and anomaly detection.",
        status: "active",
        context_summary: "Cross-team billing context.",
        created_at: ISO,
        updated_at: ISO,
      },
      {
        id: DEMO_THIRD_PROJECT_ID,
        name: "Archived pilot",
        description: "Superseded experiment — kept for layout regression.",
        status: "archived",
        context_summary: "",
        created_at: ISO,
        updated_at: ISO,
      },
    ],
    limit: 100,
  };
}

export function demoProjectWire(id: string): unknown | null {
  if (!isDemoProjectId(id)) return null;
  const row = (demoProjectsListWire() as { projects: { id: string }[] }).projects.find((p) => p.id === id);
  return row ?? null;
}

export function demoContextWire(projectId: string): unknown {
  if (projectId !== DEFAULT_PROJECT_ID) {
    return { items: [], edges: [], limit: 100 };
  }
  return {
    items: [
      {
        id: C1,
        project_id: DEFAULT_PROJECT_ID,
        kind: "decision",
        title: "JWT-first for partner APIs",
        body: "Partners accept bearer tokens only; cookies reserved for first-party.",
        created_by: "user",
        pinned: true,
        created_at: ISO,
        updated_at: ISO,
      },
      {
        id: C2,
        project_id: DEFAULT_PROJECT_ID,
        kind: "constraint",
        title: "No PII in logs",
        body: "Structured logs must redact email and phone by default.",
        created_by: "user",
        pinned: false,
        created_at: ISO,
        updated_at: ISO,
      },
      {
        id: C3,
        project_id: DEFAULT_PROJECT_ID,
        kind: "note",
        title: "Rotation cadence",
        body: "Signing keys rotate every 30 days; overlap window 72h.",
        created_by: "agent",
        pinned: false,
        created_at: ISO,
        updated_at: ISO,
      },
      {
        id: C4,
        project_id: DEFAULT_PROJECT_ID,
        kind: "decision",
        title: "Session fixation mitigation",
        body: "Regenerate session id post-auth; SameSite=Lax default.",
        created_by: "user",
        pinned: false,
        created_at: ISO,
        updated_at: ISO,
      },
      {
        id: C5,
        project_id: DEFAULT_PROJECT_ID,
        kind: "constraint",
        title: "EU residency",
        body: "Auth metadata stores primary region EU.",
        created_by: "user",
        pinned: false,
        created_at: ISO,
        updated_at: ISO,
      },
      {
        id: C6,
        project_id: DEFAULT_PROJECT_ID,
        kind: "note",
        title: "Load test window",
        body: "Saturdays 02:00–06:00 UTC only.",
        created_by: "user",
        pinned: false,
        created_at: ISO,
        updated_at: ISO,
      },
    ],
    edges: [
      {
        id: E1,
        project_id: DEFAULT_PROJECT_ID,
        source_context_id: C1,
        target_context_id: C2,
        relation: "refines",
        strength: 4,
        note: "Decision narrows how constraint is applied in middleware.",
        created_at: ISO,
        updated_at: ISO,
      },
      {
        id: "e2222222-2222-4222-8222-222222222222",
        project_id: DEFAULT_PROJECT_ID,
        source_context_id: C2,
        target_context_id: C5,
        relation: "supports",
        strength: 3,
        note: "",
        created_at: ISO,
        updated_at: ISO,
      },
      {
        id: "e3333333-3333-4333-8333-333333333333",
        project_id: DEFAULT_PROJECT_ID,
        source_context_id: C3,
        target_context_id: C1,
        relation: "depends_on",
        strength: 2,
        note: "",
        created_at: ISO,
        updated_at: ISO,
      },
      {
        id: "e4444444-4444-4444-8444-444444444444",
        project_id: DEFAULT_PROJECT_ID,
        source_context_id: C4,
        target_context_id: C1,
        relation: "related",
        strength: 3,
        note: "",
        created_at: ISO,
        updated_at: ISO,
      },
    ],
    limit: 100,
  };
}

const cyclesPhasesEmpty = {
  cycles: { by_status: {}, by_triggered_by: {} },
  phases: {
    by_phase_status: {
      execute: {},
      verify: {},
    },
  },
  runner: {
    by_runner: {},
    by_model: {},
    by_runner_model: {},
    by_runner_model_resolved: {},
  },
  recent_failures: [] as unknown[],
};

export function demoTaskStatsWire(): unknown {
  const byStatus: Record<string, number> = {};
  for (const row of ROOT_TASKS) {
    const s = row.status as string;
    byStatus[s] = (byStatus[s] ?? 0) + 1;
  }
  const total = ROOT_TASKS.length;
  return {
    total,
    ready: byStatus.ready ?? 0,
    critical: byStatus.critical ?? 0,
    scheduled: 2,
    by_status: byStatus,
    by_priority: { low: 4, medium: total - 8, high: 4, critical: 4 },
    ...cyclesPhasesEmpty,
  };
}

export function demoTasksListWire(
  limit: number,
  offset: number,
  afterId: string | null | undefined,
): unknown {
  if (afterId) {
    return { tasks: [], limit, offset: 0, has_more: false };
  }
  const slice = ROOT_TASKS.slice(offset, offset + limit);
  return {
    tasks: slice,
    limit,
    offset,
    has_more: offset + slice.length < ROOT_TASKS.length,
  };
}

export function demoTaskWire(id: string): unknown | null {
  const row = DEMO_TASK_BY_ID.get(id);
  return row ? { ...row } : null;
}

export function demoTaskDraftsWire(): unknown {
  return {
    drafts: [
      {
        id: "d1111111-1111-4111-8111-111111111111",
        name: "Draft: incident retro",
        created_at: ISO,
        updated_at: ISO,
      },
      {
        id: "d2222222-2222-4222-8222-222222222222",
        name: "Draft: Q2 planning",
        created_at: ISO,
        updated_at: ISO,
      },
    ],
  };
}

export function demoTaskTemplatesWire(): unknown {
  return {
    templates: [
      {
        id: "t1111111-1111-4111-8111-111111111111",
        name: "Template: incident runbook",
        created_at: ISO,
        updated_at: ISO,
      },
    ],
  };
}

export function demoTaskEventsWire(taskId: string): unknown {
  return {
    task_id: taskId,
    events: [],
    approval_pending: false,
    has_more_newer: false,
    has_more_older: false,
    limit: 200,
  };
}

export function demoTaskCyclesListWire(taskId: string): unknown {
  return {
    task_id: taskId,
    cycles: [],
    limit: 50,
    has_more: false,
  };
}

export function demoTaskChecklistWire(): unknown {
  return { items: [] };
}

export function demoCycleFailuresWire(): unknown {
  return {
    total: 0,
    limit: 50,
    offset: 0,
    sort: "at_desc",
    reason_sort_truncated: false,
    failures: [],
  };
}

export function isDemoTaskId(id: string): boolean {
  return DEMO_TASK_BY_ID.has(id);
}

export function allRegisteredDemoTaskIds(): readonly string[] {
  return ALL_TASK_IDS;
}
