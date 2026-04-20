import { describe, expect, it } from "vitest";
import type { Status, Task } from "@/types";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import { userAttention } from "./taskAttention";

function minimalTask(status: Status): Task {
  return {
    id: "1",
    title: "Example",
    initial_prompt: "",
    status,
    priority: "medium",
    checklist_inherit: false,
    ...TASK_TEST_DEFAULTS,
  };
}

describe("userAttention", () => {
  it("surfaces approval pending above status-specific copy", () => {
    const out = userAttention(minimalTask("ready"), { approvalPending: true });
    expect(out.show).toBe(true);
    expect(out.headline).toBe("Approval requested");
    expect(out.body).toContain("approval");
  });

  it("returns status-driven attention for review, blocked, and failed", () => {
    const review = userAttention(minimalTask("review"), {
      approvalPending: false,
    });
    expect(review.show).toBe(true);
    expect(review.headline).toContain("review");

    const blocked = userAttention(minimalTask("blocked"), {
      approvalPending: false,
    });
    expect(blocked.show).toBe(true);
    expect(blocked.headline).toBe("Blocked");

    const failed = userAttention(minimalTask("failed"), {
      approvalPending: false,
    });
    expect(failed.show).toBe(true);
    expect(failed.headline).toContain("failed");
  });

  it("hides attention for other statuses when not pending approval", () => {
    for (const status of ["ready", "running", "done"] as const) {
      const out = userAttention(minimalTask(status), { approvalPending: false });
      expect(out).toEqual({ show: false, headline: "", body: "" });
    }
  });
});
