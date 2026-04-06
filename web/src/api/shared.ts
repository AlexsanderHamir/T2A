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

function combineSignals(
  user: AbortSignal | null | undefined,
  timeout: AbortSignal | undefined,
): AbortSignal | undefined {
  if (!user) return timeout;
  if (!timeout) return user;
  const AT = AbortSignal as typeof AbortSignal & {
    any?: (signals: AbortSignal[]) => AbortSignal;
  };
  if (typeof AT.any === "function") {
    return AT.any([user, timeout]);
  }
  const combined = new AbortController();
  const abortCombined = () => {
    if (!combined.signal.aborted) {
      combined.abort();
    }
  };
  user.addEventListener("abort", abortCombined, { once: true });
  timeout.addEventListener("abort", abortCombined, { once: true });
  if (user.aborted || timeout.aborted) {
    abortCombined();
  }
  return combined.signal;
}

export async function fetchWithTimeout(
  input: RequestInfo | URL,
  init?: RequestInit,
  options?: { timeoutMs?: number },
): Promise<Response> {
  const timeoutMs = options?.timeoutMs ?? defaultFetchTimeoutMs;
  const timeout = timeoutSignal(timeoutMs);
  const signal = combineSignals(init?.signal, timeout);
  return fetch(input, { ...init, ...(signal ? { signal } : {}) });
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
