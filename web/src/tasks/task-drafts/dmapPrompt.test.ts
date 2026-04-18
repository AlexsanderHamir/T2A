import { describe, expect, it } from "vitest";
import { buildDmapPrompt, normalizeDmapCommitLimit } from "./dmapPrompt";

describe("normalizeDmapCommitLimit", () => {
  it("parses positive integers as-is", () => {
    expect(normalizeDmapCommitLimit("1")).toBe(1);
    expect(normalizeDmapCommitLimit("3")).toBe(3);
    expect(normalizeDmapCommitLimit("42")).toBe(42);
  });

  it("falls back to 1 for empty, zero, negative, or NaN input", () => {
    expect(normalizeDmapCommitLimit("")).toBe(1);
    expect(normalizeDmapCommitLimit("0")).toBe(1);
    expect(normalizeDmapCommitLimit("-3")).toBe(1);
    expect(normalizeDmapCommitLimit("abc")).toBe(1);
  });

  it("truncates floats to their integer part when the part is positive", () => {
    expect(normalizeDmapCommitLimit("2.9")).toBe(2);
  });
});

describe("buildDmapPrompt", () => {
  it("emits header, commit cap, and trimmed domain", () => {
    const out = buildDmapPrompt({
      commitLimit: "3",
      domain: "  payments  ",
      description: "",
    });
    expect(out).toBe(
      [
        "DMAP session setup",
        "",
        "- Commits until stoppage: 3",
        "- Domain: payments",
      ].join("\n"),
    );
  });

  it("renders 'unspecified' for an empty domain", () => {
    const out = buildDmapPrompt({
      commitLimit: "1",
      domain: "   ",
      description: "",
    });
    expect(out).toContain("- Domain: unspecified");
  });

  it("appends the trimmed direction line when description is provided", () => {
    const out = buildDmapPrompt({
      commitLimit: "2",
      domain: "billing",
      description: "  focus on retries  ",
    });
    expect(out.split("\n").at(-1)).toBe("- Direction: focus on retries");
  });

  it("normalizes the commit cap when the raw value is malformed", () => {
    const out = buildDmapPrompt({
      commitLimit: "not-a-number",
      domain: "x",
      description: "",
    });
    expect(out).toContain("- Commits until stoppage: 1");
  });
});
