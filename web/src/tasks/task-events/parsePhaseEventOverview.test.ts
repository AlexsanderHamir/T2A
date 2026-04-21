import { describe, expect, it } from "vitest";
import {
  normalizePhaseSummaryMarkdown,
  parseCycleTerminalOverview,
  parsePhaseEventOverview,
} from "./parsePhaseEventOverview";

describe("normalizePhaseSummaryMarkdown", () => {
  it("turns literal \\n sequences into newlines for markdown parsers", () => {
    const raw = "| A | B |\\n| --- | --- |\\n| 1 | 2 |";
    expect(normalizePhaseSummaryMarkdown(raw)).toBe(
      "| A | B |\n| --- | --- |\n| 1 | 2 |",
    );
  });

  it("leaves real newlines unchanged", () => {
    const raw = "a\nb";
    expect(normalizePhaseSummaryMarkdown(raw)).toBe("a\nb");
  });
});

describe("parseCycleTerminalOverview", () => {
  it("parses cycle_failed with failure_summary and reason code", () => {
    const m = parseCycleTerminalOverview("cycle_failed", {
      cycle_id: "fa062a1f-6050-47f9-9373-615a32b8eb5b",
      attempt_seq: 1,
      status: "failed",
      reason: "runner_non_zero_exit",
      failure_summary: "Cursor account usage limit reached.",
    });
    expect(m).not.toBeNull();
    expect(m?.terminal).toBe("failed");
    expect(m?.failureSummary).toBe("Cursor account usage limit reached.");
    expect(m?.reason).toBe("runner_non_zero_exit");
  });

  it("returns null when required cycle fields are missing", () => {
    expect(parseCycleTerminalOverview("cycle_failed", { reason: "x" })).toBeNull();
  });
});

describe("parsePhaseEventOverview", () => {
  it("returns null for unrelated event types", () => {
    expect(
      parsePhaseEventOverview("message_added", { phase: "x", status: "y" }),
    ).toBeNull();
  });

  it("parses phase_completed with nested details", () => {
    const m = parsePhaseEventOverview("phase_completed", {
      phase: "execute",
      status: "succeeded",
      cycle_id: "c1",
      phase_seq: 2,
      summary: "Done.\n\n**OK**",
      details: {
        type: "result",
        duration_ms: 33096,
        duration_api_ms: 33096,
        request_id: "req-1",
        session_id: "sess-1",
        usage: {
          inputTokens: 88710,
          outputTokens: 591,
          cacheReadTokens: 87424,
          cacheWriteTokens: 0,
        },
      },
    });
    expect(m).not.toBeNull();
    expect(m?.phase).toBe("execute");
    expect(m?.status).toBe("succeeded");
    expect(m?.summary).toContain("OK");
    expect(m?.cycleId).toBe("c1");
    expect(m?.phaseSeq).toBe(2);
    expect(m?.durationMs).toBe(33096);
    expect(m?.requestId).toBe("req-1");
    expect(m?.usage?.inputTokens).toBe(88710);
  });

  it("parses phase_failed with classification fields", () => {
    const m = parsePhaseEventOverview("phase_failed", {
      phase: "execute",
      status: "failed",
      summary: "Cursor usage limit reached",
      details: {
        stderr_tail: "limit\n",
        failure_kind: "cursor_usage_limit",
        standardized_message: "Switch model.",
      },
    });
    expect(m?.failureKind).toBe("cursor_usage_limit");
    expect(m?.standardizedMessage).toBe("Switch model.");
    expect(m?.stderrTail).toBe("limit\n");
  });
});
