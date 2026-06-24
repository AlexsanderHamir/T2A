import { describe, expect, it } from "vitest";
import { makeTaskWithCreatedAt } from "@/test/taskDefaults";
import { sortTasksByCreatedDesc } from "./taskListSort";

describe("sortTasksByCreatedDesc", () => {
  it("orders newest created_at first", () => {
    const sorted = sortTasksByCreatedDesc([
      makeTaskWithCreatedAt("old", "2026-01-01T00:00:00Z"),
      makeTaskWithCreatedAt("new", "2026-06-20T12:00:00Z"),
      makeTaskWithCreatedAt("mid", "2026-03-01T00:00:00Z"),
    ]);
    expect(sorted.map((t) => t.id)).toEqual(["new", "mid", "old"]);
  });
});
