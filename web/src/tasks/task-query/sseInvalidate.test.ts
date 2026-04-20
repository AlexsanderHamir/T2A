import { describe, expect, it } from "vitest";
import {
  collectTaskIdFromSSEData,
  parseTaskChangeFrame,
} from "./sseInvalidate";

describe("collectTaskIdFromSSEData", () => {
  it("collects trimmed id from task SSE JSON", () => {
    const s = new Set<string>();
    collectTaskIdFromSSEData(
      '{"type":"task_updated","id":"  abc-1  "}',
      s,
    );
    expect([...s]).toEqual(["abc-1"]);
  });

  it("ignores malformed JSON", () => {
    const s = new Set<string>();
    collectTaskIdFromSSEData("not json", s);
    expect(s.size).toBe(0);
  });

  it("ignores missing or non-string id", () => {
    const s = new Set<string>();
    collectTaskIdFromSSEData('{"type":"task_updated"}', s);
    collectTaskIdFromSSEData('{"id":null}', s);
    expect(s.size).toBe(0);
  });

  it("ignores blank or whitespace-only payloads", () => {
    const s = new Set<string>();
    collectTaskIdFromSSEData("", s);
    collectTaskIdFromSSEData("   \n\t  ", s);
    collectTaskIdFromSSEData('{"id":"   "}', s);
    expect(s.size).toBe(0);
  });

  it("dedupes repeated ids in the same set", () => {
    const s = new Set<string>();
    collectTaskIdFromSSEData('{"type":"task_updated","id":"same"}', s);
    collectTaskIdFromSSEData('{"type":"task_updated","id":"same"}', s);
    expect([...s]).toEqual(["same"]);
  });

  it("does not collect task ids from cycle frames (granular path uses parseTaskChangeFrame)", () => {
    const s = new Set<string>();
    collectTaskIdFromSSEData(
      '{"type":"task_cycle_changed","id":"task-1","cycle_id":"cyc-1"}',
      s,
    );
    expect(s.size).toBe(0);
  });
});

describe("parseTaskChangeFrame", () => {
  it("returns a task frame for task_created/updated/deleted", () => {
    expect(
      parseTaskChangeFrame('{"type":"task_updated","id":"task-1"}'),
    ).toEqual({ kind: "task", taskId: "task-1" });
    expect(
      parseTaskChangeFrame('{"type":"task_created","id":"task-2"}'),
    ).toEqual({ kind: "task", taskId: "task-2" });
    expect(
      parseTaskChangeFrame('{"type":"task_deleted","id":"task-3"}'),
    ).toEqual({ kind: "task", taskId: "task-3" });
  });

  it("returns a cycle frame only when both id and cycle_id are present", () => {
    expect(
      parseTaskChangeFrame(
        '{"type":"task_cycle_changed","id":"task-1","cycle_id":"cyc-1"}',
      ),
    ).toEqual({ kind: "cycle", taskId: "task-1", cycleId: "cyc-1" });
    expect(
      parseTaskChangeFrame('{"type":"task_cycle_changed","id":"task-1"}'),
    ).toBeNull();
    expect(
      parseTaskChangeFrame(
        '{"type":"task_cycle_changed","id":"task-1","cycle_id":"   "}',
      ),
    ).toBeNull();
  });

  it("returns null for blank, malformed, missing-id, unknown-type, or array payloads", () => {
    expect(parseTaskChangeFrame("")).toBeNull();
    expect(parseTaskChangeFrame("   \n")).toBeNull();
    expect(parseTaskChangeFrame("not json")).toBeNull();
    expect(parseTaskChangeFrame("[1,2,3]")).toBeNull();
    expect(parseTaskChangeFrame('{"type":"task_updated"}')).toBeNull();
    expect(parseTaskChangeFrame('{"type":"sync_ping","id":"x"}')).toBeNull();
  });

  it("returns settings/agent_run_cancelled frames without an id", () => {
    expect(parseTaskChangeFrame('{"type":"settings_changed"}')).toEqual({
      kind: "settings",
    });
    expect(parseTaskChangeFrame('{"type":"agent_run_cancelled"}')).toEqual({
      kind: "agent_run_cancelled",
    });
  });

  // Phase 2 of the realtime smoothness plan: the hub emits this
  // directive when the client's reconnect cursor fell out of the
  // ring buffer (or it was forcibly disconnected as a slow consumer).
  // The frame deliberately carries no id/cycle_id; consumers MUST
  // drop every cached query and refetch from REST. Pinning the
  // parser separately from the consumer (useTaskEventStream) keeps
  // the wire-shape contract explicit even if the hook moves.
  it("returns a resync frame for the hub-emitted resync directive", () => {
    expect(parseTaskChangeFrame('{"type":"resync"}')).toEqual({
      kind: "resync",
    });
  });
});
