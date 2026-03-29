import { describe, expect, it } from "vitest";
import {
  parseTask,
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
    });
  });
});
