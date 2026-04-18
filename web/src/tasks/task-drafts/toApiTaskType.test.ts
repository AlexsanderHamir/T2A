import { describe, expect, it } from "vitest";
import { TASK_TYPES, type TaskType } from "@/types";
import { toApiTaskType } from "./toApiTaskType";

describe("toApiTaskType", () => {
  it("rewrites dmap to general (server has no dmap enum value)", () => {
    expect(toApiTaskType("dmap")).toBe("general");
  });

  it("passes every non-dmap UI task type through unchanged", () => {
    const passThrough: TaskType[] = TASK_TYPES.filter((t) => t !== "dmap");
    for (const t of passThrough) {
      expect(toApiTaskType(t)).toBe(t);
    }
  });
});
