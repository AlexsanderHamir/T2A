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

  it("trims and validates when a string is provided", () => {
    expect(assertOptionalTaskPathId("  u2  ", "parent_id")).toBe("u2");
  });

  it("rejects whitespace-only string same as assertTaskPathId", () => {
    expect(() => assertOptionalTaskPathId("   ", "id")).toThrow(/required/);
  });
});

describe("assertAfterId", () => {
  it("returns trimmed after_id when valid", () => {
    expect(assertAfterId("  cursor-1  ")).toBe("cursor-1");
  });

  it("rejects empty after_id", () => {
    expect(() => assertAfterId("")).toThrow(/required/);
    expect(() => assertAfterId("  ")).toThrow(/required/);
  });

  it("rejects when longer than maxListAfterIDParamBytes", () => {
    const long = "b".repeat(maxListAfterIDParamBytes + 1);
    expect(() => assertAfterId(long)).toThrow(/too long/);
  });
});

describe("assertListIntQuery", () => {
  it("returns decimal string for in-range integers", () => {
    expect(assertListIntQuery("limit", 0, 0, 200)).toBe("0");
    expect(assertListIntQuery("limit", 200, 0, 200)).toBe("200");
    expect(assertListIntQuery("limit", 50, 0, 200)).toBe("50");
  });

  it("rejects non-integers and out-of-range values", () => {
    expect(() => assertListIntQuery("limit", 1.5, 0, 200)).toThrow(/integer/);
    expect(() => assertListIntQuery("limit", 201, 0, 200)).toThrow(/between/);
    expect(() => assertListIntQuery("limit", Number.NaN, 0, 200)).toThrow(/integer/);
  });
});

describe("assertNonNegativeOffset", () => {
  it("returns string for zero and positive integers", () => {
    expect(assertNonNegativeOffset("offset", 0)).toBe("0");
    expect(assertNonNegativeOffset("offset", 42)).toBe("42");
  });

  it("rejects negative offset", () => {
    expect(() => assertNonNegativeOffset("offset", -1)).toThrow(/non-negative/);
  });

  it("rejects non-integers", () => {
    expect(() => assertNonNegativeOffset("offset", 1.2)).toThrow(/non-negative/);
  });
});

describe("assertPositiveSeq", () => {
  it("returns string for positive integers", () => {
    expect(assertPositiveSeq("seq", 1)).toBe("1");
    expect(assertPositiveSeq("seq", 99)).toBe("99");
  });

  it("rejects zero and non-finite", () => {
    expect(() => assertPositiveSeq("seq", 0)).toThrow(/positive/);
    expect(() => assertPositiveSeq("seq", Number.NaN)).toThrow(/positive/);
  });

  it("rejects non-integers and negative values", () => {
    expect(() => assertPositiveSeq("seq", -3)).toThrow(/positive/);
    expect(() => assertPositiveSeq("seq", 2.5)).toThrow(/positive/);
  });
});
