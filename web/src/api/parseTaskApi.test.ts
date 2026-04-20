import { describe, expect, it } from "vitest";
import {
  maxTaskParseDepth,
  parseTask,
  parseTaskCycle,
  parseTaskCycleDetail,
  parseTaskCyclePhase,
  parseTaskCyclesListResponse,
  parseTaskEventDetail,
  parseTaskEventsResponse,
  parseDraftTaskEvaluation,
  parseTaskDraftDetail,
  parseTaskListResponse,
  parseTaskStatsResponse,
} from "./parseTaskApi";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import { TASK_EVENT_TYPES } from "@/types";

const validTask = {
  id: "a1",
  title: "One",
  initial_prompt: "",
  status: "ready",
  priority: "medium",
  task_type: "general",
  checklist_inherit: false,
  ...TASK_TEST_DEFAULTS,
};

describe("parseTask", () => {
  it("accepts a well-formed task", () => {
    expect(parseTask(validTask)).toEqual(validTask);
  });

  it("defaults missing initial_prompt to empty string", () => {
    expect(
      parseTask({
        id: "a1",
        title: "One",
        status: "ready",
        priority: "medium",
      }),
    ).toEqual(validTask);
  });

  it("rejects invalid status", () => {
    expect(() =>
      parseTask({ ...validTask, status: "nope" }),
    ).toThrow(/known task status/);
  });

  it("rejects non-object", () => {
    expect(() => parseTask(null)).toThrow(/object/);
  });

  it("rejects children nested deeper than maxTaskParseDepth", () => {
    const deepTask = (n: number): Record<string, unknown> => {
      const base: Record<string, unknown> = {
        id: `id-${n}`,
        title: "t",
        initial_prompt: "",
        status: "ready",
        priority: "medium",
        task_type: "general",
        checklist_inherit: false,
        ...TASK_TEST_DEFAULTS,
      };
      if (n <= 0) {
        return base;
      }
      return { ...base, children: [deepTask(n - 1)] };
    };
    expect(() => parseTask(deepTask(maxTaskParseDepth + 1))).toThrow(/too deep/);
    expect(parseTask(deepTask(maxTaskParseDepth))).toBeDefined();
  });

  it("parses nested children and checklist_inherit", () => {
    expect(
      parseTask({
        id: "root",
        title: "R",
        initial_prompt: "",
        status: "ready",
        priority: "medium",
        task_type: "general",
        checklist_inherit: false,
        children: [
          {
            id: "c1",
            title: "C",
            initial_prompt: "",
            status: "running",
            priority: "low",
            task_type: "bug_fix",
            checklist_inherit: true,
            parent_id: "root",
            ...TASK_TEST_DEFAULTS,
          },
        ],
      }),
    ).toEqual({
      id: "root",
      title: "R",
      initial_prompt: "",
      status: "ready",
      priority: "medium",
      task_type: "general",
      checklist_inherit: false,
      ...TASK_TEST_DEFAULTS,
      children: [
        {
          id: "c1",
          title: "C",
          initial_prompt: "",
          status: "running",
          priority: "low",
          task_type: "bug_fix",
          checklist_inherit: true,
          parent_id: "root",
          ...TASK_TEST_DEFAULTS,
        },
      ],
    });
  });
});

describe("parseTaskListResponse", () => {
  it("parses list envelope", () => {
    expect(
      parseTaskListResponse({
        tasks: [validTask],
        limit: 200,
        offset: 0,
        has_more: false,
      }),
    ).toEqual({ tasks: [validTask], limit: 200, offset: 0, has_more: false });
  });

  it("defaults has_more when omitted", () => {
    expect(
      parseTaskListResponse({
        tasks: [validTask],
        limit: 50,
        offset: 0,
      }),
    ).toEqual({ tasks: [validTask], limit: 50, offset: 0, has_more: false });
  });

  it("parses has_more true", () => {
    expect(
      parseTaskListResponse({
        tasks: [validTask],
        limit: 2,
        offset: 0,
        has_more: true,
      }),
    ).toEqual({ tasks: [validTask], limit: 2, offset: 0, has_more: true });
  });

  it("rejects invalid has_more", () => {
    expect(() =>
      parseTaskListResponse({
        tasks: [validTask],
        limit: 1,
        offset: 0,
        has_more: "yes",
      }),
    ).toThrow(/has_more/);
  });

  it("rejects non-array tasks", () => {
    expect(() =>
      parseTaskListResponse({ tasks: {}, limit: 0, offset: 0 }),
    ).toThrow(/array/);
  });

  it("treats null tasks as empty array (legacy Go nil slice JSON)", () => {
    expect(
      parseTaskListResponse({
        tasks: null,
        limit: 50,
        offset: 0,
      }),
    ).toEqual({ tasks: [], limit: 50, offset: 0, has_more: false });
  });
});

