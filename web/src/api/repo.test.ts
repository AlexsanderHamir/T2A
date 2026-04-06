import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fetchRepoFile, probeRepoWorkspace, validateRepoRange } from "./repo";

describe("probeRepoWorkspace", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns unavailable when ready is ok but workspace_repo is absent", async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(
        JSON.stringify({
          status: "ok",
          checks: { database: "ok" },
          version: "v",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    await expect(probeRepoWorkspace()).resolves.toEqual({
      state: "unavailable",
    });
  });

  it("returns available when workspace_repo is ok", async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(
        JSON.stringify({
          status: "ok",
          checks: { database: "ok", workspace_repo: "ok" },
          version: "v",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    await expect(probeRepoWorkspace()).resolves.toEqual({ state: "available" });
  });

  it("returns broken when ready is degraded with workspace_repo fail", async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(
        JSON.stringify({
          status: "degraded",
          checks: { database: "ok", workspace_repo: "fail" },
          version: "v",
        }),
        { status: 503, headers: { "Content-Type": "application/json" } },
      ),
    );
    await expect(probeRepoWorkspace()).resolves.toEqual({ state: "broken" });
  });

  it("returns unknown when response is not ok and not workspace failure", async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(
        JSON.stringify({
          status: "degraded",
          checks: { database: "fail" },
          version: "v",
        }),
        { status: 503, headers: { "Content-Type": "application/json" } },
      ),
    );
    await expect(probeRepoWorkspace()).resolves.toEqual({ state: "unknown" });
  });

  it("returns unknown when fetch throws", async () => {
    vi.mocked(fetch).mockRejectedValue(new Error("down"));
    await expect(probeRepoWorkspace()).resolves.toEqual({ state: "unknown" });
  });

  it("attaches a signal when no caller signal is provided", async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(
        JSON.stringify({
          status: "ok",
          checks: { database: "ok", workspace_repo: "ok" },
          version: "v",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    await probeRepoWorkspace();

    const [, init] = vi.mocked(fetch).mock.calls[0] as [string, RequestInit];
    expect(init.signal).toBeDefined();
  });
});

describe("fetchRepoFile", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns null on 503", async () => {
    vi.mocked(fetch).mockResolvedValue(new Response("", { status: 503 }));
    await expect(fetchRepoFile("a.go")).resolves.toBeNull();
  });

  it("parses ok JSON", async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(
        JSON.stringify({
          path: "a.go",
          content: "x",
          binary: false,
          truncated: false,
          size_bytes: 1,
          line_count: 1,
          warning: "w",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    await expect(fetchRepoFile("a.go")).resolves.toEqual({
      path: "a.go",
      content: "x",
      binary: false,
      truncated: false,
      size_bytes: 1,
      line_count: 1,
      warning: "w",
    });
  });
});

describe("validateRepoRange", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("attaches a signal when no caller signal is provided", async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(
        JSON.stringify({ ok: true, line_count: 10 }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    await validateRepoRange("a.go", 1, 2);

    const [, init] = vi.mocked(fetch).mock.calls[0] as [string, RequestInit];
    expect(init.signal).toBeDefined();
  });
});
