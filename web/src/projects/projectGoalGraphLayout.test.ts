import { describe, expect, it } from "vitest";
import type { ProjectGoal } from "@/types";
import { computeGoalLayers, goalsByLayerColumns } from "./projectGoalGraphLayout";

function goal(
  id: string,
  deps: string[],
  overrides: Partial<Pick<ProjectGoal, "title">> = {},
): ProjectGoal {
  return {
    id,
    project_id: "p1",
    title: overrides.title ?? id,
    description: "",
    depends_on_goal_ids: deps,
    gate_status: "active",
    gate_hold: false,
    criteria: [],
    created_at: "",
    updated_at: "",
  };
}

describe("computeGoalLayers", () => {
  it("places roots in layer 0 and dependents to the right", () => {
    const goals = [goal("a", []), goal("b", []), goal("c", ["a", "b"])];
    const layers = computeGoalLayers(goals);
    expect(layers.get("a")).toBe(0);
    expect(layers.get("b")).toBe(0);
    expect(layers.get("c")).toBe(1);
  });

  it("chains sequential dependencies", () => {
    const goals = [goal("a", []), goal("b", ["a"]), goal("c", ["b"])];
    const layers = computeGoalLayers(goals);
    expect(layers.get("a")).toBe(0);
    expect(layers.get("b")).toBe(1);
    expect(layers.get("c")).toBe(2);
  });

  it("ignores dependency ids that are not in the goal set", () => {
    const goals = [goal("a", ["missing"])];
    const layers = computeGoalLayers(goals);
    expect(layers.get("a")).toBe(0);
  });
});

describe("goalsByLayerColumns", () => {
  it("returns one column per occupied layer sorted by title within column", () => {
    const goals = [goal("z", []), goal("a", []), goal("m", ["a", "z"])];
    const cols = goalsByLayerColumns(goals);
    expect(cols).toHaveLength(2);
    expect(cols[0].map((g) => g.id).sort()).toEqual(["a", "z"]);
    expect(cols[1].map((g) => g.id)).toEqual(["m"]);
  });
});
