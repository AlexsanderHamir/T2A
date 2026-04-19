import { fetchWithTimeout } from "./shared";

/**
 * Loads optional dev/demo graph JSON from `VITE_TASK_GRAPH_MOCK_URL`.
 * All network I/O for this path lives in `api/` per CODE_STANDARDS.
 */
export async function fetchTaskGraphMockJson(
  mockUrl: string,
  init?: { signal?: AbortSignal },
): Promise<unknown> {
  const res = await fetchWithTimeout(
    mockUrl,
    {
      headers: { Accept: "application/json" },
      signal: init?.signal,
    },
    { timeoutMs: 20_000 },
  );
  if (!res.ok) {
    throw new Error(`Could not load graph mock from ${mockUrl}`);
  }
  return res.json() as Promise<unknown>;
}
