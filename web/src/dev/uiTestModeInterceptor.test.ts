import { afterEach, describe, expect, it, vi } from "vitest";
import * as uiTestMode from "./uiTestMode";
import { interceptUiTestModeFetch } from "./uiTestModeInterceptor";

describe("interceptUiTestModeFetch", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns null when UI test mode is off", () => {
    vi.spyOn(uiTestMode, "isUiTestMode").mockReturnValue(false);
    expect(interceptUiTestModeFetch("/projects", undefined)).toBeNull();
  });

  it("returns synthetic JSON for GET /projects when mode is on", async () => {
    vi.spyOn(uiTestMode, "isUiTestMode").mockReturnValue(true);
    const res = interceptUiTestModeFetch("/projects", undefined);
    expect(res).not.toBeNull();
    expect(res?.ok).toBe(true);
    const body = (await res!.json()) as { projects: unknown[] };
    expect(Array.isArray(body.projects)).toBe(true);
    expect(body.projects.length).toBeGreaterThanOrEqual(3);
  });

  it("does not intercept non-GET requests", () => {
    vi.spyOn(uiTestMode, "isUiTestMode").mockReturnValue(true);
    expect(
      interceptUiTestModeFetch("/projects", { method: "POST", body: "{}" }),
    ).toBeNull();
  });
});
