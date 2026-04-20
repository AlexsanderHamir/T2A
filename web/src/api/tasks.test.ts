import { afterEach, describe, expect, it, vi } from "vitest";
import {
  createTask,
  deleteTask,
  evaluateDraftTask,
  getTask,
  getTaskStats,
  getTaskEvent,
  listTasks,
  maxTaskPathIDBytes,
  patchTask,
  patchTaskEventUserResponse,
} from "./index";

describe("listTasks", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns typed list response", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          tasks: [
            {
              id: "a",
              title: "One",
              initial_prompt: "",
              status: "ready",
              priority: "medium",
              task_type: "general",
              checklist_inherit: false,
            },
          ],
          limit: 50,
          offset: 0,
          has_more: false,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    const out = await listTasks(50, 0);
    expect(out.tasks).toHaveLength(1);
    expect(out.tasks[0].title).toBe("One");
    expect(out.limit).toBe(50);
    expect(out.offset).toBe(0);
    expect(out.has_more).toBe(false);

    expect(fetch).toHaveBeenCalledWith(
      expect.stringMatching(/^\/tasks\?/),
      expect.objectContaining({ headers: { Accept: "application/json" } }),
    );
  });

  it("uses after_id when provided", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          tasks: [],
          limit: 10,
          offset: 0,
          has_more: false,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    await listTasks(10, 99, { afterId: "11111111-1111-4111-8111-111111111111" });
    const url = String(spy.mock.calls[0][0]);
    expect(url).toContain("after_id=");
    expect(url).not.toContain("offset=");
  });

  it("forwards AbortSignal to fetch when provided", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({ tasks: [], limit: 200, offset: 0, has_more: false }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    const ac = new AbortController();
    await listTasks(200, 0, { signal: ac.signal });
    expect(spy).toHaveBeenCalledWith(
      expect.stringMatching(/^\/tasks\?/),
      expect.objectContaining({
        headers: { Accept: "application/json" },
        signal: ac.signal,
      }),
    );
  });

  it("uses a timeout-backed signal when no signal is provided", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({ tasks: [], limit: 200, offset: 0, has_more: false }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    await listTasks(200, 0);

    const [, init] = spy.mock.calls[0] as [string, RequestInit];
    expect(init.signal).toBeDefined();
  });

  it("throws with response body on error", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("bad request", { status: 400 }),
    );
    await expect(listTasks()).rejects.toThrow("bad request");
  });

  it("rejects JSON that is not a task list envelope", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ tasks: "bad" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    await expect(listTasks()).rejects.toThrow(/array/);
  });

  it("rejects limit out of range before fetch", async () => {
    vi.spyOn(globalThis, "fetch");
    await expect(listTasks(201, 0)).rejects.toThrow(/between/);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("rejects overlong after_id before fetch", async () => {
    vi.spyOn(globalThis, "fetch");
    const longAfter = "a".repeat(maxTaskPathIDBytes + 1);
    await expect(listTasks(10, 0, { afterId: longAfter })).rejects.toThrow(/too long/);
    expect(fetch).not.toHaveBeenCalled();
  });
});

describe("getTask", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("rejects id longer than max before fetch", async () => {
    vi.spyOn(globalThis, "fetch");
    const longId = "x".repeat(maxTaskPathIDBytes + 1);
    await expect(getTask(longId)).rejects.toThrow(/too long/);
    expect(fetch).not.toHaveBeenCalled();
  });
});

describe("getTaskStats", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns typed global stats response", async () => {
    const cyclesPhasesEmpty = {
      cycles: { by_status: {}, by_triggered_by: {} },
      phases: {
        by_phase_status: {
          diagnose: {},
          execute: {},
          verify: {},
          persist: {},
        },
      },
      recent_failures: [],
    };
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          total: 10,
          ready: 3,
          critical: 1,
          by_status: { ready: 3, running: 4, done: 3 },
          by_priority: { low: 1, medium: 6, high: 2, critical: 1 },
          by_scope: { parent: 6, subtask: 4 },
          ...cyclesPhasesEmpty,
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    const out = await getTaskStats();
    expect(out).toEqual({
      total: 10,
      ready: 3,
      critical: 1,
      by_status: { ready: 3, running: 4, done: 3 },
      by_priority: { low: 1, medium: 6, high: 2, critical: 1 },
      by_scope: { parent: 6, subtask: 4 },
      ...cyclesPhasesEmpty,
    });
    expect(fetch).toHaveBeenCalledWith(
      "/tasks/stats",
      expect.objectContaining({ headers: { Accept: "application/json" } }),
    );
  });
});

