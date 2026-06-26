import { describe, expect, it } from "vitest";
import type { ProjectContextEdge } from "@/types";
import {
  MAX_SELECTED_PROJECT_CONTEXT_ITEMS,
  expandProjectContextSelection,
  hasProjectContextChildren,
  mergeProjectContextSelection,
  projectContextShortId,
} from "./projectContextRefs";

function edge(source: string, target: string): ProjectContextEdge {
  return {
    id: `${source}->${target}`,
    project_id: "project-1",
    source_context_id: source,
    target_context_id: target,
    relation: "supports",
    strength: 3,
    note: "",
    created_at: "2026-04-29T00:00:00Z",
    updated_at: "2026-04-29T00:00:00Z",
  };
}

describe("projectContextShortId", () => {
  it("strips dashes and lowercases UUIDs to a 6-char prefix", () => {
    expect(projectContextShortId("A1B2C3-D4E5F6")).toBe("a1b2c3");
  });

  it("returns empty for empty input", () => {
    expect(projectContextShortId("")).toBe("");
    expect(projectContextShortId("   ")).toBe("");
  });

  it("falls back to the trimmed id when only punctuation remains", () => {
    expect(projectContextShortId("---")).toBe("---");
  });

  it("preserves alphanumerics from arbitrary ids", () => {
    expect(projectContextShortId("ctx-risk")).toBe("ctxris");
  });
});

describe("expandProjectContextSelection", () => {
  it("returns just the node for nodeOnly mode", () => {
    expect(
      expandProjectContextSelection("a", "nodeOnly", [edge("a", "b")]),
    ).toEqual(["a"]);
  });

  it("returns an empty list when nodeId is blank", () => {
    expect(expandProjectContextSelection("", "withChildren", [])).toEqual([]);
  });

  it("walks outgoing edges in BFS order with cycle protection", () => {
    const edges = [
      edge("a", "b"),
      edge("a", "c"),
      edge("b", "d"),
      edge("c", "d"),
      edge("d", "a"),
    ];
    expect(
      expandProjectContextSelection("a", "withChildren", edges),
    ).toEqual(["a", "b", "c", "d"]);
  });

  it("ignores edges with empty source or target ids", () => {
    const edges = [
      edge("", "b"),
      edge("a", ""),
      edge("a", "c"),
    ];
    expect(
      expandProjectContextSelection("a", "withChildren", edges),
    ).toEqual(["a", "c"]);
  });
});

describe("mergeProjectContextSelection", () => {
  it("appends only new ids and keeps order", () => {
    expect(mergeProjectContextSelection(["a", "b"], ["b", "c"])).toEqual([
      "a",
      "b",
      "c",
    ]);
  });

  it("returns the existing list unchanged when nothing new is added", () => {
    const existing = ["a", "b"];
    const out = mergeProjectContextSelection(existing, ["a"]);
    expect(out).toEqual(existing);
    expect(out).not.toBe(existing);
  });

  it("respects the server-side selection cap", () => {
    const seed: string[] = [];
    for (let i = 0; i < MAX_SELECTED_PROJECT_CONTEXT_ITEMS; i += 1) {
      seed.push(`existing-${i}`);
    }
    const merged = mergeProjectContextSelection(seed, ["new-1", "new-2"]);
    expect(merged.length).toBe(MAX_SELECTED_PROJECT_CONTEXT_ITEMS);
    expect(merged).toEqual(seed);
  });

  it("trims incoming ids and skips empty strings", () => {
    expect(mergeProjectContextSelection([], [" a ", "", "  "])).toEqual(["a"]);
  });
});

describe("hasProjectContextChildren", () => {
  it("is true when there is an outgoing edge", () => {
    expect(hasProjectContextChildren("a", [edge("a", "b")])).toBe(true);
  });

  it("is false when the node has no outgoing edges", () => {
    expect(hasProjectContextChildren("a", [edge("b", "a")])).toBe(false);
  });
});
