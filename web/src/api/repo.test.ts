import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { probeRepoWorkspace } from "./repo";

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
});
