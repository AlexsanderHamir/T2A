import type { AgentRunProgressItem } from "@/tasks/hooks/useAgentRunProgress";

export type AgentProgressKindTone =
  | "reply"
  | "tool"
  | "done"
  | "failed"
  | "session"
  | "error"
  | "neutral";

export type AgentProgressKindDescriptor = {
  label: string;
  title: string;
  tone: AgentProgressKindTone;
};

export function agentProgressKindDescriptor(
  kind: string,
  subtype?: string,
  tool?: string,
): AgentProgressKindDescriptor {
  const toolName = tool?.trim();
  if (kind === "tool_call" || kind === "tool") {
    if (subtype === "completed" || subtype === "success" || subtype === "done") {
      return {
        label: "Tool done",
        title: toolName
          ? `Tool finished successfully: ${toolName}`
          : "Cursor tool finished successfully",
        tone: "done",
      };
    }
    if (subtype === "failed" || subtype === "error") {
      return {
        label: "Tool failed",
        title: toolName
          ? `Tool returned an error: ${toolName}`
          : "Cursor tool returned an error",
        tone: "failed",
      };
    }
    return {
      label: "Tool call",
      title: toolName
        ? `Cursor invoked a tool: ${toolName}`
        : "Cursor started running a tool",
      tone: "tool",
    };
  }
  if (kind === "assistant" || kind === "message") {
    return {
      label: "Agent reply",
      title: "Message from the Cursor agent",
      tone: "reply",
    };
  }
  if (kind === "run_state") {
    if (subtype === "idle_suspicious") {
      return {
        label: "May be stuck",
        title: "Agent stdout has gone quiet",
        tone: "failed",
      };
    }
    if (subtype === "idle_kill_pending") {
      return {
        label: "Terminating soon",
        title: "Agent will be terminated if output stays silent",
        tone: "error",
      };
    }
    if (subtype === "idle_recovered") {
      return {
        label: "Recovered",
        title: "Recovered from a silent agent using saved evidence",
        tone: "done",
      };
    }
  }
  if (kind === "system") {
    return {
      label: "Session",
      title: "Cursor CLI session event",
      tone: "session",
    };
  }
  if (kind === "error") {
    return {
      label: "Error",
      title: "Cursor stream reported an error",
      tone: "error",
    };
  }
  const normalized = kind.replace(/_/g, " ");
  return {
    label: normalized,
    title: `Cursor stream event: ${normalized}`,
    tone: "neutral",
  };
}

export function agentProgressKindLabel(
  kind: string,
  subtype?: string,
  tool?: string,
): string {
  return agentProgressKindDescriptor(kind, subtype, tool).label;
}

export function agentProgressMessage(item: AgentRunProgressItem): string {
  return item.progress.message || item.progress.tool || "Working…";
}

export function formatAgentProgressElapsed(receivedAt: number, now: number): string {
  const elapsedSeconds = Math.max(0, Math.floor((now - receivedAt) / 1000));
  if (elapsedSeconds < 1) return "just now";
  if (elapsedSeconds < 60) return `${elapsedSeconds}s ago`;
  return `${Math.floor(elapsedSeconds / 60)}m ago`;
}

export function formatAgentProgressClockTime(receivedAt: number): string {
  return new Date(receivedAt).toLocaleTimeString(undefined, {
    hour: "numeric",
    minute: "2-digit",
  });
}