describe("createTask", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("POSTs JSON to /tasks", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          id: "x",
          title: "A",
          initial_prompt: "",
          status: "ready",
          priority: "medium",
          task_type: "general",
          checklist_inherit: false,
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      ),
    );

    await createTask({
      title: "A",
      initial_prompt: "p",
      status: "running",
      priority: "medium",
      draft_id: "draft-xyz",
    });

    expect(spy).toHaveBeenCalledWith(
      "/tasks",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "Content-Type": "application/json",
          Accept: "application/json",
        }),
      }),
    );
    const [, init] = spy.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(String(init.body))).toMatchObject({
      title: "A",
      initial_prompt: "p",
      status: "running",
      priority: "medium",
      draft_id: "draft-xyz",
    });
  });

  it("defaults status to ready when omitted", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          id: "x",
          title: "A",
          initial_prompt: "",
          status: "ready",
          priority: "medium",
          task_type: "general",
          checklist_inherit: false,
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      ),
    );

    await createTask({ title: "A", priority: "high" });

    const [, init] = spy.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(String(init.body))).toMatchObject({
      title: "A",
      status: "ready",
      priority: "high",
    });
  });
});

describe("evaluateDraftTask", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("POSTs draft payload and parses response", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          evaluation_id: "eval-1",
          created_at: "2026-01-01T12:00:00Z",
          overall_score: 81,
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
          cohesion_score: 74,
          cohesion_summary: "Most sections align, but intent can be sharpened.",
          cohesion_suggestions: [
            "Ensure title, prompt, and priority describe the same outcome.",
          ],
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      ),
    );

    const out = await evaluateDraftTask({
      id: "draft-1",
      title: "Improve API docs",
      initial_prompt: "Update route docs and examples",
      priority: "high",
      checklist_items: [{ text: "Add endpoint row" }],
    });
    expect(out.evaluation_id).toBe("eval-1");

    expect(spy).toHaveBeenCalledWith(
      "/tasks/evaluate",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "Content-Type": "application/json",
          Accept: "application/json",
        }),
      }),
    );
  });
});

describe("patchTask", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("PATCHes only provided fields", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          id: "id1",
          title: "B",
          initial_prompt: "",
          status: "done",
          priority: "low",
          task_type: "general",
          checklist_inherit: false,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    await patchTask("id1", { title: "B", status: "done" });

    expect(spy).toHaveBeenCalledTimes(1);
    const [url, init] = spy.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/tasks/id1");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(String(init.body))).toEqual({
      title: "B",
      status: "done",
    });
  });
});

describe("getTaskEvent", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("GETs /tasks/{id}/events/{seq} and parses body", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          task_id: "t-1",
          seq: 2,
          at: "2026-01-01T12:00:00.000Z",
          type: "task_created",
          by: "user",
          data: {},
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    const out = await getTaskEvent("t-1", 2);
    expect(out.task_id).toBe("t-1");
    expect(out.seq).toBe(2);

    expect(spy).toHaveBeenCalledWith(
      "/tasks/t-1/events/2",
      expect.objectContaining({ headers: { Accept: "application/json" } }),
    );
  });
});

describe("patchTaskEventUserResponse", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("PATCHes user_response and parses event detail", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          task_id: "t-1",
          seq: 3,
          at: "2026-01-01T12:00:00.000Z",
          type: "approval_requested",
          by: "agent",
          data: {},
          user_response: "OK",
          user_response_at: "2026-01-01T12:05:00.000Z",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    const out = await patchTaskEventUserResponse("t-1", 3, "OK");
    expect(out.user_response).toBe("OK");
    expect(out.user_response_at).toBe("2026-01-01T12:05:00.000Z");

    expect(spy).toHaveBeenCalledWith(
      "/tasks/t-1/events/3",
      expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({ user_response: "OK" }),
      }),
    );
  });
});

describe("deleteTask", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("DELETEs /tasks/{id}", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(null, { status: 204 }),
    );

    await deleteTask("ab/c");

    expect(spy).toHaveBeenCalledWith(
      "/tasks/ab%2Fc",
      expect.objectContaining({ method: "DELETE" }),
    );
  });
});
