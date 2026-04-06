import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchWithTimeout, readError } from "./shared";

describe("fetchWithTimeout", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("uses timeout signal when caller signal is missing", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("", { status: 200 }),
    );

    await fetchWithTimeout("/tasks");

    const [, init] = fetchSpy.mock.calls[0] as [RequestInfo | URL, RequestInit];
    expect(init.signal).toBeDefined();
  });

  it("combines caller signal with timeout signal when AbortSignal.any exists", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("", { status: 200 }),
    );
    const userSignal = new AbortController().signal;
    await fetchWithTimeout("/tasks", { signal: userSignal });
    const [, init] = fetchSpy.mock.calls[0] as [RequestInfo | URL, RequestInit];
    if (typeof (AbortSignal as typeof AbortSignal & { any?: unknown }).any === "function") {
      expect(init.signal).not.toBe(userSignal);
      return;
    }
    expect(init.signal).toBe(userSignal);
  });
});

describe("readError", () => {
  it("returns trimmed error string from JSON", async () => {
    const msg = await readError(
      new Response(JSON.stringify({ error: "  bad  " }), { status: 400 }),
    );
    expect(msg).toBe("bad");
  });

  it("appends request_id when both are present", async () => {
    const msg = await readError(
      new Response(JSON.stringify({ error: "nope", request_id: "req-abc" }), {
        status: 400,
      }),
    );
    expect(msg).toBe("nope (request req-abc)");
  });

  it("returns request-only message when error is missing", async () => {
    const msg = await readError(
      new Response(JSON.stringify({ request_id: "req-only" }), { status: 500 }),
    );
    expect(msg).toBe("Error (request req-only)");
  });

  it("falls back to body text when JSON has no error or request_id", async () => {
    const msg = await readError(new Response("plain", { status: 502 }));
    expect(msg).toBe("plain");
  });

  it("falls back to statusText when body empty", async () => {
    const msg = await readError(new Response("", { status: 503, statusText: "Service Unavailable" }));
    expect(msg).toBe("Service Unavailable");
  });
});