describe("parseTaskStatsResponse", () => {
  // emptyExtras covers the cycle/phase/recent_failures blocks the
  // server always sends — most assertions in this suite focus on the
  // task-counter half of the envelope and reuse this stub.
  const emptyExtras = {
    cycles: { by_status: {}, by_triggered_by: {} },
    phases: {
      by_phase_status: {
        diagnose: {},
        execute: {},
        verify: {},
        persist: {},
      },
    },
    runner: {
      by_runner: {},
      by_model: {},
      by_runner_model: {},
    },
    recent_failures: [],
  };

  it("parses task stats envelope", () => {
    expect(
      parseTaskStatsResponse({
        total: 22,
        ready: 7,
        critical: 2,
        scheduled: 3,
        by_status: { ready: 7, running: 5 },
        by_priority: { critical: 2, high: 4 },
        by_scope: { parent: 10, subtask: 12 },
        ...emptyExtras,
      }),
    ).toEqual({
      total: 22,
      ready: 7,
      critical: 2,
      scheduled: 3,
      by_status: { ready: 7, running: 5 },
      by_priority: { critical: 2, high: 4 },
      by_scope: { parent: 10, subtask: 12 },
      ...emptyExtras,
    });
  });

  it("defaults scheduled to 0 when omitted (back-compat with pre-Stage-6 backends)", () => {
    const got = parseTaskStatsResponse({
      total: 1,
      ready: 1,
      critical: 0,
      // scheduled key intentionally absent — older backend
      by_status: { ready: 1 },
      by_priority: {},
      by_scope: { parent: 1, subtask: 0 },
      ...emptyExtras,
    });
    expect(got.scheduled).toBe(0);
  });

  it("rejects scheduled when present-but-non-numeric", () => {
    expect(() =>
      parseTaskStatsResponse({
        total: 1,
        ready: 1,
        critical: 0,
        scheduled: "3",
        by_status: { ready: 1 },
        by_priority: {},
        by_scope: { parent: 1, subtask: 0 },
        ...emptyExtras,
      }),
    ).toThrow(/scheduled/);
  });

  it("rejects invalid stats payload", () => {
    expect(() =>
      parseTaskStatsResponse({
        total: "22",
        ready: 7,
        critical: 2,
        by_status: {},
        by_priority: {},
        by_scope: { parent: 0, subtask: 0 },
        ...emptyExtras,
      }),
    ).toThrow(/total/);
  });

  it("rejects unknown status/priority keys in breakdowns", () => {
    expect(() =>
      parseTaskStatsResponse({
        total: 22,
        ready: 7,
        critical: 2,
        by_status: { nope: 1 },
        by_priority: {},
        by_scope: { parent: 10, subtask: 12 },
        ...emptyExtras,
      }),
    ).toThrow(/known status/);

    expect(() =>
      parseTaskStatsResponse({
        total: 22,
        ready: 7,
        critical: 2,
        by_status: {},
        by_priority: { urgent: 1 },
        by_scope: { parent: 10, subtask: 12 },
        ...emptyExtras,
      }),
    ).toThrow(/known priority/);
  });

  it("requires parent/subtask scope counts", () => {
    expect(() =>
      parseTaskStatsResponse({
        total: 22,
        ready: 7,
        critical: 2,
        by_status: { ready: 7 },
        by_priority: { critical: 2 },
        by_scope: { parent: 10 },
        ...emptyExtras,
      }),
    ).toThrow(/by_scope\.subtask/);
  });

  it("parses cycles aggregates and rejects unknown enums", () => {
    const got = parseTaskStatsResponse({
      total: 0,
      ready: 0,
      critical: 0,
      by_status: {},
      by_priority: {},
      by_scope: { parent: 0, subtask: 0 },
      ...emptyExtras,
      cycles: {
        by_status: { running: 1, succeeded: 4, failed: 2, aborted: 1 },
        by_triggered_by: { user: 3, agent: 5 },
      },
    });
    expect(got.cycles.by_status).toEqual({
      running: 1,
      succeeded: 4,
      failed: 2,
      aborted: 1,
    });
    expect(got.cycles.by_triggered_by).toEqual({ user: 3, agent: 5 });

    expect(() =>
      parseTaskStatsResponse({
        total: 0,
        ready: 0,
        critical: 0,
        by_status: {},
        by_priority: {},
        by_scope: { parent: 0, subtask: 0 },
        ...emptyExtras,
        cycles: {
          by_status: { weird: 1 },
          by_triggered_by: {},
        },
      }),
    ).toThrow(/cycles\.by_status/);
  });

  it("parses phases heatmap with all four phases always present", () => {
    const got = parseTaskStatsResponse({
      total: 0,
      ready: 0,
      critical: 0,
      by_status: {},
      by_priority: {},
      by_scope: { parent: 0, subtask: 0 },
      ...emptyExtras,
      phases: {
        by_phase_status: {
          // Server omits phases with no data; parser must still seed
          // every Phase enum value with `{}` so the heatmap renders.
          diagnose: { succeeded: 4 },
          execute: { failed: 2, succeeded: 1 },
          verify: {},
          persist: {},
        },
      },
    });
    expect(Object.keys(got.phases.by_phase_status).sort()).toEqual([
      "diagnose",
      "execute",
      "persist",
      "verify",
    ]);
    expect(got.phases.by_phase_status.execute).toEqual({
      failed: 2,
      succeeded: 1,
    });
  });

  it("parses recent_failures and rejects bad rows", () => {
    const got = parseTaskStatsResponse({
      total: 0,
      ready: 0,
      critical: 0,
      by_status: {},
      by_priority: {},
      by_scope: { parent: 0, subtask: 0 },
      ...emptyExtras,
      recent_failures: [
        {
          task_id: "t-1",
          event_seq: 7,
          at: "2026-04-19T12:00:00Z",
          cycle_id: "c-1",
          attempt_seq: 2,
          status: "failed",
          reason: "execute blew up",
        },
      ],
    });
    expect(got.recent_failures).toHaveLength(1);
    expect(got.recent_failures[0].cycle_id).toBe("c-1");

    expect(() =>
      parseTaskStatsResponse({
        total: 0,
        ready: 0,
        critical: 0,
        by_status: {},
        by_priority: {},
        by_scope: { parent: 0, subtask: 0 },
        ...emptyExtras,
        recent_failures: [
          {
            task_id: "t-1",
            event_seq: 7,
            at: "2026-04-19T12:00:00Z",
            cycle_id: "c-1",
            attempt_seq: 2,
            status: "succeeded",
            reason: "",
          },
        ],
      }),
    ).toThrow(/recent_failures\[0\]\.status/);
  });

  it("parses runner breakdown across by_runner, by_model, and by_runner_model", () => {
    const got = parseTaskStatsResponse({
      total: 0,
      ready: 0,
      critical: 0,
      by_status: {},
      by_priority: {},
      by_scope: { parent: 0, subtask: 0 },
      ...emptyExtras,
      runner: {
        by_runner: {
          "cursor-cli": {
            by_status: { succeeded: 2, failed: 1 },
            succeeded: 2,
            duration_p50_succeeded_seconds: 1.5,
            duration_p95_succeeded_seconds: 4.2,
          },
        },
        by_model: {
          "sonnet-4.5": {
            by_status: { succeeded: 1 },
            succeeded: 1,
            duration_p50_succeeded_seconds: 1,
            duration_p95_succeeded_seconds: 1,
          },
          "": {
            by_status: { failed: 1 },
            succeeded: 0,
            duration_p50_succeeded_seconds: 0,
            duration_p95_succeeded_seconds: 0,
          },
        },
        by_runner_model: {
          "cursor-cli|sonnet-4.5": {
            by_status: { succeeded: 1 },
            succeeded: 1,
            duration_p50_succeeded_seconds: 1,
            duration_p95_succeeded_seconds: 1,
          },
        },
      },
    });
    expect(got.runner.by_runner["cursor-cli"].succeeded).toBe(2);
    expect(got.runner.by_model[""].by_status.failed).toBe(1);
    expect(got.runner.by_runner_model["cursor-cli|sonnet-4.5"].duration_p95_succeeded_seconds).toBe(1);
  });

  it("rejects unknown cycle status keys inside a runner bucket", () => {
    expect(() =>
      parseTaskStatsResponse({
        total: 0,
        ready: 0,
        critical: 0,
        by_status: {},
        by_priority: {},
        by_scope: { parent: 0, subtask: 0 },
        ...emptyExtras,
        runner: {
          by_runner: {
            "cursor-cli": {
              by_status: { weird: 1 },
              succeeded: 0,
              duration_p50_succeeded_seconds: 0,
              duration_p95_succeeded_seconds: 0,
            },
          },
          by_model: {},
          by_runner_model: {},
        },
      }),
    ).toThrow(/by_status\.weird/);
  });

  it("rejects missing runner block entirely", () => {
    const { runner: _omit, ...withoutRunner } = emptyExtras;
    expect(() =>
      parseTaskStatsResponse({
        total: 0,
        ready: 0,
        critical: 0,
        by_status: {},
        by_priority: {},
        by_scope: { parent: 0, subtask: 0 },
        ...withoutRunner,
      }),
    ).toThrow(/runner must be an object/);
  });
});

