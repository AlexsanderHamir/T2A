import { describe, expect, it } from "vitest";
import { errorMessage } from "./errorMessage";

describe("errorMessage", () => {
  it("returns the .message when given an Error instance", () => {
    expect(errorMessage(new Error("boom"))).toBe("boom");
  });

  it("returns the message of subclassed Errors (TypeError, custom classes)", () => {
    expect(errorMessage(new TypeError("bad type"))).toBe("bad type");

    class HttpError extends Error {
      constructor(message: string) {
        super(message);
        this.name = "HttpError";
      }
    }
    expect(errorMessage(new HttpError("503"))).toBe("503");
  });

  it("stringifies non-Error primitives so banners never show 'undefined'", () => {
    expect(errorMessage("string thrown")).toBe("string thrown");
    expect(errorMessage(42)).toBe("42");
    expect(errorMessage(null)).toBe("null");
    expect(errorMessage(undefined)).toBe("undefined");
  });

  it("falls back to String() for non-Error objects (no '[object Object]' surprise hidden)", () => {
    expect(errorMessage({ status: 500 })).toBe("[object Object]");
  });

  it("returns the kinder fallback string for non-Error inputs when provided", () => {
    expect(errorMessage("string thrown", "Could not load updates.")).toBe(
      "Could not load updates.",
    );
    expect(errorMessage(undefined, "Could not load updates.")).toBe(
      "Could not load updates.",
    );
    expect(errorMessage({ status: 500 }, "Load failed")).toBe("Load failed");
  });

  it("ignores the fallback when an Error is passed (the original message wins)", () => {
    expect(errorMessage(new Error("real cause"), "Load failed")).toBe(
      "real cause",
    );
  });
});
