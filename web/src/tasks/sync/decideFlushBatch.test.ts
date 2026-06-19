import { describe, expect, it } from "vitest";
import { taskQueryKeys } from "../task-query";
import { decideFlushBatch } from "./decideFlushBatch";
import { cycleEnrichmentKey, emptyPending } from "./syncConstants";

describe("decideFlushBatch", () => {
  it("invalidates all task queries when pending is empty", () => {
    const pending = emptyPending();
    const decision = decideFlushBatch(pending);
    expect(decision.invalidateKeys).toContainEqual(taskQueryKeys.all);
    expect(decision.invalidateKeys).toContainEqual(taskQueryKeys.stats());
    expect(decision.invalidateKeys).toContainEqual(taskQueryKeys.cycleFailuresRoot());
  });

  it("skips detail prefix when every task was enriched", () => {
    const pending = emptyPending();
    pending.tasks.add("t1");
    pending.enrichedTasks.add("t1");
    const decision = decideFlushBatch(pending);
    expect(decision.invalidateKeys).toContainEqual(taskQueryKeys.listRoot());
    expect(decision.invalidateKeys).not.toContainEqual([...taskQueryKeys.all, "detail"]);
    expect(decision.invalidateKeys).toContainEqual(taskQueryKeys.commits("t1"));
  });

  it("invalidates detail prefix when any task was not enriched", () => {
    const pending = emptyPending();
    pending.tasks.add("t1");
    pending.tasks.add("t2");
    pending.enrichedTasks.add("t1");
    const decision = decideFlushBatch(pending);
    expect(decision.invalidateKeys).toContainEqual([...taskQueryKeys.all, "detail"]);
  });

  it("invalidates cycles bucket when cycle-only and not all enriched", () => {
    const pending = emptyPending();
    pending.cycles.set("t1", new Set(["c1", "c2"]));
    pending.enrichedCycles.add(cycleEnrichmentKey("t1", "c1"));
    const decision = decideFlushBatch(pending);
    expect(decision.invalidateKeys).toContainEqual(taskQueryKeys.cycles("t1"));
  });

  it("skips cycles invalidation when task id is also in tasks set", () => {
    const pending = emptyPending();
    pending.tasks.add("t1");
    pending.cycles.set("t1", new Set(["c1"]));
    const decision = decideFlushBatch(pending);
    expect(decision.invalidateKeys).not.toContainEqual(taskQueryKeys.cycles("t1"));
  });
});