describe("parseTaskEventsResponse", () => {
  it("parses events envelope", () => {
    const at = "2026-01-01T12:00:00Z";
    expect(
      parseTaskEventsResponse({
        task_id: "tid",
        events: [
          {
            seq: 1,
            at,
            type: "task_created",
            by: "user",
            data: {},
          },
        ],
      }),
    ).toEqual({
      task_id: "tid",
      events: [
        {
          seq: 1,
          at,
          type: "task_created",
          by: "user",
          data: {},
        },
      ],
      approval_pending: false,
      has_more_newer: false,
      has_more_older: false,
    });
  });

  it("parses optional user_response on events", () => {
    const at = "2026-01-01T12:00:00Z";
    expect(
      parseTaskEventsResponse({
        task_id: "tid",
        events: [
          {
            seq: 2,
            at,
            type: "approval_requested",
            by: "agent",
            data: {},
            user_response: "Approved",
          },
        ],
        approval_pending: false,
      }),
    ).toEqual({
      task_id: "tid",
      events: [
        {
          seq: 2,
          at,
          type: "approval_requested",
          by: "agent",
          data: {},
          user_response: "Approved",
          response_thread: [{ at, by: "user", body: "Approved" }],
        },
      ],
      approval_pending: false,
      has_more_newer: false,
      has_more_older: false,
    });
  });

  it("accepts every server-declared EventType (regression: cycle/phase mirrors)", () => {
    // The backend emits cycle_started / cycle_completed / cycle_failed /
    // phase_started / phase_completed / phase_failed / phase_skipped audit
    // mirrors as soon as a real agent run dispatches (see
    // pkgs/tasks/domain/enums.go). When TASK_EVENT_TYPES drifted from the
    // server enum, parseTaskEventsResponse rejected the entire /events
    // payload with "event type must be a known value" the moment any of
    // those rows landed, collapsing the whole Updates section into an
    // error banner. Walk every declared TaskEventType through the parser
    // so future server-side additions either get mirrored here or fail
    // this test loudly instead of breaking the timeline silently in prod.
    const at = "2026-01-01T12:00:00Z";
    const events = TASK_EVENT_TYPES.map((type, idx) => ({
      seq: idx + 1,
      at,
      type,
      by: "agent" as const,
      data: {},
    }));
    const out = parseTaskEventsResponse({ task_id: "tid", events });
    expect(out.events.map((e) => e.type)).toEqual(
      TASK_EVENT_TYPES as readonly string[],
    );
  });

  it("parses keyset-paged envelope", () => {
    const at = "2026-01-01T12:00:00Z";
    expect(
      parseTaskEventsResponse({
        task_id: "tid",
        limit: 20,
        total: 45,
        range_start: 21,
        range_end: 40,
        has_more_newer: true,
        has_more_older: true,
        approval_pending: true,
        events: [
          {
            seq: 3,
            at,
            type: "sync_ping",
            by: "user",
            data: {},
          },
        ],
      }),
    ).toEqual({
      task_id: "tid",
      limit: 20,
      total: 45,
      range_start: 21,
      range_end: 40,
      has_more_newer: true,
      has_more_older: true,
      approval_pending: true,
      events: [
        {
          seq: 3,
          at,
          type: "sync_ping",
          by: "user",
          data: {},
        },
      ],
    });
  });
});

