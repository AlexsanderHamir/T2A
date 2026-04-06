import { describe, expect, it } from "vitest";
import {
  maxTaskParseDepth,
  parseTask,
  parseTaskEventDetail,
  parseTaskEventsResponse,
  parseDraftTaskEvaluation,
  parseTaskListResponse,
  parseTaskStatsResponse,
} from "./parseTaskApi";

const validTask = {
  id: "a1",
  title: "One",
  initial_prompt: "",
  status: "ready",
  priority: "medium",
  task_type: "general",
  checklist_inherit: false,
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
  it("parses task stats envelope", () => {
    expect(
      parseTaskStatsResponse({
        total: 22,
        ready: 7,
        critical: 2,
        by_status: { ready: 7, running: 5 },
        by_priority: { critical: 2, high: 4 },
        by_scope: { parent: 10, subtask: 12 },
      }),
    ).toEqual({
      total: 22,
      ready: 7,
      critical: 2,
      by_status: { ready: 7, running: 5 },
      by_priority: { critical: 2, high: 4 },
      by_scope: { parent: 10, subtask: 12 },
    });
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
      }),
    ).toThrow(/by_scope\.subtask/);
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
