import { describe, expect, it } from "vitest";
import { DEFAULT_PROJECT_ID } from "@/types";
import {
  parseProjectContextListResponse,
  parseProjectGoalsListResponse,
  parseProjectListResponse,
  parseProjectStepsListResponse,
} from "@/api/projects";
import { parseTaskListResponse, parseTaskStatsResponse } from "@/api/parseTaskApi";
import {
  demoContextWire,
  demoGoalsWire,
  demoProjectsListWire,
  demoStepsWire,
  demoTaskStatsWire,
  demoTasksListWire,
} from "./uiTestModeDemoWire";

describe("uiTestModeDemoWire", () => {
  it("parses as valid API payloads", () => {
    expect(() => parseProjectListResponse(demoProjectsListWire())).not.toThrow();
    expect(() => parseProjectGoalsListResponse(demoGoalsWire(DEFAULT_PROJECT_ID))).not.toThrow();
    expect(() => parseProjectStepsListResponse(demoStepsWire(DEFAULT_PROJECT_ID))).not.toThrow();
    expect(() => parseProjectContextListResponse(demoContextWire(DEFAULT_PROJECT_ID))).not.toThrow();
    expect(() => parseTaskListResponse(demoTasksListWire(200, 0, null))).not.toThrow();
    expect(() => parseTaskStatsResponse(demoTaskStatsWire())).not.toThrow();
  });

  it("includes dependent and independent goals on the primary demo project", () => {
    const { goals } = parseProjectGoalsListResponse(demoGoalsWire(DEFAULT_PROJECT_ID));
    const deps = goals.map((g) => g.depends_on_goal_ids.length);
    expect(deps.some((n) => n === 0)).toBe(true);
    expect(deps.some((n) => n >= 1)).toBe(true);
    expect(goals.length).toBeGreaterThanOrEqual(3);
  });
});
