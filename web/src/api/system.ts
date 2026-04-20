import type { SystemHealthResponse } from "@/types";
import { parseSystemHealthResponse } from "./parseSystemHealth";
import { fetchWithTimeout, readError } from "./shared";

/**
 * Operator-facing snapshot of the running taskapi process. Aggregated
 * from the in-process Prometheus default registry; see
 * docs/API-HTTP.md "System health" for the wire shape and invariants.
 */
export async function getSystemHealth(
  options?: { signal?: AbortSignal },
): Promise<SystemHealthResponse> {
  const res = await fetchWithTimeout("/system/health", {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseSystemHealthResponse(raw);
}