describe("parseTaskEventDetail", () => {
  it("parses GET /tasks/{id}/events/{seq} envelope", () => {
    const at = "2026-01-02T15:30:00.000Z";
    expect(
      parseTaskEventDetail({
        task_id: "tid",
        seq: 4,
        at,
        type: "approval_requested",
        by: "agent",
        data: { reason: "review" },
      }),
    ).toEqual({
      task_id: "tid",
      seq: 4,
      at,
      type: "approval_requested",
      by: "agent",
      data: { reason: "review" },
    });
  });

  it("parses user_response on event detail", () => {
    const at = "2026-01-02T15:30:00.000Z";
    const user_response_at = "2026-01-02T16:00:00.000Z";
    expect(
      parseTaskEventDetail({
        task_id: "tid",
        seq: 4,
        at,
        type: "task_failed",
        by: "agent",
        data: {},
        user_response: "Retry scheduled",
        user_response_at,
      }),
    ).toEqual({
      task_id: "tid",
      seq: 4,
      at,
      type: "task_failed",
      by: "agent",
      data: {},
      user_response: "Retry scheduled",
      user_response_at,
      response_thread: [
        { at: user_response_at, by: "user", body: "Retry scheduled" },
      ],
    });
  });
});

describe("parseDraftTaskEvaluation", () => {
  it("parses draft evaluation payload", () => {
    const createdAt = "2026-01-02T15:30:00.000Z";
    expect(
      parseDraftTaskEvaluation({
        evaluation_id: "eval-1",
        created_at: createdAt,
        overall_score: 82,
        overall_summary: "Promising draft with a few improvement opportunities.",
        sections: [
          {
            key: "title",
            label: "Title quality",
            score: 90,
            summary: "Title is clear and specific.",
            suggestions: ["Use a verb + object format in the title."],
          },
        ],
        cohesion_score: 78,
        cohesion_summary: "Most sections align, but intent can be sharpened.",
        cohesion_suggestions: [
          "Ensure title, prompt, and priority describe the same outcome.",
        ],
      }),
    ).toEqual({
      evaluation_id: "eval-1",
      created_at: createdAt,
      overall_score: 82,
      overall_summary: "Promising draft with a few improvement opportunities.",
      sections: [
        {
          key: "title",
          label: "Title quality",
          score: 90,
          summary: "Title is clear and specific.",
          suggestions: ["Use a verb + object format in the title."],
        },
      ],
      cohesion_score: 78,
      cohesion_summary: "Most sections align, but intent can be sharpened.",
      cohesion_suggestions: [
        "Ensure title, prompt, and priority describe the same outcome.",
      ],
    });
  });
});

