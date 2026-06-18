import type { VerifierKind } from "@/types/cycle";

export type VerificationCriterion = {
  criterionId: string;
  text?: string;
  verified: boolean;
  verifierKind?: VerifierKind | "";
  reasoning?: string;
  evidence?: string;
};

export type VerificationSnapshot = {
  attemptSeq: number;
  passedCount: number;
  failedCount: number;
  criteria: VerificationCriterion[];
};

function num(v: unknown): number | undefined {
  return typeof v === "number" && Number.isFinite(v) ? v : undefined;
}

function str(v: unknown): string | undefined {
  return typeof v === "string" && v.length > 0 ? v : undefined;
}

function bool(v: unknown): boolean | undefined {
  return typeof v === "boolean" ? v : undefined;
}

function parseVerifierKind(v: unknown): VerifierKind | "" {
  const s = str(v);
  if (!s) return "";
  switch (s) {
    case "verify_agent":
    case "agent_self":
    case "deterministic_check":
    case "human_override":
    case "legacy":
      return s;
    default:
      return "";
  }
}

function parseCriterion(raw: unknown): VerificationCriterion | null {
  if (!raw || typeof raw !== "object") return null;
  const o = raw as Record<string, unknown>;
  const criterionId = str(o.criterion_id);
  const verified = bool(o.verified);
  if (!criterionId || verified === undefined) return null;
  return {
    criterionId,
    text: str(o.text),
    verified,
    verifierKind: parseVerifierKind(o.verifier_kind),
    reasoning: str(o.reasoning),
    evidence: str(o.evidence),
  };
}

/**
 * Parses the structured verification snapshot embedded in phase
 * details by the agent harness. Returns null when the payload predates
 * enrichment or is not a verify-phase event.
 */
export function parseVerificationSnapshot(
  details: unknown,
): VerificationSnapshot | null {
  if (!details || typeof details !== "object") return null;
  const verification = (details as Record<string, unknown>).verification;
  if (!verification || typeof verification !== "object") return null;
  const v = verification as Record<string, unknown>;
  const attemptSeq = num(v.attempt_seq);
  const passedCount = num(v.passed_count);
  const failedCount = num(v.failed_count);
  if (
    attemptSeq === undefined ||
    passedCount === undefined ||
    failedCount === undefined
  ) {
    return null;
  }
  const rawCriteria = v.criteria;
  if (!Array.isArray(rawCriteria)) return null;
  const criteria: VerificationCriterion[] = [];
  for (const row of rawCriteria) {
    const parsed = parseCriterion(row);
    if (parsed) criteria.push(parsed);
  }
  if (criteria.length === 0) return null;
  return {
    attemptSeq,
    passedCount,
    failedCount,
    criteria,
  };
}

export function verifierKindLabel(kind: VerifierKind | ""): string {
  switch (kind) {
    case "verify_agent":
      return "Verify agent";
    case "agent_self":
      return "Self-reported";
    case "deterministic_check":
      return "Deterministic check";
    case "human_override":
      return "Human override";
    case "legacy":
      return "Legacy";
    default:
      return "";
  }
}

export function verdictPillClass(verified: boolean): string {
  return verified ? "cell-pill--status-done" : "cell-pill--status-failed";
}
