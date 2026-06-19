import { describe, expect, it } from "vitest";
import {
  agentProgressKindLabel,
  agentProgressMessage,
  formatAgentProgressElapsed,
} from "./agentProgressDisplay";
import type { AgentRunProgressItem } from "@/tasks/hooks/useAgentRunProgress";

function item(
  overrides: Partial<AgentRunProgressItem> & Pick<AgentRunProgressItem, "progress">,
): AgentRunProgressItem {
  return {
    taskId: "task-1",
    cycleId: "cycle-1",
    phaseSeq: 1,
    receivedAt: 1_000_000,
    ...overrides,
  };
}

describe("agentProgressDisplay", () => {
  it.each([
    ["tool_call", "done", undefined, "Tool done"],
    ["tool_call", "error", "Read", "Tool failed"],
    ["tool_call", "started", "Read", "Tool call"],
    ["assistant", undefined, undefined, "Agent reply"],
    ["run_state", "idle_suspicious", undefined, "May be stuck"],
    ["run_state", "idle_kill_pending", undefined, "Terminating soon"],
    ["run_state", "idle_recovered", undefined, "Recovered"],
    ["custom_event", undefined, undefined, "custom event"],
  ] as const)(
    "agentProgressKindLabel(%s, %s) → %s",
    (kind, subtype, tool, expected) => {
      expect(agentProgressKindLabel(kind, subtype, tool)).toBe(expected);
    },
  );

  it("agentProgressMessage prefers message then tool then fallback", () => {
    expect(
      agentProgressMessage(
        item({ progress: { kind: "assistant", message: "Hello" } }),
      ),
    ).toBe("Hello");
    expect(
      agentProgressMessage(item({ progress: { kind: "tool_call", tool: "Grep" } })),
    ).toBe("Grep");
    expect(
      agentProgressMessage(item({ progress: { kind: "system" } })),
    ).toBe("Working…");
  });

  it.each([
    [1_000_000, 1_000_000, "just now"],
    [1_000_000, 1_000_500, "just now"],
    [1_000_000, 1_012_000, "12s ago"],
    [1_000_000, 1_130_000, "2m ago"],
    [1_000_000, 999_000, "just now"],
  ] as const)(
    "formatAgentProgressElapsed handles elapsed edge (%i, %i)",
    (receivedAt, now, expected) => {
      expect(formatAgentProgressElapsed(receivedAt, now)).toBe(expected);
    },
  );
});
