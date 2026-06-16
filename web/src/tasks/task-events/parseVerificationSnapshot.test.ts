import { describe, expect, it } from "vitest";
import { parseVerificationSnapshot } from "./parseVerificationSnapshot";

describe("parseVerificationSnapshot", () => {
  it("parses structured verification details from phase payload", () => {
    const m = parseVerificationSnapshot({
      verification: {
        attempt_seq: 1,
        passed_count: 3,
        failed_count: 1,
        criteria: [
          {
            criterion_id: "c1",
            text: "Each branch has a test",
            verified: false,
            verifier_kind: "verify_agent",
            reasoning: "Missing limit=201 test",
          },
        ],
      },
    });
    expect(m).not.toBeNull();
    expect(m?.attemptSeq).toBe(1);
    expect(m?.failedCount).toBe(1);
    expect(m?.criteria[0]?.text).toBe("Each branch has a test");
    expect(m?.criteria[0]?.reasoning).toBe("Missing limit=201 test");
  });

  it("returns null when verification block is absent", () => {
    expect(parseVerificationSnapshot({ stderr_tail: "boom" })).toBeNull();
    expect(parseVerificationSnapshot(undefined)).toBeNull();
  });
});
