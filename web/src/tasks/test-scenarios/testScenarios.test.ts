import { describe, expect, it } from "vitest";
import { PRIORITIES } from "@/types";
import {
  TEST_SCENARIO_DIFFICULTY_ORDER,
  TEST_SCENARIOS,
  findTestScenarioById,
  groupTestScenariosByDifficulty,
} from "./testScenarios";

describe("TEST_SCENARIOS catalog", () => {
  it("has at least one scenario per difficulty bucket", () => {
    const byDifficulty = groupTestScenariosByDifficulty();
    for (const difficulty of TEST_SCENARIO_DIFFICULTY_ORDER) {
      expect(byDifficulty[difficulty].length).toBeGreaterThan(0);
    }
  });

  it("every scenario has a unique id", () => {
    const ids = TEST_SCENARIOS.map((s) => s.id);
    const unique = new Set(ids);
    expect(unique.size).toBe(ids.length);
  });

  it("every scenario picks a known Priority", () => {
    for (const scenario of TEST_SCENARIOS) {
      expect(PRIORITIES).toContain(scenario.priority);
    }
  });

  it("every scenario has non-empty title, description, prompt, and at least one checklist item", () => {
    for (const scenario of TEST_SCENARIOS) {
      expect(scenario.title.trim()).not.toBe("");
      expect(scenario.description.trim()).not.toBe("");
      expect(scenario.prompt.trim()).not.toBe("");
      expect(scenario.checklist.length).toBeGreaterThan(0);
      for (const item of scenario.checklist) {
        expect(item.trim()).not.toBe("");
      }
    }
  });

  it("findTestScenarioById returns the matching scenario or undefined", () => {
    const first = TEST_SCENARIOS[0]!;
    expect(findTestScenarioById(first.id)?.id).toBe(first.id);
    expect(findTestScenarioById("does-not-exist")).toBeUndefined();
  });

  it("groupTestScenariosByDifficulty preserves catalog order within each bucket", () => {
    const byDifficulty = groupTestScenariosByDifficulty();
    for (const difficulty of TEST_SCENARIO_DIFFICULTY_ORDER) {
      const fromCatalog = TEST_SCENARIOS.filter(
        (s) => s.difficulty === difficulty,
      ).map((s) => s.id);
      const fromGroup = byDifficulty[difficulty].map((s) => s.id);
      expect(fromGroup).toEqual(fromCatalog);
    }
  });

  it("does not place the same scenario in multiple difficulty buckets", () => {
    const byDifficulty = groupTestScenariosByDifficulty();
    const totalGrouped = TEST_SCENARIO_DIFFICULTY_ORDER.reduce(
      (sum, d) => sum + byDifficulty[d].length,
      0,
    );
    expect(totalGrouped).toBe(TEST_SCENARIOS.length);
  });

});
