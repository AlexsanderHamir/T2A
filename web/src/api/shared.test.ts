import { afterEach, describe, expect, it, vi } from "vitest";
import {
  ApiError,
  apiErrorFromResponse,
  fetchWithTimeout,
  jsonHeaders,
  maxErrorResponseBodyBytes,
  readError,
} from "./shared";

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
    expect(init.signal).toBeDefined();
    expect(init.signal).not.toBe(userSignal);
  });

  it("falls back to composed controller when AbortSignal.any is unavailable", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("", { status: 200 }),
    );
    const descriptor = Object.getOwnPropertyDescriptor(AbortSignal, "any");
    if (descriptor) {
      Object.defineProperty(AbortSignal, "any", {
        value: undefined,
        configurable: true,
      });
    }
    const userSignal = new AbortController().signal;
    await fetchWithTimeout("/tasks", { signal: userSignal });
    const [, init] = fetchSpy.mock.calls[0] as [RequestInfo | URL, RequestInit];
    expect(init.signal).toBeDefined();
    expect(init.signal).not.toBe(userSignal);
    if (descriptor) {
      Object.defineProperty(AbortSignal, "any", descriptor);
    }
  });

  it("uses manual timeout controller when AbortSignal.timeout is unavailable", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("", { status: 200 }),
    );
    const descriptor = Object.getOwnPropertyDescriptor(AbortSignal, "timeout");
    if (descriptor) {
      Object.defineProperty(AbortSignal, "timeout", {
        value: undefined,
        configurable: true,
      });
    }
    try {
      await fetchWithTimeout("/tasks");
      const [, init] = fetchSpy.mock.calls[0] as [RequestInfo | URL, RequestInit];
      expect(init.signal).toBeDefined();
    } finally {
      if (descriptor) {
        Object.defineProperty(AbortSignal, "timeout", descriptor);
      }
    }
  });
});

describe("jsonHeaders", () => {
  it("pins JSON request headers", () => {
    expect(jsonHeaders).toEqual({
      "Content-Type": "application/json",
      Accept: "application/json",
    });
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

  it("falls back to trimmed body when JSON error is only whitespace", async () => {
    const body = '{ "error": "   " }';
    const msg = await readError(new Response(body, { status: 400 }));
    expect(msg).toBe(body.trim());
  });

  it("ignores non-string error field and uses raw body", async () => {
    const body = '{"error":true}';
    const msg = await readError(new Response(body, { status: 400 }));
    expect(msg).toBe(body);
  });

  it("falls back to statusText when body empty", async () => {
    const msg = await readError(new Response("", { status: 503, statusText: "Service Unavailable" }));
    expect(msg).toBe("Service Unavailable");
  });

  it("streams at most maxErrorResponseBodyBytes from oversized error bodies", async () => {
    const pad = "x".repeat(maxErrorResponseBodyBytes + 500);
    const res = new Response(
      new ReadableStream({
        start(controller) {
          controller.enqueue(new TextEncoder().encode(pad));
          controller.close();
        },
      }),
      { status: 500 },
    );
    const msg = await readError(res);
    expect(msg.length).toBe(maxErrorResponseBodyBytes);
    expect(msg).toBe("x".repeat(maxErrorResponseBodyBytes));
  });

  it("still parses JSON error when body fits under limit", async () => {
    const res = new Response(
      new ReadableStream({
        start(controller) {
          controller.enqueue(
            new TextEncoder().encode(JSON.stringify({ error: "short", request_id: "r1" })),
          );
          controller.close();
        },
      }),
      { status: 400 },
    );
    const msg = await readError(res);
    expect(msg).toBe("short (request r1)");
  });
});

describe("ApiError + apiErrorFromResponse", () => {
  it("preserves status, message, code and request id from the response body", async () => {
    const res = new Response(
      JSON.stringify({
        error: "validation failed",
        code: "validation_failed",
        request_id: "req-42",
      }),
      { status: 422 },
    );
    const err = await apiErrorFromResponse(res);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toBeInstanceOf(Error);
    expect(err.status).toBe(422);
    expect(err.code).toBe("validation_failed");
    expect(err.requestId).toBe("req-42");
    // `.message` keeps the legacy display string so existing UI that
    // renders `error.message` directly does not regress.
    expect(err.message).toBe("validation failed (request req-42)");
  });

  it("falls back to statusText when the body is empty", async () => {
    const res = new Response("", {
      status: 503,
      statusText: "Service Unavailable",
    });
    const err = await apiErrorFromResponse(res);
    expect(err.status).toBe(503);
    expect(err.message).toBe("Service Unavailable");
    expect(err.code).toBeUndefined();
    expect(err.requestId).toBeUndefined();
  });

  it("lets callers branch on status without regex-matching the message", async () => {
    const res = new Response(JSON.stringify({ error: "not found" }), {
      status: 404,
    });
    const err = await apiErrorFromResponse(res);
    expect(err.status).toBe(404);
    if (err instanceof ApiError && err.status === 404) {
      expect(err.message).toBe("not found");
    } else {
      throw new Error("expected an ApiError with status 404");
    }
  });
});
