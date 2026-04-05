import { describe, expect, it } from "vitest";
import { readError } from "./shared";

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
