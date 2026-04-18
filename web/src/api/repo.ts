import { readError } from "./shared";

/** Match pkgs/tasks/handler/repo_handlers.go and docs/API-HTTP.md (abuse guards). */
export const maxRepoPathQueryBytes = 4096;
export const maxRepoSearchQueryBytes = 512;
export const maxRepoLineQueryParamBytes = 32;

/**
 * Hard ceiling on every `/repo/*` and `/health/ready` fetch in this module
 * (probe, search, validate-range, file). Long enough to absorb a cold
 * filesystem walk on first hit, short enough that a hung backend cannot
 * pin a UI request forever.
 */
const repoFetchTimeoutMs = 45_000;

function assertRepoRelPath(path: string): string {
  const t = path.trim();
  if (t.length === 0) {
    throw new Error("path is required");
  }
  if (t.length > maxRepoPathQueryBytes) {
    throw new Error("path is too long");
  }
  return t;
}

function assertRepoSearchQueryLength(q: string): void {
  if (q.length > maxRepoSearchQueryBytes) {
    throw new Error("search query is too long");
  }
}

function assertRepoLineQueryParam(name: string, n: number): string {
  if (!Number.isFinite(n) || !Number.isInteger(n) || n < 1) {
    throw new Error(`${name} must be a positive integer`);
  }
  const s = String(n);
  if (s.length > maxRepoLineQueryParamBytes) {
    throw new Error(`${name} is too large`);
  }
  return s;
}

/**
 * Build an AbortSignal that fires when either the caller's signal aborts or
 * the shared `repoFetchTimeoutMs` deadline elapses. Shared by every fetch
 * in this module so abort/timeout semantics stay consistent across endpoints.
 */
function repoFetchCombinedSignal(
  user?: AbortSignal,
): AbortSignal | undefined {
  const AT = AbortSignal as typeof AbortSignal & {
    timeout?: (ms: number) => AbortSignal;
    any?: (signals: AbortSignal[]) => AbortSignal;
  };
  const timeoutSig =
    typeof AT.timeout === "function" ? AT.timeout(repoFetchTimeoutMs) : undefined;
  if (!timeoutSig) {
    return user;
  }
  if (!user) {
    return timeoutSig;
  }
  if (typeof AT.any === "function") {
    return AT.any([user, timeoutSig]);
  }
  const combined = new AbortController();
  const abortCombined = () => {
    if (!combined.signal.aborted) {
      combined.abort();
    }
  };
  user.addEventListener("abort", abortCombined, { once: true });
  timeoutSig.addEventListener("abort", abortCombined, { once: true });
  if (user.aborted || timeoutSig.aborted) {
    abortCombined();
  }
  return combined.signal;
}

/** Result of probing whether taskapi has a usable workspace repo (see GET /health/ready). */
export type RepoWorkspaceProbe =
  | { state: "available" }
  | { state: "unavailable" }
  | { state: "broken" }
  | { state: "unknown" };

/**
 * Lightweight check: does the running taskapi have REPO_ROOT configured and on disk?
 * Prefer this over GET /repo/search?q= on mount (avoids walking the tree).
 */
export async function probeRepoWorkspace(
  options?: { signal?: AbortSignal },
): Promise<RepoWorkspaceProbe> {
  try {
    const res = await fetch("/health/ready", {
      headers: { Accept: "application/json" },
      signal: repoFetchCombinedSignal(options?.signal),
    });
    let raw: unknown;
    try {
      raw = await res.json();
    } catch {
      return { state: "unknown" };
    }
    if (raw === null || typeof raw !== "object") {
      return { state: "unknown" };
    }
    const body = raw as {
      status?: string;
      checks?: Record<string, string>;
    };
    const checks = body.checks ?? {};
    const st = body.status ?? "";

    if (!res.ok) {
      if (
        st === "degraded" &&
        checks.database === "ok" &&
        checks.workspace_repo === "fail"
      ) {
        return { state: "broken" };
      }
      return { state: "unknown" };
    }

    if (st === "ok" && checks.database === "ok") {
      if (checks.workspace_repo === "ok") return { state: "available" };
      if (checks.workspace_repo === undefined) return { state: "unavailable" };
      if (checks.workspace_repo === "fail") return { state: "broken" };
    }

    return { state: "unknown" };
  } catch {
    return { state: "unknown" };
  }
}

