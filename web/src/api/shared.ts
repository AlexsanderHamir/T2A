/** Shared headers and error text for JSON APIs. */
export const jsonHeaders = {
  "Content-Type": "application/json",
  Accept: "application/json",
};

const defaultFetchTimeoutMs = 20_000;

function timeoutSignal(ms: number): AbortSignal | undefined {
  const AT = AbortSignal as typeof AbortSignal & {
    timeout?: (timeoutMs: number) => AbortSignal;
  };
  if (typeof AT.timeout !== "function") {
    return undefined;
  }
  return AT.timeout(ms);
}

export async function fetchWithTimeout(
  input: RequestInfo | URL,
  init?: RequestInit,
  options?: { timeoutMs?: number },
): Promise<Response> {
  const timeoutMs = options?.timeoutMs ?? defaultFetchTimeoutMs;
  const timeout = timeoutSignal(timeoutMs);
  if (!timeout || init?.signal) {
    return fetch(input, init);
  }
  return fetch(input, { ...init, signal: timeout });
}

export async function readError(res: Response): Promise<string> {
  const t = await res.text();
  try {
    const j = JSON.parse(t) as { error?: string; request_id?: string };
    const msg = typeof j?.error === "string" && j.error.trim() ? j.error.trim() : "";
    const rid =
      typeof j?.request_id === "string" && j.request_id.trim() ? j.request_id.trim() : "";
    if (msg) {
      return rid ? `${msg} (request ${rid})` : msg;
    }
    if (rid) {
      return `Error (request ${rid})`;
    }
  } catch {
    /* plain text */
  }
  return t.trim() || res.statusText;
}
