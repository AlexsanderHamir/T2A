import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { listGlobalGitLiveBranches } from "./gitGlobal";
import { respondGlobalGitApi } from "@/test/handlers/gitGlobal";

describe("gitGlobal live branches API", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === "string" ? input : input.toString();
        const method = init?.method ?? "GET";
        const mocked = respondGlobalGitApi(url, method);
        if (mocked) return mocked;
        return new Response(JSON.stringify({ error: "not found" }), { status: 404 });
      }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("lists live branch refs for a repository", async () => {
    const rows = await listGlobalGitLiveBranches("00000000-0000-4000-8000-000000000010");
    expect(rows).toHaveLength(1);
    expect(rows[0]?.name).toBe("main");
    expect(rows[0]?.head_sha).toBe("abc123");
  });
});