/** File paths under REPO_ROOT matching q, or null if repo is not configured (503). */
export async function searchRepoFiles(
  q: string,
  options?: { signal?: AbortSignal },
): Promise<string[] | null> {
  assertRepoSearchQueryLength(q);
  const params = new URLSearchParams({ q });
  const res = await fetch(`/repo/search?${params}`, {
    headers: { Accept: "application/json" },
    signal: repoFetchCombinedSignal(options?.signal),
  });
  if (res.status === 503) {
    return null;
  }
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  if (
    raw !== null &&
    typeof raw === "object" &&
    "paths" in raw &&
    Array.isArray((raw as { paths: unknown }).paths)
  ) {
    return (raw as { paths: string[] }).paths.filter(
      (p): p is string => typeof p === "string",
    );
  }
  throw new Error("unexpected search response");
}

export type RepoValidateRangeResult = {
  ok: boolean;
  line_count?: number;
  warning?: string;
};

/** Returns null if repo is not configured (503). */
export async function validateRepoRange(
  path: string,
  start: number,
  end: number,
  options?: { signal?: AbortSignal },
): Promise<RepoValidateRangeResult | null> {
  const p = assertRepoRelPath(path);
  const params = new URLSearchParams({
    path: p,
    start: assertRepoLineQueryParam("start", start),
    end: assertRepoLineQueryParam("end", end),
  });
  const res = await fetch(`/repo/validate-range?${params}`, {
    headers: { Accept: "application/json" },
    signal: repoFetchCombinedSignal(options?.signal),
  });
  if (res.status === 503) {
    return null;
  }
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  if (raw !== null && typeof raw === "object" && "ok" in raw) {
    const o = raw as {
      ok: boolean;
      line_count?: number;
      warning?: string;
    };
    return {
      ok: Boolean(o.ok),
      line_count: typeof o.line_count === "number" ? o.line_count : undefined,
      warning: typeof o.warning === "string" ? o.warning : undefined,
    };
  }
  throw new Error("unexpected validate-range response");
}

export type RepoFileResult = {
  path: string;
  content: string;
  binary: boolean;
  truncated: boolean;
  size_bytes: number;
  line_count: number;
  warning?: string;
};

/** Full file text for @ line-range UI, or null if repo is not configured (503). */
export async function fetchRepoFile(
  path: string,
  options?: { signal?: AbortSignal },
): Promise<RepoFileResult | null> {
  const p = assertRepoRelPath(path);
  const params = new URLSearchParams({ path: p });
  const res = await fetch(`/repo/file?${params}`, {
    headers: { Accept: "application/json" },
    signal: repoFetchCombinedSignal(options?.signal),
  });
  if (res.status === 503) {
    return null;
  }
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  if (raw === null || typeof raw !== "object") {
    throw new Error("unexpected file response");
  }
  const o = raw as Record<string, unknown>;
  const pathVal = o.path;
  const contentVal = o.content;
  const binaryVal = o.binary;
  const truncatedVal = o.truncated;
  const sizeVal = o.size_bytes;
  const linesVal = o.line_count;
  if (
    typeof pathVal !== "string" ||
    typeof contentVal !== "string" ||
    typeof binaryVal !== "boolean" ||
    typeof truncatedVal !== "boolean" ||
    typeof sizeVal !== "number" ||
    typeof linesVal !== "number"
  ) {
    throw new Error("unexpected file response shape");
  }
  const out: RepoFileResult = {
    path: pathVal,
    content: contentVal,
    binary: binaryVal,
    truncated: truncatedVal,
    size_bytes: sizeVal,
    line_count: linesVal,
  };
  if (typeof o.warning === "string") {
    out.warning = o.warning;
  }
  return out;
}
