import type { TaskEvent } from "@/types";
import { eventTypeNeedsUserInput } from "./taskEventNeedsUser";

/** Who sent the last thread message, or null if no thread / legacy empty. */
export function lastThreadSpeaker(ev: TaskEvent): "user" | "agent" | null {
  const t = ev.response_thread;
  if (t && t.length > 0) {
    return t[t.length - 1]!.by;
  }
  if (ev.user_response?.trim()) {
    return "user";
  }
  return null;
}

/** True when this event type expects input and the agent spoke last (or nobody has replied yet). */
export function awaitingUserReply(ev: TaskEvent): boolean {
  if (!eventTypeNeedsUserInput(ev.type)) return false;
  const last = lastThreadSpeaker(ev);
  if (last === null) return true;
  return last === "agent";
}
