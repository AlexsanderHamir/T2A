import { describe, expect, it } from "vitest";
import { taskCreateModalDmapReady } from "./taskCreateModalDmapReady";

describe("taskCreateModalDmapReady", () => {
  it("is ready when not in DMAP mode regardless of fields", () => {
    expect(taskCreateModalDmapReady(false, "", "")).toBe(true);
    expect(taskCreateModalDmapReady(false, "0", "")).toBe(true);
  });

  it("requires positive integer commit limit and non-empty domain in DMAP mode", () => {
    expect(taskCreateModalDmapReady(true, "5", "example.com")).toBe(true);
    expect(taskCreateModalDmapReady(true, "1", " x ")).toBe(true);
  });

  it("rejects non-positive or unparseable commit limits", () => {
    expect(taskCreateModalDmapReady(true, "0", "example.com")).toBe(false);
    expect(taskCreateModalDmapReady(true, "-3", "example.com")).toBe(false);
    expect(taskCreateModalDmapReady(true, "", "example.com")).toBe(false);
  });

  it("accepts leading digits in commit limit (parseInt prefix)", () => {
    expect(taskCreateModalDmapReady(true, "12abc", "example.com")).toBe(true);
  });

  it("rejects empty or whitespace-only domain", () => {
    expect(taskCreateModalDmapReady(true, "5", "")).toBe(false);
    expect(taskCreateModalDmapReady(true, "5", "   ")).toBe(false);
    expect(taskCreateModalDmapReady(true, "5", "\t\n")).toBe(false);
  });
});
