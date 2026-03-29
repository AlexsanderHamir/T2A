import { afterEach, describe, expect, it, vi } from "vitest";
import { createTask, deleteTask, listTasks, patchTask } from "./index";

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
            },
          ],
          limit: 50,
          offset: 0,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    const out = await listTasks(50, 0);
    expect(out.tasks).toHaveLength(1);
    expect(out.tasks[0].title).toBe("One");
    expect(out.limit).toBe(50);
    expect(out.offset).toBe(0);

    expect(fetch).toHaveBeenCalledWith(
      expect.stringMatching(/^\/tasks\?/),
      expect.objectContaining({ headers: { Accept: "application/json" } }),
    );
  });

  it("forwards AbortSignal to fetch when provided", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({ tasks: [], limit: 200, offset: 0 }),
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
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      ),
    );

    await createTask({ title: "A", initial_prompt: "p", status: "running" });

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
    });
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

describe("deleteTask", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("DELETEs /tasks/{id}", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(null, { status: 204 }),
    );

    await deleteTask("ab/c");

    expect(spy).toHaveBeenCalledWith("/tasks/ab%2Fc", { method: "DELETE" });
  });
});
