import { describe, expect, it } from "vitest";
import {
  materializeSubtaskDependsOn,
  remapPendingSubtaskSiblingIndices,
  SubtaskDependsOnError,
} from "./materializeSubtaskDependsOn";

describe("materializeSubtaskDependsOn", () => {
  it("returns empty when no opts selected", () => {
    expect(
      materializeSubtaskDependsOn({
        waitForParent: false,
        parentId: "p1",
      }),
    ).toEqual([]);
  });

  it("includes parent with criteria_complete when waitForParent is true", () => {
    expect(
      materializeSubtaskDependsOn({
        waitForParent: true,
        parentId: "p1",
      }),
    ).toEqual([{ task_id: "p1", satisfies: "criteria_complete" }]);
  });

  it("dedupes parent when also listed as explicit sibling id", () => {
    expect(
      materializeSubtaskDependsOn({
        waitForParent: true,
        parentId: "p1",
        siblingIds: ["p1", "s2"],
      }),
    ).toEqual([
      { task_id: "p1", satisfies: "criteria_complete" },
      { task_id: "s2", satisfies: "done" },
    ]);
  });

  it("resolves sibling indices via map", () => {
    const map = new Map<number, string>([
      [0, "a"],
      [1, "b"],
    ]);
    expect(
      materializeSubtaskDependsOn({
        waitForParent: true,
        parentId: "parent",
        siblingIndices: [0, 1],
        siblingIdsByIndex: map,
        selfIndex: 2,
      }),
    ).toEqual([
      { task_id: "parent", satisfies: "criteria_complete" },
      { task_id: "a", satisfies: "done" },
      { task_id: "b", satisfies: "done" },
    ]);
  });

  it("rejects self-index", () => {
    expect(() =>
      materializeSubtaskDependsOn({
        waitForParent: false,
        parentId: "p",
        siblingIndices: [1],
        siblingIdsByIndex: new Map([[1, "x"]]),
        selfIndex: 1,
      }),
    ).toThrow(SubtaskDependsOnError);
  });

  it("rejects unknown sibling index", () => {
    expect(() =>
      materializeSubtaskDependsOn({
        waitForParent: false,
        parentId: "p",
        siblingIndices: [99],
        siblingIdsByIndex: new Map([[0, "a"]]),
      }),
    ).toThrow(/unknown sibling index/);
  });
});

describe("remapPendingSubtaskSiblingIndices", () => {
  it("drops references to removed row and shifts higher indices", () => {
    expect(remapPendingSubtaskSiblingIndices([0, 1, 2], 1)).toEqual([0, 1]);
  });
});
