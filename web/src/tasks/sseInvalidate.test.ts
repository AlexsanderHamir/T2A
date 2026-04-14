import { describe, expect, it } from "vitest";
import { collectTaskIdFromSSEData } from "./sseInvalidate";

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
    collectTaskIdFromSSEData('{"id":"same"}', s);
    collectTaskIdFromSSEData('{"id":"same"}', s);
    expect([...s]).toEqual(["same"]);
  });
});