const validCycle = {
  id: "cyc-1",
  task_id: "task-1",
  attempt_seq: 1,
  status: "running",
  started_at: "2026-04-18T10:00:00.000Z",
  triggered_by: "user",
  meta: { source: "manual" },
};

const emptyCycleMeta = {
  runner: "",
  runner_version: "",
  cursor_model: "",
  cursor_model_effective: "",
  prompt_hash: "",
};

describe("parseTaskCycle", () => {
  it("accepts a well-formed running cycle and defaults meta when missing", () => {
    expect(parseTaskCycle(validCycle)).toEqual({
      ...validCycle,
      cycle_meta: emptyCycleMeta,
    });
    const noMeta = { ...validCycle };
    delete (noMeta as Partial<typeof validCycle>).meta;
    expect(parseTaskCycle(noMeta)).toEqual({
      ...validCycle,
      meta: {},
      cycle_meta: emptyCycleMeta,
    });
  });

  it("includes optional ended_at and parent_cycle_id when present", () => {
    const out = parseTaskCycle({
      ...validCycle,
      status: "succeeded",
      ended_at: "2026-04-18T10:05:00.000Z",
      parent_cycle_id: "cyc-0",
    });
    expect(out.ended_at).toBe("2026-04-18T10:05:00.000Z");
    expect(out.parent_cycle_id).toBe("cyc-0");
  });

  it("rejects unknown status, bad actor, and unparseable started_at", () => {
    expect(() => parseTaskCycle({ ...validCycle, status: "weird" })).toThrow(
      /known cycle status/,
    );
    expect(() =>
      parseTaskCycle({ ...validCycle, triggered_by: "robot" }),
    ).toThrow(/user or agent/);
    expect(() =>
      parseTaskCycle({ ...validCycle, started_at: "not-a-date" }),
    ).toThrow(/started_at/);
  });

  it("extracts the typed cycle_meta projection when the server provides it", () => {
    const out = parseTaskCycle({
      ...validCycle,
      meta: {
        runner: "cursor-cli",
        runner_version: "0.42.0",
        cursor_model: "",
        cursor_model_effective: "opus",
        prompt_hash: "deadbeef",
      },
      cycle_meta: {
        runner: "cursor-cli",
        runner_version: "0.42.0",
        cursor_model: "",
        cursor_model_effective: "opus",
        prompt_hash: "deadbeef",
      },
    });
    expect(out.cycle_meta).toEqual({
      runner: "cursor-cli",
      runner_version: "0.42.0",
      cursor_model: "",
      cursor_model_effective: "opus",
      prompt_hash: "deadbeef",
    });
  });

  it("falls back to meta when cycle_meta is absent (forward/back compat)", () => {
    const out = parseTaskCycle({
      ...validCycle,
      meta: {
        runner: "cursor-cli",
        runner_version: "0.42.0",
        cursor_model: "opus",
        cursor_model_effective: "opus",
        prompt_hash: "deadbeef",
      },
    });
    // Same shape as the cycle_meta object the server would have sent.
    expect(out.cycle_meta).toEqual({
      runner: "cursor-cli",
      runner_version: "0.42.0",
      cursor_model: "opus",
      cursor_model_effective: "opus",
      prompt_hash: "deadbeef",
    });
  });

  it("preserves empty strings as semantic values, not coerced to undefined", () => {
    const out = parseTaskCycle({
      ...validCycle,
      cycle_meta: {
        runner: "cursor-cli",
        runner_version: "0.42.0",
        cursor_model: "",
        cursor_model_effective: "",
        prompt_hash: "",
      },
    });
    // "" is the truth: no model anywhere — must NOT be coerced to undefined.
    expect(out.cycle_meta.cursor_model).toBe("");
    expect(out.cycle_meta.cursor_model_effective).toBe("");
    expect(out.cycle_meta.prompt_hash).toBe("");
  });
});

