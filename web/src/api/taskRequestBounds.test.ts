import { describe, expect, it } from "vitest";
import {
  assertAfterId,
  assertListIntQuery,
  assertNonNegativeOffset,
  assertOptionalTaskPathId,
  assertPositiveSeq,
  assertTaskPathId,
  maxListAfterIDParamBytes,
  maxTaskPathIDBytes,
} from "./taskRequestBounds";

describe("assertTaskPathId", () => {
  it("returns trimmed id when valid", () => {
    expect(assertTaskPathId("  u1  ")).toBe("u1");
  });

  it("rejects empty and whitespace-only", () => {
    expect(() => assertTaskPathId("")).toThrow(/required/);
    expect(() => assertTaskPathId("   ")).toThrow(/required/);
  });

  it("rejects over max length", () => {
    const long = "a".repeat(maxTaskPathIDBytes + 1);
    expect(() => assertTaskPathId(long)).toThrow(/too long/);
  });
});

describe("assertOptionalTaskPathId", () => {
  it("returns undefined when input undefined", () => {
    expect(assertOptionalTaskPathId(undefined, "id")).toBeUndefined();
  });
});

describe("assertAfterId", () => {
  it("rejects when longer than maxListAfterIDParamBytes", () => {
    const long = "b".repeat(maxListAfterIDParamBytes + 1);
    expect(() => assertAfterId(long)).toThrow(/too long/);
  });
});

describe("assertListIntQuery", () => {
  it("rejects non-integers and out-of-range values", () => {
    expect(() => assertListIntQuery("limit", 1.5, 0, 200)).toThrow(/integer/);
    expect(() => assertListIntQuery("limit", 201, 0, 200)).toThrow(/between/);
    expect(() => assertListIntQuery("limit", Number.NaN, 0, 200)).toThrow(/integer/);
  });
});

describe("assertNonNegativeOffset", () => {
  it("rejects negative offset", () => {
    expect(() => assertNonNegativeOffset("offset", -1)).toThrow(/non-negative/);
  });
});

describe("assertPositiveSeq", () => {
  it("rejects zero and non-finite", () => {
    expect(() => assertPositiveSeq("seq", 0)).toThrow(/positive/);
    expect(() => assertPositiveSeq("seq", Number.NaN)).toThrow(/positive/);
  });
});
