import { describe, expect, it } from "vitest";
import { canEditTaskPickupSchedule } from "./canEditTaskPickupSchedule";

describe("canEditTaskPickupSchedule", () => {
  it("allows schedule edits for queued and on-hold tasks", () => {
    expect(canEditTaskPickupSchedule("ready")).toBe(true);
    expect(canEditTaskPickupSchedule("on_hold")).toBe(true);
  });

  it("blocks schedule edits once the task is running or in flight", () => {
    expect(canEditTaskPickupSchedule("running")).toBe(false);
    expect(canEditTaskPickupSchedule("blocked")).toBe(false);
    expect(canEditTaskPickupSchedule("review")).toBe(false);
  });

  it("blocks schedule edits for terminal tasks", () => {
    expect(canEditTaskPickupSchedule("done")).toBe(false);
    expect(canEditTaskPickupSchedule("failed")).toBe(false);
  });
});