const validPhase = {
  id: "ph-1",
  cycle_id: "cyc-1",
  phase: "diagnose",
  phase_seq: 1,
  status: "running",
  started_at: "2026-04-18T10:00:01.000Z",
  details: {},
};

describe("parseTaskCyclePhase", () => {
  it("accepts a well-formed running phase and defaults details when missing", () => {
    expect(parseTaskCyclePhase(validPhase)).toEqual(validPhase);
    const noDetails = { ...validPhase };
    delete (noDetails as Partial<typeof validPhase>).details;
    expect(parseTaskCyclePhase(noDetails)).toEqual({
      ...validPhase,
      details: {},
    });
  });

  it("includes optional summary, ended_at, event_seq when present", () => {
    const out = parseTaskCyclePhase({
      ...validPhase,
      status: "succeeded",
      ended_at: "2026-04-18T10:01:00.000Z",
      summary: "diagnosed root cause",
      event_seq: 7,
      details: { hint: "x" },
    });
    expect(out.summary).toBe("diagnosed root cause");
    expect(out.ended_at).toBe("2026-04-18T10:01:00.000Z");
    expect(out.event_seq).toBe(7);
    expect(out.details).toEqual({ hint: "x" });
  });

  it("rejects unknown phase or status", () => {
    expect(() => parseTaskCyclePhase({ ...validPhase, phase: "ship" })).toThrow(
      /known phase/,
    );
    expect(() => parseTaskCyclePhase({ ...validPhase, status: "weird" })).toThrow(
      /known phase status/,
    );
  });
});

