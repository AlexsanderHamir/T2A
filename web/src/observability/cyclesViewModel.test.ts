import { describe, expect, it } from "vitest";
import type { CycleMeta } from "@/types/cycle";
import {
  cycleRunnerChipClass,
  formatRunnerModel,
  runnerLabel,
} from "./cyclesViewModel";

function meta(overrides: Partial<CycleMeta> = {}): CycleMeta {
  return {
    runner: "cursor",
    runner_version: "v1",
    cursor_model: "",
    cursor_model_effective: "",
    prompt_hash: "deadbeef",
    ...overrides,
  };
}

describe("runnerLabel", () => {
  it("maps canonical runner names to operator-friendly labels", () => {
    expect(runnerLabel("cursor")).toBe("Cursor CLI");
    expect(runnerLabel("cursor-cli")).toBe("Cursor CLI");
    expect(runnerLabel("fake")).toBe("Fake runner");
  });

  it("returns unknown runner for the empty string (pre-feature cycles)", () => {
    expect(runnerLabel("")).toBe("unknown runner");
    expect(runnerLabel("   ")).toBe("unknown runner");
  });

  it("falls through verbatim for unmapped runner identifiers", () => {
    expect(runnerLabel("future-runner-v2")).toBe("future-runner-v2");
  });
});

describe("formatRunnerModel", () => {
  it("renders runner and effective model when both present", () => {
    expect(formatRunnerModel(meta({ cursor_model_effective: "opus-4" }))).toBe(
      "Cursor CLI · opus-4",
    );
  });

  it("renders 'default model' when the runner is known but model is empty", () => {
    expect(formatRunnerModel(meta({ cursor_model_effective: "" }))).toBe(
      "Cursor CLI · default model",
    );
  });

  it("returns 'unknown runner' without a model suffix for pre-feature cycles", () => {
    expect(
      formatRunnerModel(meta({ runner: "", cursor_model_effective: "whatever" })),
    ).toBe("unknown runner");
  });

  it("reads cursor_model_effective, not cursor_model (plan decision D1)", () => {
    // The operator's intent was "opus", but the runner resolved to
    // "sonnet-4.5" at cycle start. The chip shows the resolved value
    // so the UI and Prometheus agree.
    expect(
      formatRunnerModel(
        meta({
          cursor_model: "opus",
          cursor_model_effective: "sonnet-4.5",
        }),
      ),
    ).toBe("Cursor CLI · sonnet-4.5");
  });

  it("trims whitespace-only effective model to default", () => {
    expect(
      formatRunnerModel(meta({ cursor_model_effective: "   " })),
    ).toBe("Cursor CLI · default model");
  });
});

describe("cycleRunnerChipClass", () => {
  it("returns the shared runtime variant class", () => {
    expect(cycleRunnerChipClass()).toBe("cell-pill--runtime");
  });
});
