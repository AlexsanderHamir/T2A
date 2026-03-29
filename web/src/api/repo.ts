import { readError } from "./shared";

/** File paths under REPO_ROOT matching q, or null if repo is not configured (503). */
export async function searchRepoFiles(
  q: string,
  options?: { signal?: AbortSignal },
): Promise<string[] | null> {
  const params = new URLSearchParams({ q });
  const res = await fetch(`/repo/search?${params}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
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
  const params = new URLSearchParams({
    path,
    start: String(start),
    end: String(end),
  });
  const res = await fetch(`/repo/validate-range?${params}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
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