describe("parseTaskCyclesListResponse", () => {
  it("parses an empty list with limit and has_more", () => {
    expect(
      parseTaskCyclesListResponse({
        task_id: "task-1",
        cycles: [],
        limit: 50,
        has_more: false,
      }),
    ).toEqual({ task_id: "task-1", cycles: [], limit: 50, has_more: false });
  });

  it("parses cycles array element-by-element with index in error", () => {
    expect(() =>
      parseTaskCyclesListResponse({
        task_id: "task-1",
        cycles: [validCycle, { ...validCycle, status: "weird" }],
        limit: 10,
        has_more: false,
      }),
    ).toThrow(/cycles\[1\]/);
  });

  it("rejects when cycles is missing or not an array", () => {
    expect(() =>
      parseTaskCyclesListResponse({
        task_id: "task-1",
        limit: 10,
        has_more: false,
      }),
    ).toThrow(/cycles must be an array/);
  });
});

describe("parseTaskCycleDetail", () => {
  it("parses cycle + ordered phases envelope", () => {
    const out = parseTaskCycleDetail({
      ...validCycle,
      phases: [
        validPhase,
        {
          ...validPhase,
          id: "ph-2",
          phase: "execute",
          phase_seq: 2,
          status: "running",
        },
      ],
    });
    expect(out.id).toBe("cyc-1");
    expect(out.phases).toHaveLength(2);
    expect(out.phases[1].phase).toBe("execute");
  });

  it("rejects when phases is missing", () => {
    expect(() => parseTaskCycleDetail(validCycle)).toThrow(
      /phases must be an array/,
    );
  });
});

describe("parseTaskDraftDetail (payload.priority validation)", () => {
  // Regression: parseDraftPayload used to do
  //   priority: (value.priority as TaskDraftPayload["priority"]) ?? "",
  // which let arbitrary server values through unvalidated, even though
  // sibling pending_subtasks priorities are properly validated by
  // parsePriority(). The downstream UI (PrioritySelect) silently drops
  // invalid values; the parser is the chokepoint that should reject them.
  const baseDraft = {
    id: "d1",
    name: "draft",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    payload: {
      title: "t",
      initial_prompt: "p",
      priority: "medium",
      task_type: "general",
      parent_id: "",
      checklist_inherit: false,
      checklist_items: [],
      pending_subtasks: [],
    },
  };

  it("accepts a valid PriorityChoice value on the parent draft", () => {
    const out = parseTaskDraftDetail(baseDraft);
    expect(out.payload.priority).toBe("medium");
  });

  it("accepts an empty-string priority (user has not selected one yet)", () => {
    const out = parseTaskDraftDetail({
      ...baseDraft,
      payload: { ...baseDraft.payload, priority: "" },
    });
    expect(out.payload.priority).toBe("");
  });

  it("defaults a missing priority to empty string", () => {
    const { priority: _omit, ...rest } = baseDraft.payload;
    const out = parseTaskDraftDetail({ ...baseDraft, payload: rest });
    expect(out.payload.priority).toBe("");
  });

  it("rejects an unknown priority string on the parent draft", () => {
    expect(() =>
      parseTaskDraftDetail({
        ...baseDraft,
        payload: { ...baseDraft.payload, priority: "Critical" },
      }),
    ).toThrow(/known task priority/);
  });

  it("rejects a non-string priority on the parent draft", () => {
    expect(() =>
      parseTaskDraftDetail({
        ...baseDraft,
        payload: { ...baseDraft.payload, priority: 42 },
      }),
    ).toThrow(/known task priority/);
  });
});
