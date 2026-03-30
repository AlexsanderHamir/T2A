import { describe, expect, it } from "vitest";
import {
  parseTask,
  parseTaskEventDetail,
  parseTaskEventsResponse,
  parseTaskListResponse,
} from "./parseTaskApi";

const validTask = {
  id: "a1",
  title: "One",
  initial_prompt: "",
  status: "ready",
  priority: "medium",
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
});

describe("parseTaskListResponse", () => {
  it("parses list envelope", () => {
    expect(
      parseTaskListResponse({
        tasks: [validTask],
        limit: 200,
        offset: 0,
      }),
    ).toEqual({ tasks: [validTask], limit: 200, offset: 0 });
  });

  it("rejects non-array tasks", () => {
    expect(() =>
      parseTaskListResponse({ tasks: {}, limit: 0, offset: 0 }),
    ).toThrow(/array/);
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
